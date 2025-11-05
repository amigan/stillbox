package callstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

var (
	ErrNotYetPruneTime = errors.New("not yet time to prune")
)

type RefJournal interface {
	// AddCreate adds a create ref operation to the journal.
	AddCreate(ctx context.Context, callID uuid.UUID, backend string) (id JournalID, err error)

	// AddDelete adds a delete ref operation to the journal.
	AddDelete(ctx context.Context, backend string, ref string, pruneAfter *time.Time) (id JournalID, err error)

	// GC gets all failed refs meeting passed criteria and tries to prune them.
	// It returns the number of successful operations.
	// If errCh is not nil, errors are sent to it.
	GC(ctx context.Context, arg database.GetRefJournalParams, errCh chan<- error) (count, pruneCount int64, err error)

	// Increment increments the failure count of the operation.
	Increment(ctx context.Context, id JournalID) error

	// UpdatePruneAfter sets the entry's pruneAfter.
	UpdatePruneAfter(ctx context.Context, id JournalID, pruneAfter *time.Time) error

	// Drop drops a journal entry.
	Drop(ctx context.Context, id JournalID) error

	// Count gets a count of journal entries meeting criteria.
	Count(ctx context.Context, arg database.GetRefJournalParams) (int64, error)

	// PrimeMetrics sets the metric counters for journal entries stored in the database.
	PrimeMetrics(context.Context) error
}

type JournalID int64

type refJournal struct {
	ab           AudioBackends
	store        *store
	errThreshold int
}

func (rs *refJournal) AddCreate(ctx context.Context, callID uuid.UUID, backend string) (id JournalID, err error) {
	return rs.add(ctx, &callID, backend, "", nil, 1)
}

func (rs *refJournal) AddDelete(ctx context.Context, backend string, ref string, pruneAfter *time.Time) (id JournalID, err error) {
	return rs.add(ctx, nil, backend, ref, pruneAfter, 1)
}

func (rs *refJournal) add(ctx context.Context, callID *uuid.UUID, backend string, ref string, pruneAfter *time.Time, tries int) (JournalID, error) {
	var pA pgtype.Timestamptz
	if pruneAfter != nil {
		pA = pgtype.Timestamptz{Time: *pruneAfter, Valid: true}
	}

	var refJSON []byte
	var err error
	if ref != "" {
		refJSON, err = json.Marshal(ref)
		if err != nil {
			return JournalID(0), err
		}
	}

	jid, err := rs.store.db.AddRefJournal(ctx, database.AddRefJournalParams{
		CallID:     common.PGUUID(callID),
		Backend:    backend,
		Ref:        refJSON,
		PruneAfter: pA,
		Tries:      tries,
	})

	return JournalID(jid), err
}

func (rs *refJournal) Increment(ctx context.Context, id JournalID) error {
	return rs.store.db.IncrementRefJournal(ctx, int64(id))
}

func (rs *refJournal) UpdatePruneAfter(ctx context.Context, id JournalID, pruneAfter *time.Time) error {
	var pa pgtype.Timestamptz
	if pruneAfter != nil {
		pa = pgtype.Timestamptz{Valid: true, Time: *pruneAfter}
	}
	return rs.store.db.SetRefJournalPrune(ctx, int64(id), pa)
}

func (rs *refJournal) Drop(ctx context.Context, id JournalID) error {
	return rs.store.db.DropRefJournal(ctx, int64(id))
}

