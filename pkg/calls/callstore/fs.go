package callstore

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"

	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/config"
	"github.com/go-viper/mapstructure/v2"
	"github.com/rs/zerolog/log"
)

type fsBackend struct {
	Root string `yaml:"root"`
}

func (*fsBackend) Type() string { return "fs" }

func (fsb *fsBackend) Get(_ context.Context, audioName *string, ref AudioRef, _ bool) ([]byte, *url.URL, error) {
	refPath, ok := ref.(string)
	if !ok {
		log.Error().Str("refPath", fmt.Sprint(refPath)).Msg("call path was not a string")
		return nil, nil, ErrBadAudioRef
	}

	cPath := fsb.callPath(refPath)

	// it would be nice to be able to use sendfile(2) here
	audio, err := os.ReadFile(cPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, ErrCallAudioNotFound
		}

		return nil, nil, err
	}

	return audio, nil, nil
}

func (fsb *fsBackend) Delete(_ context.Context, audioRef AudioRef) error {
	refPath, ok := audioRef.(string)
	if !ok {
		return ErrBadAudioRef
	}

	return os.Remove(refPath)
}

func (fsb *fsBackend) callPath(blobPath string) string {
	return path.Join(fsb.Root, blobPath)
}

func (fsb *fsBackend) Store(_ context.Context, call *calls.Call) (AudioRef, error) {
	blobp := blobPath(call)
	p := fsb.callPath(blobp)
	err := os.WriteFile(p, call.Audio, 0640)
	if err != nil {
		if os.IsNotExist(err) {
			cdir := path.Dir(p)
			err := os.MkdirAll(cdir, 0755)
			if err != nil {
				return nil, err
			}
		}

		err := os.WriteFile(p, call.Audio, 0644)
		if err != nil {
			return nil, err
		}
	}

	return blobp, nil
}

func newFSbackend(cfg config.ConfigMap) (*fsBackend, error) {
	fsb := new(fsBackend)

	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           fsb,
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

	fsb.Root = strings.TrimRight(fsb.Root, "/")

	fi, err := os.Stat(fsb.Root)
	if err != nil {
		return nil, fmt.Errorf("root: %w", err)
	}

	if !fi.IsDir() {
		return nil, fmt.Errorf("fs backend root '%s' is not a directory", fsb.Root)
	}

	return fsb, nil
}
