package callstore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/database"
	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
)

type MoveCallParams struct {
	CallsParams

	// If SweptCalls is true, all other parameters are ignored
	// and only swept calls are operated on.
	SweptCalls *bool `json:"sweptCalls"`

	// If HasBackend is not nil, it selects calls that have the specified backend as their storage.
	HasBackend *string `json:"hasBackend"`

	// DestBackend specifies the destination backend. If nil or empty, the DB is used.
	DestBackend *string `json:"destBackend"`

	// If HasBlob is not nil, it selects calls that have an audio blob set or not.
	HasBlob *bool `json:"hasBlob"`

	// If Copy is true, the old object is not deleted.
	// Dangling references will never be left.
	Copy bool `json:"copy"`

	// DryRun specifies whether to just return the number of affected calls rather than actually moving.
	DryRun bool `json:"dryRun"`

	// ProgressChan is a channel where the number of rows is written as the call progresses if not nil.
	// It is closed by MoveCalls on finish (or error)
	ProgressChan chan int `json:"-"`
}

const batchSize = 1000 // incidentally this is also the number of keys S3 allows in a DeleteObjects call

func getCallAudioRowToSkinnyCallAudio(row database.GetCallAudioRow) *calls.CallAudio {
	return &calls.CallAudio{
		ID:        row.ID,
		CallDate:  jsontypes.Time(row.CallDate.Time),
		AudioName: row.AudioName,
		AudioType: (*string)(&row.AudioType.AudioMIME),
	}
}

type arTuple struct {
	b AudioBackend
	r AudioRef
}

type refManager struct {
	sync.Mutex
	del     []arTuple    // deletes are queued until transaction commit
	cre     []AudioRef   // but creates are tracked for deletion on rollback
	dst     AudioBackend // cre all refers to one backend
	dstName string

	ab AudioBackends
}

// Rollback deletes all created objects.
func (rm *refManager) Rollback(ctx context.Context) error {
	rm.Lock()
	defer rm.Unlock()

	if rm.dst == nil {
		return nil
	}
	return rm.dst.DeleteBulk(ctx, rm.cre)
}

// Commit deletes all queued-for-deletion objects.
func (rm *refManager) Commit(ctx context.Context) error {
	rm.Lock()
	defer rm.Unlock()

	m := make(map[AudioBackend][]AudioRef)
	for _, d := range rm.del {
		m[d.b] = append(m[d.b], d.r)
	}

	for b, rs := range m {
		err := b.DeleteBulk(ctx, rs)
		if err != nil {
			return err
		}
	}

	return nil
}

func newRefManager(ab AudioBackends, dstName string, dst AudioBackend) *refManager {
	return &refManager{
		ab:      ab,
		dst:     dst,
		dstName: dstName,
	}
}

func (rm *refManager) QueueDeleteAll(ar AudioRefList) error {
	for ben, loc := range ar {
		if ben == "" {
			continue
		}
		be := rm.ab.Backend(ben)
		if be == nil {
			return fmt.Errorf("no such backend '%s'", ben)
		}

		rm.QueueDelete(be, loc)
	}

	return nil
}

func (rm *refManager) QueueDelete(ab AudioBackend, ar AudioRef) {
	rm.Lock()
	defer rm.Unlock()

	rm.del = append(rm.del, arTuple{ab, ar})
}

func (rm *refManager) Created(ar AudioRef) {
	rm.Lock()
	defer rm.Unlock()

	rm.cre = append(rm.cre, ar)
}

type updateRequest struct {
	id uuid.UUID
	blob []byte
	ref AudioRefJSON
}

