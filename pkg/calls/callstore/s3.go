package callstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/config"

	"github.com/go-viper/mapstructure/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	"github.com/rs/zerolog/log"
)

type s3Backend struct {
	Bucket         string        `yaml:"bucket"`
	Secure         bool          `yaml:"secure"`
	Endpoint       string        `yaml:"endpoint"`
	ExternalHost   *string       `yaml:"externalHost"`
	ExternalSecure bool          `yaml:"externalSecure"`
	Region         string        `yaml:"region"`
	KeyID          string        `yaml:"keyID"`
	SecretKey      string        `yaml:"secretKey"`
	Timeout        time.Duration `yaml:"timeout"`
	Trace          bool          `yaml:"trace"`

	// LegacyPrefix puts <Prefix/> right under the <Rule>. If it is false, modern S3-style
	// <Filter><Prefix/></Filter> is used. Some "S3 compatible" APIs require this.
	LegacyPrefix bool `yaml:"legacyPrefix"`

	cli *minio.Client
	st  Store
	rj  *ruleJob
}

func (*s3Backend) Type() string { return "s3" }

const S3LifecycleTTL = 10 * time.Minute

type ruleJob struct {
	cfg     *lifecycle.Configuration
	be      *s3Backend
	ruleMap map[string]lifecycle.Rule
	adds    []lifecycle.Rule
	dels    map[string]struct{} // keys to delete
}

func (rj *ruleJob) delete(id string) {
	rj.dels[id] = struct{}{}
	delete(rj.ruleMap, id)
}

func (rj *ruleJob) add(r lifecycle.Rule) error {
	if _, has := rj.ruleMap[r.ID]; has {
		return errors.New("rule already exists")
	}

	rj.ruleMap[r.ID] = r

	rj.adds = append(rj.adds, r)

	return nil
}

func (rj *ruleJob) lifecycleConfig() *lifecycle.Configuration {
	i := 0
	for _, r := range rj.cfg.Rules {
		if _, hasDel := rj.dels[r.ID]; !hasDel {
			rj.cfg.Rules[i] = r
			i++
		}
	}

	// erase truncated Values
	for j := i; j < len(rj.cfg.Rules); j++ {
		rj.cfg.Rules[j] = lifecycle.Rule{}
	}
	rj.cfg.Rules = rj.cfg.Rules[:i]

	rj.cfg.Rules = append(rj.cfg.Rules, rj.adds...)

	return rj.cfg
}

func (rj *ruleJob) addRmRule(refPath string) error {
	ruleID := s3ruleID(refPath)

	log.Debug().Str("prefix", refPath).Msg("add rm rule")
	lr := lifecycle.Rule{
		ID:     ruleID,
		Status: "Enabled",
		Expiration: lifecycle.Expiration{
			Days: lifecycle.ExpirationDays(1),
		},
	}

	if rj.be.LegacyPrefix {
		lr.Prefix = refPath
	} else {
		lr.RuleFilter = lifecycle.Filter{
			Prefix: refPath,
		}
	}

	return rj.add(lr)
}

func (rj *ruleJob) pruneRmRule(refPath string) error {
	rj.delete(s3ruleID(refPath))

	return nil
}

func (sb *s3Backend) NewPruneJob(ctx context.Context) (PruneJob, error) {
	return sb.newRuleJob(ctx)
}

func (*ruleJob) IsPruneJob() {}

func (sb *s3Backend) newRuleJob(ctx context.Context) (*ruleJob, error) {
	rj := &ruleJob{
		ruleMap: make(map[string]lifecycle.Rule),
		be:      sb,
		dels:    make(map[string]struct{}),
	}

	lc, err := sb.cli.GetBucketLifecycle(ctx, sb.Bucket)
	if err != nil {
		if !sb.isNoSuchLifecycleConfig(err) {
			return nil, err
		}

		lc = lifecycle.NewConfiguration()
	}

	rj.cfg = lc

	for _, r := range rj.cfg.Rules {
		rj.ruleMap[r.ID] = r
	}

	return rj, nil
}

