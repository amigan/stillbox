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
	AddDelete(ctx context.Context, backend string, ref json.RawMessage, pruneAfter *time.Time) (id JournalID, err error)

	// GC gets all failed refs meeting passed criteria and tries to prune them.
	// It returns the number of successful operations.
	// If errCh is not nil, errors are sent to it.
	GC(ctx context.Context, arg database.GetRefJournalParams, errCh chan<- error) (int64, error)

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
	return rs.add(ctx, &callID, backend, nil, nil, 1)
}

func (rs *refJournal) AddDelete(ctx context.Context, backend string, ref json.RawMessage, pruneAfter *time.Time) (id JournalID, err error) {
	return rs.add(ctx, nil, backend, ref, pruneAfter, 1)
}

func (rs *refJournal) add(ctx context.Context, callID *uuid.UUID, backend string, ref json.RawMessage, pruneAfter *time.Time, tries int) (JournalID, error) {
	var pA pgtype.Timestamptz
	if pruneAfter != nil {
		pA = pgtype.Timestamptz{Time: *pruneAfter, Valid: true}
	}

	jid, err := rs.store.db.AddRefJournal(ctx, database.AddRefJournalParams{
		CallID:     common.PGUUID(callID),
		Backend:    backend,
		Ref:        ref,
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
func (rs *refJournal) GC(ctx context.Context, arg database.GetRefJournalParams, errCh chan<- error) (count int64, err error) {
	arg.Missing = common.PtrTo(false)
	errCounts := make(map[*audioStorageBackend]int)

	err = rs.store.db.GetAudioRefJournalCb(ctx, arg, func(fr database.AudioRefJournal) {
		log.Debug().Interface("journalEntry", fmt.Sprintf("%+v", fr)).Str("ref", string(fr.Ref)).Msg("journ")
		create := fr.Ref == nil

		var ref AudioRef
		rerr := json.Unmarshal(fr.Ref, &ref)
		if rerr != nil && errCh != nil {
			errCh <- rerr
		}

		back := rs.ab.Backend(fr.Backend)
		if back == nil {
			if errCh != nil {
				errCh <- fmt.Errorf("%w '%s'", ErrNXBackend, fr.Backend)
			}

			return
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
			newPruneAfter, rerr = back.Prune(ctx, ref, pruneAfter)
			if rerr != nil {
				rerr = fmt.Errorf("%v: %w", ref, rerr)
			}
		}
		if rerr != nil {
			jErr(rerr)
			errCounts[back] = errCounts[back] + 1
			if terr := rs.Increment(ctx, JournalID(fr.ID)); terr != nil {
				log.Error().Err(terr).Int64("journalEntry", fr.ID).Msg("tries increment")
			}

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
		}

		count++
	})

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
