package callstore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/calls/filter"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"github.com/go-viper/mapstructure/v2"
	"github.com/goccy/go-json"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type AudioRef any

type AudioBackend interface {
	StoreCall(context.Context, *calls.Call) (AudioRef, error)
	GetCall(ctx context.Context, audioName *string, audioRef AudioRef, resolveBlob bool) (blob []byte, audioURL *url.URL, err error)
}

type AudioBackends interface {
	// Store tries all backends and stores the call if any match.
	// If the call was stored, audioRef will be non-nil.
	// If the call was not stored, but not due to an error, AudioRef will be nil along with err.
	Store(ctx context.Context, call *calls.Call) (AudioRefJSON, error)

	// CallAudio gets the call audio from the backend and location specified by audioRef.
	// It returns either a non-nil blob or url, but never both.
	// resolveBlob disables URL generation.
	CallAudio(ctx context.Context, audioName *string, audioRef AudioRefJSON, resolveBlob bool) (blob []byte, audioURL *url.URL, err error)
}

type audioBackends struct {
	storeList []string // in config order

	backends map[string]*audioStorageBackend
}

type audioStorageBackend struct {
	Name   string
	Filter *filter.Filter
	AudioBackend
}

type fsBackend struct {
	nameGen
}

func (fsb *fsBackend) GetCall(_ context.Context, _ *string, _ AudioRef, _ bool) ([]byte, *url.URL, error) {
	return nil, nil, fmt.Errorf("FS backend not implemented")
}

func (fsb *fsBackend) StoreCall(_ context.Context, _ *calls.Call) (AudioRef, error) {
	return nil, fmt.Errorf("FS backend not implemented")
}

func newFSbackend(cfg config.ConfigMap) (*fsBackend, error) {
	return nil, fmt.Errorf("FS backend not implemented")
}

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

	nameGen
	cli *minio.Client
}

type nameGen struct{}

func (nameGen) objectName(call *calls.Call) string {
	u := call.ID.String()
	return string(u[0]) + "/" + string(u[1:2]) + "/" + u + "/" + call.AudioName
}

type objectPath string

func (sb *s3Backend) StoreCall(ctx context.Context, call *calls.Call) (AudioRef, error) {
	key := sb.objectName(call)

	dctx, cancel := sb.ctxTimeout(ctx)
	defer cancel()

	_, err := sb.cli.PutObject(dctx, sb.Bucket, key, bytes.NewReader(call.Audio), int64(len(call.Audio)), minio.PutObjectOptions{ContentType: call.AudioType})
	if err != nil {
		return nil, err
	}

	return objectPath(key), nil
}

func (sb *s3Backend) getBlob(ctx context.Context, objKey string) ([]byte, error) {
	dctx, cancel := sb.ctxTimeout(ctx)
	defer cancel()

	b, err := sb.cli.GetObject(dctx, sb.Bucket, objKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}

	return io.ReadAll(b)
}

func (sb *s3Backend) getURL(ctx context.Context, audioName *string, objKey string) (*url.URL, error) {
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

func (sb *s3Backend) GetCall(ctx context.Context, audioName *string, ref AudioRef, resolveBlob bool) (blob []byte, audioURL *url.URL, err error) {
	objKey, ok := ref.(string)
	if !ok {
		return nil, nil, fmt.Errorf("reference was not a string")
	}

	if resolveBlob {
		blob, err = sb.getBlob(ctx, objKey)
	} else {
		audioURL, err = sb.getURL(ctx, audioName, objKey)
	}

	return
}

func (sb *s3Backend) ctxTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if sb.Timeout == 0 {
		return ctx, func() {}
	}

	return context.WithTimeout(ctx, sb.Timeout)
}

func newS3backend(_ context.Context, cfg config.ConfigMap) (*s3Backend, error) {
	sb := new(s3Backend)

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

	cli, err := minio.New(sb.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(sb.KeyID, sb.SecretKey, ""),
		Secure: sb.Secure,
		Region: sb.Region,
	})
	if err != nil {
		return nil, err
	}

	sb.cli = cli

	return sb, nil
}

var _ AudioBackends = (*audioBackends)(nil)

func (sb *audioBackends) CallAudio(ctx context.Context, audioName *string, audioRef AudioRefJSON, resolveBlob bool) (blob []byte, ur *url.URL, err error) {
	var refm map[string]AudioRef
	err = json.Unmarshal(audioRef, &refm)
	if err != nil {
		return
	}

	for backend, location := range refm {
		be, has := sb.backends[backend]
		if !has {
			err = fmt.Errorf("no such backend '%s'", backend)
			continue
		}
		blob, ur, err = be.GetCall(ctx, audioName, location, resolveBlob)
		if err == nil {
			break
		}
	}

	return
}

func (ab *audioBackends) Store(ctx context.Context, call *calls.Call) (arj AudioRefJSON, err error) {
	for _, beName := range ab.storeList {
		be, has := ab.backends[beName]
		if !has {
			// this should never happen
			return nil, fmt.Errorf("no such backend '%s'", beName)
		}

		res := be.Filter.Test(ctx, call)
		if !res {
			continue
		}

		var ref AudioRef
		ref, err = be.StoreCall(ctx, call)
		if err != nil {
			return nil, fmt.Errorf("backend '%s': %w", beName, err)
		}

		refMap := map[string]AudioRef{
			beName: ref,
		}

		return json.Marshal(refMap)
	}

	return
}

func MakeBackends(ctx context.Context, fc tgstore.FilterCache, cfg []config.CallStorage) (*audioBackends, error) {
	ab := &audioBackends{
		storeList: make([]string, 0, len(cfg)),
		backends:  make(map[string]*audioStorageBackend, len(cfg)),
	}

	for _, cf := range cfg {
		if cf.Name == "" {
			return nil, fmt.Errorf("blank name invalid")
		}

		var filt *filter.Filter
		var err error
		var be AudioBackend
		if cf.Filter != nil {
			filt, err = filter.FromMap(cf.Filter)
			if err != nil {
				return nil, fmt.Errorf("filter '%s': %w", cf.Name, err)
			}
		}

		switch cf.Backend {
		case "s3":
			be, err = newS3backend(ctx, cf.Config)
		case "fs":
			be, err = newFSbackend(cf.Config)
		default:
			return nil, fmt.Errorf("unknown backend '%s'", cf.Backend)
		}

		if err != nil {
			return nil, fmt.Errorf("backend '%s': %w", cf.Name, err)
		}

		ab.backends[cf.Name] = &audioStorageBackend{
			Name:         cf.Name,
			Filter:       filt,
			AudioBackend: be,
		}

		if !cf.ReadOnly {
			ab.storeList = append(ab.storeList, cf.Name)
		}
	}

	return ab, nil
}