func ruleJobFromCtx(ctx context.Context) *ruleJob {
	pjfc := PruneJobFromContext(ctx)
	if pjfc == nil {
		return nil
	}

	return pjfc.(*ruleJob)
}

func (rj *ruleJob) Begin(ctx context.Context) error {
	return nil
}

func (rj *ruleJob) Commit(ctx context.Context) error {
	if rj == nil || (len(rj.adds) == 0 && len(rj.dels) == 0) {
		return nil
	}

	return rj.be.commitRuleJob(ctx, rj)
}

func (rj *ruleJob) Rollback(ctx context.Context) error {
	// do nothing
	return nil
}

func (sb *s3Backend) commitRuleJob(ctx context.Context, rj *ruleJob) error {
	err := sb.cli.SetBucketLifecycle(ctx, sb.Bucket, rj.lifecycleConfig())
	if err != nil {
		return fmt.Errorf("set bucket lifecycle: %w", err)
	}
	log.Info().Str("bucket", sb.Bucket).Msg("lifecycle policy set")

	return nil
}

func (sb *s3Backend) Store(ctx context.Context, call *calls.CallAudio) (AudioRef, error) {
	audioPath, audioRef := sb.st.BlobPath(call)

	dctx, cancel := sb.ctxTimeout(ctx)
	defer cancel()

	var contentType string
	if call.AudioType != nil {
		contentType = *call.AudioType
	}

	_, err := sb.cli.PutObject(dctx, sb.Bucket, audioPath, bytes.NewReader(call.AudioBlob), int64(len(call.AudioBlob)), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return nil, err
	}

	return makeAudioRef(audioRef), nil
}

func (sb *s3Backend) getBlob(ctx context.Context, objKey string) ([]byte, error) {
	dctx, cancel := sb.ctxTimeout(ctx)
	defer cancel()

	b, err := sb.cli.GetObject(dctx, sb.Bucket, objKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}

	defer b.Close()

	return io.ReadAll(b)
}

func (sb *s3Backend) generateSignedURL(ctx context.Context, audioName *string, objKey string) (*url.URL, error) {
	par := make(url.Values)
	if audioName != nil {
		par.Set("response-content-disposition", fmt.Sprintf(`attachment; filename="%s"`, *audioName))
	}

	hdr := make(http.Header)
	if sb.ExternalHost != nil {
		hdr.Set("Host", *sb.ExternalHost)
	}

	ur, err := sb.cli.PresignHeader(ctx, "GET", sb.Bucket, objKey, time.Hour, par, hdr)
	if err != nil {
		return nil, err
	}

	if sb.ExternalHost != nil {
		ur.Host = *sb.ExternalHost
		if sb.ExternalSecure {
			ur.Scheme = "https"
		} else {
			ur.Scheme = "http"
		}
	}

	return ur, nil
}

func (sb *s3Backend) Get(ctx context.Context, call *calls.CallAudio, ref AudioRef, opts *CallAudioOptions) (blob []byte, audioURL *url.URL, err error) {
	objKey := ref.Ref(sb.st.PartMan(), call.CallDate.Time())

	if opts != nil && opts.resolveBlob {
		blob, err = sb.getBlob(ctx, objKey)
	} else {
		audioURL, err = sb.generateSignedURL(ctx, call.AudioName, objKey)
	}

	if err != nil {
		err = fmt.Errorf("%s: %w", objKey, err)
	}

	return
}

func (sb *s3Backend) prefixExists(ctx context.Context, prefix string) (bool, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	counter := 0
	for ob := range sb.cli.ListObjects(ctx, sb.Bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
		MaxKeys:   1,
	}) {
		if ob.Err != nil {
			if errors.Is(ob.Err, context.Canceled) {
				return counter > 0, nil
			}

			return counter > 0, ob.Err
		}

		if counter > 0 {
			cancel()
		}

		counter++
	}

	return counter > 0, nil
}

