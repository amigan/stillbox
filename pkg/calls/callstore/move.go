package callstore

import (
	"context"
	"errors"
	"fmt"

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
	del     []arTuple    // deletes are queued until transaction commit
	cre     []AudioRef   // but creates are tracked for deletion on rollback
	dst     AudioBackend // cre all refers to one backend
	dstName string

	ab AudioBackends
}

// Rollback deletes all created objects.
func (rm *refManager) Rollback(ctx context.Context) error {
	if rm.dst == nil {
		return nil
	}
	return rm.dst.DeleteBulk(ctx, rm.cre)
}

// Commit deletes all queued-for-deletion objects.
func (rm *refManager) Commit(ctx context.Context) error {
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

func (gc *refManager) QueueDeleteAll(ar AudioRefList) error {
	for ben, loc := range ar {
		if ben == "" {
			continue
		}
		be := gc.ab.Backend(ben)
		if be == nil {
			return fmt.Errorf("no such backend '%s'", ben)
		}

		gc.QueueDelete(be, loc)
	}

	return nil
}

func (mt *refManager) QueueDelete(ab AudioBackend, ar AudioRef) {
	mt.del = append(mt.del, arTuple{ab, ar})
}

func (mt *refManager) Created(ar AudioRef) {
	mt.cre = append(mt.cre, ar)
}

func (ab *store) moveCallAudio(ctx context.Context, tx database.Store, rm *refManager, row database.GetCallAudioRow, dst AudioBackend, opts MoveCallParams) error {
	var blob []byte
	fromBlob := false

	cao := CallAudioOptions{
		resolveBlob: true,
	}

	// check this invariant for sanity
	if row.AudioBlob == nil && row.AudioRef == nil {
		return ErrCallAudioNotFound
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
			return err
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
			return fmt.Errorf("%s blob already in db", row.ID.String())
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
			return err
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

	var ref []byte
	var err error
	// marshal audioRefOut to ref
	if len(cao.audioRefOut) > 0 {
		ref, err = json.Marshal(cao.audioRefOut)
		if err != nil {
			return err
		}
	}

	err = tx.SetCallAudio(ctx, row.ID, ref, blob)
	if err != nil {
		return err
	}

	return nil
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
			Count:         batchSize,
			Swept:         par.SweptCalls,
			Start:         par.Start.PGTypeTSTZ(),
			End:           par.End.PGTypeTSTZ(),
			TagsAny:       par.TagsAny,
			TagsNot:       par.TagsNot,
			LongerThan:    toPGNumericMilliseconds(par.AtLeastSeconds),
			HasBackend:    par.HasBackend,
			HasBlob:       par.HasBlob,
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
				err = s.moveCallAudio(ctx, tx, mt, row, dst, par)
				if err != nil {
					return fmt.Errorf("call %s: %w", row.ID, err)
				}

				// increment successful rows
				numRows++
			}

			if par.ProgressChan != nil {
				par.ProgressChan <- numRows
			}
		}

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
