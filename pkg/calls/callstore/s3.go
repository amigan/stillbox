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
	"dynatron.me/x/stillbox/pkg/config"
	"github.com/go-viper/mapstructure/v2"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
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

	cli *minio.Client
}

func (*s3Backend) Type() string { return "s3" }

func (sb *s3Backend) Store(ctx context.Context, call *calls.Call) (AudioRef, error) {
	key := blobPath(call)

	dctx, cancel := sb.ctxTimeout(ctx)
	defer cancel()

	_, err := sb.cli.PutObject(dctx, sb.Bucket, key, bytes.NewReader(call.Audio), int64(len(call.Audio)), minio.PutObjectOptions{ContentType: call.AudioType})
	if err != nil {
		return nil, err
	}

	return key, nil
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

func (sb *s3Backend) Get(ctx context.Context, call *calls.CallAudio, ref AudioRef, opts *CallAudioOptions) (blob []byte, audioURL *url.URL, err error) {
	objKey, ok := ref.(string)
	if !ok {
		return nil, nil, ErrBadAudioRef
	}

	if opts != nil && opts.resolveBlob {
		blob, err = sb.getBlob(ctx, objKey)
	} else {
		audioURL, err = sb.getURL(ctx, call.AudioName, objKey)
	}

	return
}

func (sb *s3Backend) Delete(ctx context.Context, audioRef AudioRef) error {
	objKey, ok := audioRef.(string)
	if !ok {
		return ErrBadAudioRef
	}

	return sb.cli.RemoveObject(ctx, sb.Bucket, objKey, minio.RemoveObjectOptions{})
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
