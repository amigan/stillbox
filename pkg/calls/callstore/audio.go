package callstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/calls/filter"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/metrics"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"

	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

var (
	ErrCallAudioNotFound = errors.New("call audio not found")
	ErrNoAudioBlob       = errors.New("no call audio blob")
	ErrBadAudioRef       = errors.New("bad audio reference")
	ErrNXBackend         = errors.New("no such backend")
)

const (
	JournalErrThreshold = 4
)

type AudioRef any
type AudioRefJSON []byte
type AudioRefFQ struct {
	Backend *audioStorageBackend
	Ref     AudioRef
}

type AudioBackend interface {
	// Store stores a call in the backend. It returns the reference that, combined with the backend, can retrieve the call audio.
	Store(context.Context, *calls.CallAudio) (AudioRef, error)

	// Get retrieves a call from the backend using audioRef. If audioName is not nil and the backend returns a URL instead of a blob, the URL will result in a content-disposition of attachment rather than inline.
	Get(ctx context.Context, call *calls.CallAudio, audioRef AudioRef, opts *CallAudioOptions) (blob []byte, audioURL *url.URL, err error)

	// Delete deletes a call from the backend.
	Delete(ctx context.Context, audioRef AudioRef) error

	// Prune either does a special pruning operation (i.e. for S3, deletes a lifecycle rule) or deletes a call or prefix from the backend.
	Prune(ctx context.Context, audioRef AudioRef, pruneAfter *time.Time) (newPruneAfter *time.Time, err error)

	// DeleteBulk bulk deletes calls from the backend.
	DeleteBulk(ctx context.Context, refs []AudioRef) error

	// Type returns the backend's type.
	Type() string
}

type AudioBackends interface {
	// Store tries all backends and stores the call if any match.
	// If the call was stored, audioRef will be non-nil.
	// If the call was not stored, but not due to an error, AudioRef will be nil along with err.
	Store(ctx context.Context, call *calls.Call) (*AudioRefFQ, error)

	// CallAudio gets the call audio from the backend and location specified by audioRef.
	// It mutates the passed CallAudio with AudioBlob and/or AudioURL.
	// If io.EOF is returned as the error, the call audio was successfully sent over the wire and nothing more should be written to the connection.
	CallAudio(ctx context.Context, call *calls.CallAudio, audioRef AudioRefJSON, opts *CallAudioOptions) error

	// Prune delete the provided ref from the provided backend using the journal.
	// Semantics are similar to AudioBackend#Prune.
	Prune(ctx context.Context, beName string, ref AudioRef) error

	// Backend looks up a backend by name.
	Backend(name string) *audioStorageBackend

	// RefTracker() gets a refTracker from the pool.
	RefTracker() *refTracker

	// PutRefTracker puts a refTracker back in the pool.
	PutRefTracker(*refTracker)

	// JournalSizeMetric returns the prometheus metric for the given backendName and missing state.
	JournalSizeMetric(backendName string, missing bool) prometheus.Gauge
}

type audioBackends struct {
	backendList []string // in config order

	backends       map[string]*audioStorageBackend
	metrics        audioStorageMetrics
	refTrackerPool sync.Pool
	journal        RefJournal
}

func (ab *audioBackends) Backend(name string) *audioStorageBackend {
	be, has := ab.backends[name]
	if !has {
		return nil
	}

	return be
}

type audioStorageBackend struct {
	Name    string
	Filter  *filter.Filter
	OnError config.StorageDisposition
	AudioBackend
}

type audioStorageMetrics struct {
	TotalStores  *prometheus.CounterVec `help:"Total call stores" labels:"backend,type"`
	FailedStores *prometheus.CounterVec `help:"Failed call storage attempts by backend" labels:"backend,type"`
	JournalSize  *prometheus.GaugeVec   `help:"AudioRef journal size" labels:"backend,kind"`
}

const (
	MissingDeleteLabel = "delete"
	MissingCreateLabel = "create"
)

func (ab *audioBackends) JournalSizeMetric(backendName string, missing bool) prometheus.Gauge {
	missingLabel := MissingDeleteLabel
	if missing {
		missingLabel = MissingCreateLabel
	}

	return ab.metrics.JournalSize.WithLabelValues(backendName, missingLabel)
}

var _ AudioBackends = (*audioBackends)(nil)

type AudioRefList map[string]AudioRef

