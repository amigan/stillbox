package callstore

import (
	"context"
	"fmt"

	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/database"
	"github.com/goccy/go-json"
	"github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx/v5"
)

type MoveCallAudioOptions struct {
	// If Copy is true, the old object is not deleted.
	// Dangling references will never be left.
	Copy bool

	DryRun bool
}

type MoveCallSrcParams struct {
	CallsParams

	// If SweptCalls is true, all other parameters are ignored
	// and only swept calls are operated on.
	SweptCalls *bool `json:"sweptCalls"`

	// If HasBackend is not nil, it selects calls that have the specified backend as their storage.
	HasBackend *string `json:"hasBackend"`

	// If HasBlob is true, it selects calls that have an audio blob set.
	HasBlob bool `json:"hasBlob"`
}

const batchSize = 1000

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

func (ab *store) moveRow(ctx context.Context, tx database.Store, rm *refManager, row database.GetCallAudioRow, dst AudioBackend, opts MoveCallAudioOptions) error {
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

	// we aren't copying, so queue a clear of all existing audiorefs
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

func (s *store) MoveCallAudio(ctx context.Context, src MoveCallSrcParams, dstBackend string, opts MoveCallAudioOptions) (numRows int, err error) {
	// TODO add authz
	var dst AudioBackend
	if dstBackend != "" {
		dst = s.audioBackends.Backend(dstBackend)
		if dst == nil {
			return 0, fmt.Errorf("no such backend '%s'", dstBackend)
		}
	} // otherwise dst is the database

	mt := newRefManager(s.audioBackends, dstBackend, dst)
	err = s.db.InTx(ctx, func(tx database.Store) error {
		var rows []database.GetCallAudioRow
		for err == nil {
			rows, err = tx.GetCallAudio(ctx, database.GetCallAudioParams{
				Count:      batchSize,
				Swept:      src.SweptCalls,
				Start:      src.Start.PGTypeTSTZ(),
				End:        src.End.PGTypeTSTZ(),
				TagsAny:    src.TagsAny,
				TagsNot:    src.TagsNot,
				LongerThan: toPGNumericMilliseconds(src.AtLeastSeconds),
				HasBackend: src.HasBackend,
				HasBlob:    src.HasBlob,
			})
			if err != nil {
				return err
			}

			if opts.DryRun {
				numRows += len(rows)
				continue
			}

			for _, row := range rows {
				err = s.moveRow(ctx, tx, mt, row, dst, opts)
				if err != nil {
					return fmt.Errorf("call %s: %w", row.ID, err)
				}

				// increment successful rows
				numRows++
			}
		}

		return err
	}, pgx.TxOptions{})

	if err != nil {
		rbErr := mt.Rollback(ctx)
		if rbErr != nil {
			err = multierror.Append(err, rbErr)
		}
	} else {
		err = mt.Commit(ctx)
	}

	return
}
