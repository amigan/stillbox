package callstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/config"
	"github.com/go-viper/mapstructure/v2"
)

type fsBackend struct {
	Root     string `yaml:"root"`
	rootStat os.FileInfo

	st Store
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

func (fsb *fsBackend) Get(ctx context.Context, call *calls.CallAudio, refPath AudioRef, opts *CallAudioOptions) ([]byte, *url.URL, error) {
	cPath := fsb.callPath(refPath.Ref(fsb.st.PartMan(), call.CallDate.Time()))

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

func (fsb *fsBackend) Prune(ctx context.Context, audioRef string, pruneAfter *time.Time) (*time.Time, error) {
	if pruneAfter != nil && !time.Now().After(*pruneAfter) {
		// this probably won't ever happen
		return nil, ErrNotYetPruneTime
	}

	composedPath, isDir, err := fsb.checkPath(audioRef)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// ENOENT; our work here is done
			return nil, nil
		}
		return nil, err
	}

	if !isDir { // single file remove
		return nil, os.Remove(composedPath)
	}

	if !strings.HasSuffix(composedPath, "/") {
		return nil, fmt.Errorf("'%s' is a directory but path does not end in '/'", composedPath)
	}

	return nil, os.RemoveAll(composedPath)
}

func (fsb *fsBackend) checkPath(refPath string) (composedPath string, isDir bool, err error) {
	if refPath == "" {
		err = ErrBadAudioRef
		return
	}

	if !filepath.IsLocal(refPath) {
		err = ErrBadAudioRef
		return
	}

	composedPath = fsb.callPath(refPath)
	st, err := os.Stat(composedPath)
	if err != nil {
		return
	}

	isDir = st.IsDir()
	if isDir {
		composedPath += "/"
	}

	if os.SameFile(st, fsb.rootStat) {
		err = errors.New("audio ref is the root")
		return
	}

	return
}

func (fsb *fsBackend) Delete(_ context.Context, call *calls.CallAudio, audioRef AudioRef) error {
	return fsb.delete(audioRef.Ref(fsb.st.PartMan(), call.CallDate.Time()))
}

func (fsb *fsBackend) delete(path string) error {
	composedPath, isDir, err := fsb.checkPath(path)
	if err != nil {
		return err
	}

	if isDir {
		return fmt.Errorf("'%s' is a directory", composedPath)
	}

	return os.Remove(composedPath)
}

func (fsb *fsBackend) DeleteBulk(ctx context.Context, refs []AbsoluteRef) error {
	for _, r := range refs {
		rErr := fsb.delete(r.String())
		if rErr != nil {
			return rErr
		}
	}

	return nil
}

// callPath composes an absolute path to the given call filename.
func (fsb *fsBackend) callPath(blobPath string) string {
	return path.Join(fsb.Root, blobPath)
}

const (
	// this could be configurable someday?
	FSDefaultMode          = 0640
	FSDefaultDirectoryMode = 0755
)

func (fsb *fsBackend) Store(_ context.Context, call *calls.CallAudio) (AudioRef, error) {
	audPath, audRef := fsb.st.BlobPath(call)
	p := fsb.callPath(audPath)
	err := os.WriteFile(p, call.AudioBlob, FSDefaultMode)
	if err != nil {
		switch os.IsNotExist(err) {
		case true:
			// try creating missing directories
			cdir := path.Dir(p)
			err := os.MkdirAll(cdir, FSDefaultDirectoryMode)
			if err != nil {
				return nil, err
			}

			// try to write again
			err = os.WriteFile(p, call.AudioBlob, FSDefaultMode)
			if err != nil {
				return nil, err
			}
		case false:
			return nil, err
		}
	}

	return makeAudioRef(audRef), nil
}

func init() {
	RegisterAudioBackend("fs", newFSbackend)
}

func newFSbackend(st Store, cfg config.ConfigMap) (AudioBackend, error) {
	fsb := &fsBackend{
		st: st,
	}

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

	fsb.rootStat = fi

	return fsb, nil
}
