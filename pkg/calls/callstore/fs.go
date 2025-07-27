package callstore

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/config"
	"github.com/go-viper/mapstructure/v2"
	"github.com/rs/zerolog/log"
)

type fsBackend struct {
	Root string `yaml:"root"`
}

func (*fsBackend) Type() string { return "fs" }

func (fsb *fsBackend) serveFile(w ZeroCopyResponseWriter, file *os.File, call *calls.CallAudio, opts *CallAudioOptions) error {
	st, err := file.Stat()
	if err != nil {
		return err
	}

	contentLength := st.Size()
	// ReadFrom will not call sendfile(2) if the Content-Length is not set in advance
	w.Header().Add("Content-Length", strconv.Itoa(int(contentLength)))

	if call.AudioType != nil && call.AudioName != nil {
		common.ContentDisposition(w.Header(), *call.AudioType, *call.AudioName, opts.isDownload)
	}
	w.WriteHeader(http.StatusOK)
	w.Flush()

	// without LimitReader, two sendfile(2) calls get emitted
	_, err = w.ReadFrom(io.LimitReader(file, contentLength))
	if err != nil {
		return err
	}

	return io.EOF // io.EOF is the sentinel that everything is all done
}

func (fsb *fsBackend) Get(ctx context.Context, call *calls.CallAudio, ref AudioRef, opts *CallAudioOptions) ([]byte, *url.URL, error) {
	refPath, ok := ref.(string)
	if !ok {
		log.Error().Str("refPath", fmt.Sprint(refPath)).Msg("call path was not a string")
		return nil, nil, ErrBadAudioRef
	}

	cPath := fsb.callPath(refPath)

	file, err := os.Open(cPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, ErrCallAudioNotFound
		}

		return nil, nil, err
	}
	defer file.Close()

	// special case: we have a ResponseWriter set. Emit the file directly out the socket.
	if w := opts.zcrw; w != nil {
		return nil, nil, fsb.serveFile(opts.zcrw, file, call, opts)
	}

	//  otherwise, read a blob buffer
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

	return os.Remove(path.Join(fsb.Root, refPath))
}

func (fsb *fsBackend) DeleteBulk(ctx context.Context, refs []AudioRef) error {
	for _, r := range refs {
		rErr := fsb.Delete(ctx, r)
		if rErr != nil {
			return rErr
		}
	}

	return nil
}

func (fsb *fsBackend) callPath(blobPath string) string {
	return path.Join(fsb.Root, blobPath)
}

func (fsb *fsBackend) Store(_ context.Context, call *calls.CallAudio) (AudioRef, error) {
	blobp := blobPath(call)
	p := fsb.callPath(blobp)
	err := os.WriteFile(p, call.AudioBlob, 0640)
	if err != nil {
		if os.IsNotExist(err) {
			cdir := path.Dir(p)
			err := os.MkdirAll(cdir, 0755)
			if err != nil {
				return nil, err
			}
		}

		err := os.WriteFile(p, call.AudioBlob, 0644)
		if err != nil {
			return nil, err
		}
	}

	return blobp, nil
}

func init() {
	registerAudioBackend("fs", newFSbackend)
}

func newFSbackend(cfg config.ConfigMap) (AudioBackend, error) {
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