// GC enumerates the audio ref journal and attempts to prune any due entries.
// It removes any successful prunes and increments Tries on any failures.
func (rs *refJournal) GC(ctx context.Context, arg database.GetRefJournalParams, errCh chan<- error) (count, attempted int64, err error) {
	arg.Missing = common.PtrTo(false)
	errCounts := make(map[*audioStorageBackend]int)

	var back *audioStorageBackend
	var pj PruneJob

	commitPJ := func() {
		if pj != nil {
			// if there is an active prune job, try to commit it
			err := pj.Commit(ctx)
			if err != nil {
				errCh <- err
			}
			pj = nil
		}
	}

	// if NewPruneJob for a given backend fails, it will be in here
	newPruneJobErrors := make(map[*audioStorageBackend]error)

	err = rs.store.db.GetAudioRefJournalCb(ctx, arg, func(fr database.AudioRefJournal) {
		create := fr.Ref == nil

		var ref string
		rerr := json.Unmarshal(fr.Ref, &ref)
		if rerr != nil {
			if errCh != nil {
				errCh <- rerr
			}
			return
		}

		incrementTries := func() {
			errCounts[back]++
			if terr := rs.Increment(ctx, JournalID(fr.ID)); terr != nil {
				log.Error().Err(terr).Int64("journalEntry", fr.ID).Msg("tries increment")
			}
		}

		rowIsNewBackend := false
		// if this is the first row, or if we are onto a new backend (query is ordered by backend)
		if back == nil || back.Name != fr.Backend {
			commitPJ()

			back = rs.ab.Backend(fr.Backend)
			if back == nil {
				if errCh != nil {
					errCh <- fmt.Errorf("%w '%s'", ErrNXBackend, fr.Backend)
				}
				return
			}

			rowIsNewBackend = true
		}

		// if we could not create a prunejob for this backend, fail all others for it
		if _, hasErr := newPruneJobErrors[back]; hasErr {
			incrementTries()
			return
		}

		if rowIsNewBackend {
			if p, isBatchPruner := back.AudioBackend.(BatchPruner); !create && isBatchPruner {
				var err error
				pj, err = p.NewPruneJob(ctx)
				if err != nil {
					newPruneJobErrors[back] = err
					if errCh != nil {
						errCh <- fmt.Errorf("NewPruneJob '%s': %w", fr.Backend, err)
					}
					incrementTries()

					return
				}
			}
		}

		if rs.errThreshold > 0 && errCounts[back] > rs.errThreshold {
			return
		}

		jErr := func(err error) {
			errCh <- err

			rs.ab.JournalGCErrorMetric(back.Name, create).Inc()
		}

		var pruneAfter *time.Time
		if fr.PruneAfter.Valid {
			pruneAfter = &fr.PruneAfter.Time
		}

		var newPruneAfter *time.Time
		switch create {
		case true: // create
			if fr.CallID.Valid {
				rerr = rs.store.StoreAudioFromDB(ctx, uuid.UUID(fr.CallID.Bytes), back)
			} else { // shouldn't happen
				jErr(ErrCallAudioNotFound)
				return
			}
		case false: // delete
			mctx := ctx
			if pj != nil {
				mctx = CtxWithPruneJob(ctx, pj)
			}
			newPruneAfter, rerr = back.Prune(mctx, ref, pruneAfter)
			if rerr != nil {
				rerr = fmt.Errorf("%v: %w", ref, rerr)
			}
		}
		if rerr != nil {
			jErr(rerr)
			incrementTries()

			return
		}

		if newPruneAfter != nil {
			rerr = rs.UpdatePruneAfter(ctx, JournalID(fr.ID), newPruneAfter)
			if rerr != nil {
				jErr(rerr)
				return
			}
		} else {
			// drop the journal entry
			rerr = rs.Drop(ctx, JournalID(fr.ID))
			if rerr != nil {
				jErr(rerr)

				return
			}

			// Decrement journal size
			rs.ab.JournalSizeMetric(back.Name, create).Dec()
			count++
		}
		attempted++
	})

	commitPJ()

	return
}

func (rs *refJournal) Count(ctx context.Context, arg database.GetRefJournalParams) (int64, error) {
	return rs.store.db.CountRefJournal(ctx, arg.Missing, arg.Since, arg.Until)
}

func (rs *refJournal) PrimeMetrics(ctx context.Context) error {
	dbMetrics, err := rs.store.db.DetailedCountRefJournal(ctx)
	if err != nil {
		return err
	}

	for _, m := range dbMetrics {
		rs.ab.JournalSizeMetric(m.Backend, m.HasRef).Set(float64(m.Count))
	}

	return nil
}

// NewRefJournal creates a new reference journal. If errThreshold >0, it indicates the number
// of operations resulting in error before the backend is disabled for garbage collection by GC().
func NewRefJournal(ctx context.Context, ab AudioBackends, store *store, errThreshold int) RefJournal {
	rs := &refJournal{
		ab:           ab,
		store:        store,
		errThreshold: errThreshold,
	}

	return rs
}