func (sb *audioBackends) CallAudio(ctx context.Context, call *calls.CallAudio, audioRef AudioRefJSON, opts *CallAudioOptions) (err error) {
	var refm AudioRefList
	if opts != nil && opts.audioRefOut != nil {
		refm = opts.audioRefOut
	}

	err = json.Unmarshal(audioRef, &refm)
	if err != nil {
		return
	}

	for backend, location := range refm {
		be, has := sb.backends[backend]
		if !has {
			err = fmt.Errorf("get call audio: %w '%s'", ErrNXBackend, backend)
			continue
		}
		call.AudioBlob, call.AudioURL, err = be.Get(ctx, call, location, opts)
		switch err {
		case nil, io.EOF:
			return
		default:
			continue // try next backend
		}
	}

	return
}

type PruneError struct {
	// err is the error.
	err error

	// DropError is whether this error describes a failure of an entry drop itself.
	DropError bool

	// JEID is the standing journal entry ID in case removal is desired.
	JEID JournalID
}

func (err *PruneError) Error() string {
	return err.err.Error()
}

func (err *PruneError) Unwrap() error {
	return err.err
}

func (err *PruneError) JournalID() JournalID {
	return err.JEID
}

func PruneErr(err error, jeid JournalID, dropErr bool) error {
	return &PruneError{
		err:       err,
		JEID:      jeid,
		DropError: dropErr,
	}
}

func (ab *audioBackends) Prune(ctx context.Context, beName string, ref AudioRef) error {
	be := ab.Backend(beName)
	if be == nil {
		return ErrNXBackend
	}

	rj, err := json.Marshal(ref)
	if err != nil {
		return err
	}

	jeid, err := ab.journal.AddDelete(ctx, beName, rj, nil)
	if err != nil {
		return err
	}

	pruneAfter, err := be.Prune(ctx, ref, nil) // nil pruneAfter because this is initial
	if err != nil {
		perr := PruneErr(err, jeid, false)
		ierr := ab.journal.Increment(ctx, jeid)
		if ierr != nil {
			perr = multierror.Append(perr, ierr)
		}

		return perr
	}

	if pruneAfter != nil {
		err := ab.journal.UpdatePruneAfter(ctx, jeid, pruneAfter)
		if err != nil {
			return PruneErr(err, jeid, false)
		}

		return nil
	}

	err = ab.journal.Drop(ctx, jeid)
	if err != nil {
		return PruneErr(err, jeid, true)
	}

	ab.RefTracker().ab.JournalSizeMetric(be.Name, false).Dec()

	return nil
}

func (ab *audioBackends) Store(ctx context.Context, call *calls.Call) (rfq *AudioRefFQ, err error) {
	for _, beName := range ab.backendList {
		be, has := ab.backends[beName]
		if !has {
			// this should never happen
			return nil, fmt.Errorf("%w '%s'", ErrNXBackend, beName)
		}

		res := be.Filter.Test(ctx, call)
		if !res {
			continue
		}

		var ref AudioRef
		ref, err = be.Store(ctx, call.ToCallAudio())
		if err != nil {
			ab.metrics.FailedStores.WithLabelValues(beName, be.Type()).Inc()

			switch be.OnError {
			case config.OnErrorFail:
				return nil, fmt.Errorf("backend '%s': %w", beName, err)
			case config.OnErrorDB:
				log.Error().Str("callID", call.ID.String()).Err(err).Msg("failed to store audio, storing in DB")
				return nil, nil
			case config.OnErrorNextThenDB:
				log.Error().Str("callID", call.ID.String()).Err(err).Msg("failed to store audio, trying next then storing in DB")
				err = nil // so if nobody else stores, it stores in DB
				continue
			case config.OnErrorNextThenFail: // default
				log.Error().Str("callID", call.ID.String()).Err(err).Msg("failed to store audio, trying next")
				continue
			}
		} else if ref != nil {
			ab.metrics.TotalStores.WithLabelValues(beName, be.Type()).Inc()
		}
		rfq = &AudioRefFQ{
			Backend: be,
			Ref:     ref,
		}

		return rfq, nil
	}

	return
}

type BackendFactory func(Store, config.ConfigMap) (AudioBackend, error)
type backendRegistry map[string]BackendFactory

var backendsRegistry = make(backendRegistry)

func registerAudioBackend(name string, f BackendFactory) {
	backendsRegistry[name] = f
}

func (ab *audioBackends) RefTracker() *refTracker {
	return ab.refTrackerPool.Get().(*refTracker)
}

func (ab *audioBackends) PutRefTracker(rt *refTracker) {
	ab.refTrackerPool.Put(rt)
}

