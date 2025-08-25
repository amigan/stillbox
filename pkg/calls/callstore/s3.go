package callstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

	lc  s3LifecycleCache
	cli *minio.Client
	st  Store
}

func (*s3Backend) Type() string { return "s3" }

const S3LifecycleTTL = 10 * time.Minute

type s3LifecycleCache struct {
	cfg     *lifecycle.Configuration
	tm      time.Time
	ruleMap map[string]lifecycle.Rule
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

	return audioRef, nil
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
	objKey, ok := ref.(string)
	if !ok {
		return nil, nil, ErrBadAudioRef
	}

	if opts != nil && opts.resolveBlob {
		blob, err = sb.getBlob(ctx, objKey)
	} else {
		audioURL, err = sb.generateSignedURL(ctx, call.AudioName, objKey)
	}

	return
}

func (sb *s3Backend) Prune(ctx context.Context, audioRef AudioRef, pruneAfter *time.Time) (*time.Time, error) {
	refPath, ok := audioRef.(string)
	if !ok {
		return nil, ErrBadAudioRef
	}

	if pruneAfter != nil { // this has already been pruned, now check if the rule needs to be removed yet
		if !time.Now().After(*pruneAfter) {
			// this probably won't ever happen
			return nil, ErrNotYetPruneTime
		}

		return nil, sb.pruneRmRule(ctx, refPath)
	}

	if !strings.HasSuffix(refPath, "/") {
		// singleton remove
		return nil, sb.delete(ctx, refPath)
	}

	err := sb.addRmRule(ctx, refPath)
	if err != nil {
		return nil, fmt.Errorf("add lifecycle rule: %w", err)
	}

	newPruneAfter := time.Now().Add(48 * time.Hour)
	return &newPruneAfter, nil
}

func (sb *s3Backend) isNoSuchLifecycleConfig(err error) bool {
	var erR minio.ErrorResponse
	return errors.As(err, &erR) && erR.Code == "NoSuchLifecycleConfiguration"
}

func (sb *s3Backend) addRmRule(ctx context.Context, refPath string) error {
	lcCfg, err := sb.getRules(ctx)
	if err != nil {
		return err
	}

	if _, exists := sb.lc.ruleMap[refPath]; exists {
		return fmt.Errorf("rule exists for '%s'", refPath)
	}

	log.Debug().Str("prefix", refPath).Msg("add rm rule")
	lcCfg.Rules = append(lcCfg.Rules, lifecycle.Rule{
		ID:     refPath,
		Prefix: refPath,
		Expiration: lifecycle.Expiration{
			Days: lifecycle.ExpirationDays(1),
		},
	})

	return sb.setRules(ctx, lcCfg)
}

func (sb *s3Backend) pruneRmRule(ctx context.Context, refPath string) error {
	lcCfg, err := sb.getRules(ctx)
	if err != nil {
		return err
	}

	if _, exists := sb.lc.ruleMap[refPath]; !exists {
		return fmt.Errorf("rule doesn't exist for '%s'", refPath)
	}

	// filter
	r := lcCfg.Rules[:0]
	for _, x := range lcCfg.Rules {
		if x.ID == refPath {
			r = append(r, x)
		}
	}

	return sb.setRules(ctx, lcCfg)
}

func (sb *s3Backend) setRules(ctx context.Context, cfg *lifecycle.Configuration) error {
	err := sb.cli.SetBucketLifecycle(ctx, sb.Bucket, cfg)
	if err != nil {
		return fmt.Errorf("set bucket lifecycle: %w", err)
	}

	sb.lc.set(cfg)

	return nil
}

func (lc *s3LifecycleCache) set(cfg *lifecycle.Configuration) {
	lc.cfg = cfg
	lc.tm = time.Now()
	lc.ruleMap = make(map[string]lifecycle.Rule, len(lc.cfg.Rules))
	for _, r := range lc.cfg.Rules {
		lc.ruleMap[r.Prefix] = r
	}
}

func (sb *s3Backend) getRules(ctx context.Context) (*lifecycle.Configuration, error) {
	now := time.Now()

	if now.After(sb.lc.tm.Add(S3LifecycleTTL)) {
		lc, err := sb.cli.GetBucketLifecycle(ctx, sb.Bucket)
		if err != nil {
			if !sb.isNoSuchLifecycleConfig(err) {
				return nil, err
			}

			lc = lifecycle.NewConfiguration()
		}

		sb.lc.set(lc)

		return lc, nil
	}

	return sb.lc.cfg, nil
}

func (sb *s3Backend) delete(ctx context.Context, objKey string) error {
	return sb.cli.RemoveObject(ctx, sb.Bucket, objKey, minio.RemoveObjectOptions{})
}

func (sb *s3Backend) Delete(ctx context.Context, audioRef AudioRef) error {
	objKey, ok := audioRef.(string)
	if !ok {
		return ErrBadAudioRef
	}
	return sb.delete(ctx, objKey)
}

func (sb *s3Backend) DeleteBulk(ctx context.Context, refs []AudioRef) error {
	objCh := make(chan minio.ObjectInfo)

	go func() {
		defer close(objCh)

		for _, ref := range refs {
			ref := ref.(string)
			objCh <- minio.ObjectInfo{Key: ref}
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
	registerAudioBackend("s3", newS3backend)
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
	if sb.Trace {
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
