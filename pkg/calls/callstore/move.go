package callstore

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/database"

	"github.com/goccy/go-json"
	"github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

// NB: None of the move code uses the ref journal. This probably won't change unless a good reason arises.

// number of store workers
const numStoreWorkers = 16
const numStoreWorkersLimit = 128

type MoveCallParams struct {
	CallsParams

	// If SweptCalls is true, all other parameters are ignored
	// and only swept calls are operated on.
	SweptCalls *bool `json:"sweptCalls,omitempty" desc:"swept calls only" flag:"swept-calls S"`

	// If HasBackend is not nil, it selects calls that have the specified backend as their storage.
	HasBackend *string `json:"hasBackend,omitempty" desc:"calls that have specified backend" flag:"has-backend H"`

	// DestBackend specifies the destination backend. If nil or empty, the DB is used.
	DestBackend *string `json:"destBackend,omitempty" desc:"destination backend" flag:"destination D"`

	// If HasBlob is not nil, it selects calls that have an audio blob set or not.
	HasBlob *bool `json:"hasBlob,omitempty" desc:"only calls that have blob set" flag:"has-blob B"`

	// If Copy is true, the old object is not deleted.
	// Dangling references will never be left.
	Copy bool `json:"copy,omitzero" desc:"do not delete old audio object" flag:"copy c"`

	// DryRun specifies whether to just return the number of affected calls rather than actually moving.
	DryRun bool `json:"dryRun,omitzero" desc:"dry run" flag:"dry-run n"`

	// NumWorkers specifies the number of workers to use for the move. It is bounded internally by numStoreWorkersLimit.
	NumWorkers *uint `json:"numWorkers,omitempty" desc:"number of workers" flag:"workers w"`

	// Tries is the number of times to retry moving a file *iff* the worker comes back with context.DeadlineExceeded as an error.
	Tries int `json:"tries,omitzero" desc:"number of times to retry on deadline exceeded" flag:"tries t"`

	// ProgressChan, if not nil, is a channel where the number of rows is written as the call progresses.
	// It is closed by MoveCalls on finish (or error)
	ProgressChan chan int64 `json:"-"`
}

const (
	batchSize        = 5000
	progressInterval = 50
)

func GetCallAudioRowToSkinnyCallAudio(row *database.GetCallAudioRow) *calls.CallAudio {
	return &calls.CallAudio{
		ID:        row.ID,
		CallDate:  jsontypes.Time(row.CallDate),
		AudioName: row.AudioName,
		AudioType: (*string)(&row.AudioType.AudioMIME),
	}
}

func (m *mover) moveCallAudio(ctx context.Context, row *database.GetCallAudioRow) (ref AudioRefJSON, blob []byte, err error) {
	fromBlob := false

	cao := CallAudioOptions{
		resolveBlob: true,
	}

	// check this invariant for sanity
	if row.AudioBlob == nil && row.AudioRef == nil {
		return nil, nil, ErrCallAudioNotFound
	}

	// prepare a CallAudio without the blob
	ca := GetCallAudioRowToSkinnyCallAudio(row)

	// the blob comes from the database
	if row.AudioBlob != nil {
		ca.AudioBlob = row.AudioBlob
		fromBlob = true
	} else { // the blob comes from a ref
		// initialize audioRefOut map so we can benefit from CallAudio's unmarshaling
		cao.audioRefOut = make(AudioRefList)
		// get the blob, CallAudio() will fill in the CallAudio blob
		err := m.ab.CallAudio(ctx, ca, row.AudioRef, &cao)
		if err != nil {
			return nil, nil, err
		}
	}

	// if we aren't copying, queue a clear of all existing audiorefs
	if !m.par.Copy && len(cao.audioRefOut) > 0 {
		if err := m.refs.QueueDeleteAll(cao.audioRefOut, ca.CallDate.Time()); err != nil {
			return nil, nil, err
		}
		clear(cao.audioRefOut)
	}

	switch m.dst {
	case nil: // use database
		if fromBlob {
			// already in DB
			return nil, nil, fmt.Errorf("%s blob already in db", row.ID.String())
		}
		// set the final blob parameter for storage in the DB
		blob = ca.AudioBlob
	default:
		// store in backend
		crRef, err := m.dst.Store(ctx, ca)
		if err != nil {
			return nil, nil, fmt.Errorf("store: %w", err)
		}

		// storage succeeded, log the creation
		m.refs.Created(m.dst, AbsoluteRef(crRef.Ref(m.refs.st.PartMan(), row.CallDate)))

		if cao.audioRefOut == nil {
			cao.audioRefOut = make(AudioRefList)
		}

		cao.audioRefOut[m.dst.Name] = crRef
		if !m.par.Copy && fromBlob {
			// we are from the DB and copy is disabled and we are a ref, clear blob
			blob = nil
		}
	}

	// marshal audioRefOut to ref
	if len(cao.audioRefOut) > 0 {
		ref, err = json.Marshal(cao.audioRefOut)
		if err != nil {
			return nil, nil, err
		}
	}
	return
}

type mover struct {
	ab         AudioBackends
	numWorkers int
	tries      int
	dbTx       database.Store
	dbMtx      sync.Mutex
	refs       *refTracker

	completedRows atomic.Int64
	dst           *audioStorageBackend
	par           MoveCallParams
}

func (m *mover) moveWorker(ctx context.Context, row *database.GetCallAudioRow) error {
	ref, blob, err := m.moveCallAudio(ctx, row)
	if err != nil {
		return fmt.Errorf("call %s: %w", row.ID, err)
	}

	m.dbMtx.Lock()
	err = m.dbTx.SetCallAudio(ctx, row.ID, ref, blob)
	m.dbMtx.Unlock()
	if err != nil {
		return err
	}

	cr := m.completedRows.Add(1)
	if cr%progressInterval == 0 && m.par.ProgressChan != nil {
		m.par.ProgressChan <- cr
	}

	return nil
}