func (sb *s3Backend) Prune(ctx context.Context, refPath string, pruneAfter *time.Time) (*time.Time, error) {
	// get the ruleJob out of the context
	rj := ruleJobFromCtx(ctx)
	if rj == nil {
		return nil, fmt.Errorf("rule job not set in context")
	}

	isPrefix := strings.HasSuffix(refPath, "/")
	if !isPrefix {
		// singleton remove
		return nil, sb.delete(ctx, refPath)
	}

	// prune after 3 days
	newPruneAfter := time.Now().Add(72 * time.Hour)

	if pruneAfter != nil { // this has already been pruned, now check if the rule needs to be removed yet
		if !time.Now().After(*pruneAfter) {
			// this probably won't ever happen
			return nil, ErrNotYetPruneTime
		}

		exists, err := sb.prefixExists(ctx, refPath)
		if err != nil {
			return pruneAfter, err
		}

		if exists {
			log.Debug().Str("prefix", refPath).Msg("prefix still exists")
			return &newPruneAfter, nil
		}

		return nil, rj.pruneRmRule(refPath)
	}

	err := rj.addRmRule(refPath)
	if err != nil {
		return nil, fmt.Errorf("add lifecycle rule: %w", err)
	}

	return &newPruneAfter, nil
}

func (sb *s3Backend) isNoSuchLifecycleConfig(err error) bool {
	var erR minio.ErrorResponse
	return errors.As(err, &erR) && erR.Code == "NoSuchLifecycleConfiguration"
}

func s3ruleID(prefix string) string {
	return "sb_" + prefix
}

func (sb *s3Backend) delete(ctx context.Context, objKey string) error {
	return sb.cli.RemoveObject(ctx, sb.Bucket, objKey, minio.RemoveObjectOptions{})
}

func (sb *s3Backend) Delete(ctx context.Context, call *calls.CallAudio, objKey AudioRef) error {
	return sb.delete(ctx, objKey.Ref(sb.st.PartMan(), call.CallDate.Time()))
}

func (sb *s3Backend) DeleteBulk(ctx context.Context, refs []AbsoluteRef) error {
	objCh := make(chan minio.ObjectInfo)

	go func() {
		defer close(objCh)

		for _, ref := range refs {
			objCh <- minio.ObjectInfo{Key: ref.String()}
		}
	}()

	var err error
	var notFoundCount int64
	for rErr := range sb.cli.RemoveObjects(ctx, sb.Bucket, objCh, minio.RemoveObjectsOptions{}) {
		if rErr.Error() == "Key not found" {
			notFoundCount++
			continue
		}

		err = multierror.Append(err, &rErr)
	}

	if notFoundCount > 0 {
		err = multierror.Append(fmt.Errorf("Key not found (x%d)", notFoundCount))
	}

	return err
}

func (sb *s3Backend) ctxTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if sb.Timeout == 0 {
		return ctx, func() {}
	}

	return context.WithTimeout(ctx, sb.Timeout)
}

func init() {
	RegisterAudioBackend("s3", newS3backend)
}

func newS3backend(s Store, cfg config.ConfigMap) (AudioBackend, error) {
	sb := &s3Backend{
		st: s,
	}

	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           sb,
		TagName:          "yaml",
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.TextUnmarshallerHookFunc(),
		),
	})
	if err != nil {
		return nil, err
	}

	err = dec.Decode(cfg)
	if err != nil {
		return nil, err
	}

	var rt http.RoundTripper
	if sb.Trace || os.Getenv("STILLBOX_S3_TRACE") == "true" {
		rt = common.LoggingRoundTripper()
	}

	cli, err := minio.New(sb.Endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(sb.KeyID, sb.SecretKey, ""),
		Secure:    sb.Secure,
		Region:    sb.Region,
		Transport: rt,
	})
	if err != nil {
		return nil, err
	}

	sb.cli = cli

	return sb, nil
}