func (ab *store) moveCallAudio(ctx context.Context, rm *refManager, row database.GetCallAudioRow, dst AudioBackend, opts *MoveCallParams) (ref AudioRefJSON, blob []byte, err error) {
	fromBlob := false

	cao := CallAudioOptions{
		resolveBlob: true,
	}

	// check this invariant for sanity
	if row.AudioBlob == nil && row.AudioRef == nil {
		return nil, nil, ErrCallAudioNotFound
	}

	// prepare a CallAudio without the blob
	ca := getCallAudioRowToSkinnyCallAudio(row)

	// the blob comes from the database
	if row.AudioBlob != nil {
		ca.AudioBlob = row.AudioBlob
		fromBlob = true
	} else { // the blob comes from a ref
		// initialize audioRefOut map so we can benefit from CallAudio's unmarshaling
		cao.audioRefOut = make(AudioRefList)
		// get the blob, CallAudio() will fill in the CallAudio blob
		err := ab.audioBackends.CallAudio(ctx, ca, row.AudioRef, &cao)
		if err != nil {
			return nil, nil, err
		}
	}

	// if we aren't copying, queue a clear of all existing audiorefs
	if !opts.Copy && len(cao.audioRefOut) > 0 {
		rm.QueueDeleteAll(cao.audioRefOut)
		clear(cao.audioRefOut)
	}

	switch dst {
	case nil: // use database
		if fromBlob {
			// already in DB
			return nil, nil, fmt.Errorf("%s blob already in db", row.ID.String())
		}
		// set the final blob parameter for storage in the DB
		blob = ca.AudioBlob
	default:
		// store in backend
		crRef, err := dst.Store(ctx, ca)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				// the call may have completed; log it anyway
				rm.Created(crRef)
			}
			return nil, nil, err
		}

		// storage succeeded, log the creation
		rm.Created(crRef)

		if cao.audioRefOut == nil {
			cao.audioRefOut = make(AudioRefList)
		}

		cao.audioRefOut[rm.dstName] = crRef
		if !opts.Copy && fromBlob {
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

func (s *store) MoveCallAudio(ctx context.Context, par MoveCallParams) (numRows int, err error) {
	_, err = authz.Check(ctx, authz.UseResource(entities.ResourceCall), authz.WithActions(entities.ActionMoveCallAudio))
	if err != nil {
		return 0, err
	}
	var dst AudioBackend
	var destBackend string
	if par.DestBackend != nil {
		destBackend = *par.DestBackend
	}
	if destBackend != "" {
		dst = s.audioBackends.Backend(destBackend)
		if dst == nil {
			return 0, fmt.Errorf("no such backend '%s'", *par.DestBackend)
		}
	} else {
		// otherwise dst is the database
		par.HasBlob = common.PtrTo(false)
	}

	if par.HasBackend != nil && par.DestBackend != nil && *par.HasBackend == *par.DestBackend {
		return 0, fmt.Errorf("source hasBackend same as destination backend '%s'", *par.HasBackend)
	}

	mt := newRefManager(s.audioBackends, destBackend, dst)
	err = s.db.InTx(ctx, func(tx database.Store) error {
		dbPar := database.GetCallAudioParams{
				Count:      batchSize,
				Swept:      par.SweptCalls,
				Start:      par.Start.PGTypeTSTZ(),
				End:        par.End.PGTypeTSTZ(),
				TagsAny:    par.TagsAny,
				TagsNot:    par.TagsNot,
				LongerThan: toPGNumericMilliseconds(par.AtLeastSeconds),
				HasBackend: par.HasBackend,
				HasBlob:    par.HasBlob,
				NotHasBackend: par.DestBackend, // not already moved
		}
		var rows []database.GetCallAudioRow
		count, err := tx.GetCallAudioCount(ctx, dbPar)
		if err != nil {
			return fmt.Errorf("count: %w", err)
		}


		if par.ProgressChan != nil {
			par.ProgressChan <- int(count) // first message is always total
		}

		const numStoreWorkers = 4

		moveCh := make(chan database.GetCallAudioRow, numStoreWorkers)
		errCh := make(chan error, numStoreWorkers)
		resCh := make(chan updateRequest, numStoreWorkers)

		ctx, cancel := context.WithCancel(ctx)

		var totalRows int64

		var wg sync.WaitGroup
		for range numStoreWorkers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for  {
					select {
					case row, ok := <-moveCh:
						if !ok {
							return
						}
						ref, blob, err := s.moveCallAudio(ctx, mt, row, dst, &par)
						if err != nil {
							errCh <- fmt.Errorf("call %s: %w", row.ID, err)
							cancel()
							return
						}
						resCh <- updateRequest{id: row.ID, blob: blob, ref: ref}
					case <-ctx.Done():
						return
					}
				}
			}()
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case res, ok := <-resCh:
					if !ok {
						return
					}
					err := tx.SetCallAudio(ctx, res.id, res.ref, res.blob)
					if err != nil {
						errCh <- err
						cancel()
					}
					// increment successful rows
					totalRows++
					if totalRows % batchSize == 0 && par.ProgressChan != nil {
						par.ProgressChan <- batchSize
					}
				case <-ctx.Done():
					return
				}
			}
		}()



		batchLoop:
		for counter := 0; err == nil; counter++ {
			rows, err = tx.GetCallAudio(ctx, dbPar)
			if err != nil {
				return err
			}

			if len(rows) == 0 {
				break batchLoop
			}

			if par.DryRun {
				if counter > 1 {
					break batchLoop
				}
				continue
			}


			for _, row := range rows {
				moveCh <- row
			}
		}
		wg.Wait()

		close(moveCh)
		close(errCh)
		close(resCh)

		return err
	}, pgx.TxOptions{})


	if err != nil {
		rbErr := mt.Rollback(context.WithoutCancel(ctx))
		if rbErr != nil {
			err = multierror.Append(err, rbErr)
		}
	} else {
		go func() {
			err := mt.Commit(ctx)
			if err != nil {
				log.Error().Err(err).Msg("move refTracker commit")
			}
		}()
	}

	if par.ProgressChan != nil {
		close(par.ProgressChan)
	}

	return
}