func backoff() {
	r := rand.Intn(20)
	time.Sleep(time.Duration(r*100) * time.Millisecond)
}

func (m *mover) do(ctx context.Context, dbPar database.GetCallAudioParams) error {
	m.dbMtx.Lock()
	count, err := m.dbTx.GetCallAudioCount(ctx, dbPar)
	m.dbMtx.Unlock()

	if err != nil {
		return fmt.Errorf("count: %w", err)
	}

	log.Info().Int64("count", count).Msg("move begin")

	if m.par.ProgressChan != nil {
		m.par.ProgressChan <- count // first message is always total
	}

	eg, wctx := errgroup.WithContext(ctx)
	eg.SetLimit(m.numWorkers)
	for count > 0 {
		m.dbMtx.Lock()
		rows, err := m.dbTx.GetCallAudio(wctx, dbPar)
		m.dbMtx.Unlock()
		if err != nil {
			return err
		}

		// if we are dry run, or there were no rows, we can finish now
		if len(rows) == 0 || m.par.DryRun {
			return nil
		}

		// XXX this might be racy since the lower bound of the interval is inclusive
		dbPar.Start = common.NilIfZero(rows[len(rows)-1].CallDate)

		count -= int64(len(rows))

		for _, row := range rows {
			eg.Go(func() (err error) {
				defer func() { // for errgroup
					if rec := recover(); rec != nil {
						err = common.FromPanicValue(rec)
						log.Error().Err(err).Msg("panic in worker")
					}
				}()

				for i := range m.tries + 1 {
					err = m.moveWorker(wctx, &row)
					switch {
					case err == nil:
						return nil
					case errors.Is(err, context.DeadlineExceeded):
						log.Error().Int("tries", i).Err(err).Msg("moveWorker")
						backoff()
						continue
					case errors.Is(err, context.Canceled):
						return err
					default: // err != nil
						log.Error().Err(err).Msg("moveWorker")
						return err
					}
				}

				return err
			})
		}
	}

	if err := eg.Wait(); err != nil {
		log.Info().Err(err).Msg("move done")

		return err
	}

	return nil
}

func (s *store) newMover(dst *audioStorageBackend, tx database.Store, rt *refTracker, par MoveCallParams) *mover {
	numWorkers := numStoreWorkers
	if par.NumWorkers != nil {
		numWorkers = min(int(*par.NumWorkers), numStoreWorkersLimit)
	}
	return &mover{
		ab:         s.audioBackends,
		dbTx:       tx,
		numWorkers: numWorkers,
		tries:      par.Tries,
		dst:        dst,
		par:        par,
		refs:       rt,
	}
}

var (
	ErrSrcDestSame    = errors.New("source and destination backend are the same")
	ErrMoveInProgress = errors.New("move in progress")
)

// MoveCallAudio moves calls from one audio backing store to another. It returns the number of rows moved.
func (s *store) MoveCallAudio(ctx context.Context, par MoveCallParams) (numRows int64, err error) {
	_, err = authz.Check(ctx, authz.UseResource(entities.ResourceCall), authz.WithActions(entities.ActionMoveCallAudio))
	if err != nil {
		return 0, err
	}

	if !s.moveInProgress.TryLock() {
		return 0, ErrMoveInProgress
	}

	var dst *audioStorageBackend

	if par.DestBackend != nil {
		dst = s.audioBackends.Backend(*par.DestBackend)
		if dst == nil {
			return 0, fmt.Errorf("move params: %w '%s'", ErrNXBackend, *par.DestBackend)
		}
	} else {
		// otherwise dst is the database, exclude already copied
		par.HasBlob = common.PtrTo(false)
	}

	if par.HasBackend != nil && par.DestBackend != nil && *par.HasBackend == *par.DestBackend {
		return 0, fmt.Errorf("%w '%s'", ErrSrcDestSame, *par.HasBackend)
	}
	dbPar := database.GetCallAudioParams{
		Count:         batchSize,
		Swept:         par.SweptCalls,
		Start:         (*time.Time)(par.Start),
		End:           (*time.Time)(par.End),
		TagsAny:       par.TagsAny,
		TagsNot:       par.TagsNot,
		LongerThan:    toPGNumericMilliseconds(par.AtLeastSeconds),
		HasBackend:    par.HasBackend,
		HasBlob:       par.HasBlob,
		NotHasBackend: par.DestBackend, // not already moved
	}

	refT := newRefTracker(s.audioBackends, s, nil)

	err = s.db.InTx(context.WithoutCancel(ctx), func(tx database.Store) error {
		m := s.newMover(dst, tx, refT, par)
		err = m.do(ctx, dbPar)
		numRows = m.completedRows.Load()

		if par.ProgressChan != nil {
			par.ProgressChan <- numRows - 1
		}

		return err
	}, pgx.TxOptions{})

	if err != nil {
		go func() {
			// unlock only after the rollback finishes
			defer s.moveInProgress.Unlock()
			numRows = 0
			rbErr := refT.Rollback(context.WithoutCancel(ctx))
			if rbErr != nil {
				err = multierror.Append(err, rbErr)
			} else {
				log.Debug().Msg("move rollback finished")
			}
		}()
	} else {
		go func() {
			// unlock only after the commit finishes
			defer s.moveInProgress.Unlock()
			err := refT.Commit(context.WithoutCancel(ctx))
			if err != nil {
				log.Error().Err(err).Msg("move tx commit")
			} else {
				log.Debug().Msg("move commit finished")
			}
		}()
	}

	if par.ProgressChan != nil {
		close(par.ProgressChan)
	}

	return
}