func (s *store) MakeBackends(ctx context.Context, fc tgstore.FilterCache, met metrics.Metrics, cfg []config.CallStorage) (*audioBackends, error) {
	ab := &audioBackends{
		backendList: make([]string, 0, len(cfg)),
		backends:    make(map[string]*audioStorageBackend, len(cfg)),
	}

	ab.journal = NewRefJournal(ctx, ab, s, JournalErrThreshold)

	ab.refTrackerPool = sync.Pool{
		New: func() any {
			return newRefTracker(ab, ab.journal)
		},
	}

	met.Register("callaudio", &ab.metrics)

	for _, cf := range cfg {
		if cf.Name == "" {
			return nil, fmt.Errorf("blank name invalid")
		}

		if _, exists := ab.backends[cf.Name]; exists {
			return nil, fmt.Errorf("backend with duplicate name '%s'", cf.Name)
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

		makeBackend, hasBackendType := backendsRegistry[cf.Backend]
		if !hasBackendType {
			return nil, fmt.Errorf("%w '%s'", ErrNXBackend, cf.Backend)
		}

		be, err = makeBackend(s, cf.Config)
		if err != nil {
			return nil, fmt.Errorf("backend '%s': %w", cf.Name, err)
		}

		ab.backends[cf.Name] = &audioStorageBackend{
			Name:         cf.Name,
			Filter:       filt,
			OnError:      cf.OnError,
			AudioBackend: be,
		}

		if cf.Ingest {
			ab.backendList = append(ab.backendList, cf.Name)
		}
	}

	if journal := ab.RefTracker().journal; journal != nil {
		err := journal.PrimeMetrics(ctx)
		if err != nil {
			return nil, err
		}
	}

	return ab, nil
}

// DerefSweptCallAudios resolves all audio_refs of swept calls and puts the audio into audio_blob.
func (s *store) derefSweptCallAudios(ctx context.Context, tx database.Store) error {
	cas, err := tx.GetSweptCallsWithRef(ctx)
	if err != nil {
		return err
	}

	for _, ca := range cas {
		var out calls.CallAudio
		err := s.audioBackends.CallAudio(ctx, &out, ca.AudioRef, &CallAudioOptions{resolveBlob: true})
		if err != nil {
			return err
		}

		err = tx.SetSweptAudioAndClearRef(ctx, out.AudioBlob, ca.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *store) callPartitionPrefix(ca *calls.CallAudio) string {
	if pm := s.partman; pm != nil {
		return pm.PartitionPrefix(ca.CallDate.Time()) + "/"
	}

	return ""
}

// partitionizePath replaces any leading underscore components `_/` in path with the current partiiton prefix.
func (s *store) partitionizePath(path string, ca *calls.CallAudio) string {
	if s.partman == nil || !strings.HasPrefix(path, "_/") {
		return path
	}

	pre := s.partman.PartitionPrefix(ca.CallDate.Time())

	return pre + path[1:]
}

// BlobPath generates the audio path for FS and S3 backends. audioPath is the real path into
// the store, while audioRef will contain a leading `_/` if partitioning is enabled.
func (s *store) BlobPath(call *calls.CallAudio) (audioPath, audioRef string) {
	prefix := s.callPartitionPrefix(call)
	u := call.ID.String()

	var name string
	if call.AudioName != nil {
		name = "_" + *call.AudioName
	}

	audPath := string(u[0:2]) + "/" + string(u[2:3]) + "/" + u + name

	if prefix != "" {
		return prefix + audPath, "_/" + audPath
	}

	return audPath, audPath
}

func CallAudioFromCAIDRow(id uuid.UUID, row database.GetCallAudioByIDRow) *calls.CallAudio {
	return &calls.CallAudio{
		ID:        id,
		CallDate:  jsontypes.Time(row.CallDate.Time),
		AudioName: row.AudioName,
		AudioType: (*string)(&row.AudioType.AudioMIME),
		AudioBlob: row.AudioBlob,
	}
}

func (s *store) StoreAudioFromDB(ctx context.Context, callID uuid.UUID, back *audioStorageBackend) error {
	return s.db.InTx(ctx, func(db database.Store) error {
		ca, err := db.GetCallAudioByID(ctx, callID)
		if err != nil {
			return err
		}

		if len(ca.AudioBlob) < 1 {
			return ErrNoAudioBlob
		}

		callAudio := CallAudioFromCAIDRow(callID, ca)

		ref, err := back.Store(ctx, callAudio)
		if err != nil {
			return err
		}

		refJSON, err := json.Marshal(map[string]any{back.Name: ref})
		if err != nil {
			return err
		}

		return db.SetCallAudio(ctx, callID, refJSON, nil)
	}, pgx.TxOptions{})
}
