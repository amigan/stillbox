package callstore

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog/log"
)

type beRefMap map[AudioBackend][]AbsoluteRef

func (brm beRefMap) reset() {
	for k := range brm {
		brm[k] = brm[k][:0]
	}
}

// A refTracker gives us transaction-ish semantics for audio storage backends. It is similar to a refJournal but without persistence, for increased performance for operations such as moving.
type refTracker struct {
	sync.Mutex
	del beRefMap // deletes are queued until transaction commit
	cre beRefMap // but creates are tracked for deletion on rollback

	ab      AudioBackends
	journal RefJournal
	st      Store
}

func (rt *refTracker) Reset() {
	rt.Lock()
	defer rt.Unlock()

	rt.reset()
}

func (rt *refTracker) reset() {
	rt.del.reset()
	rt.cre.reset()
}

// Rollback deletes all created objects.
// Pass it a context without cancel.
func (rt *refTracker) Rollback(ctx context.Context) error {
	rt.Lock()
	defer rt.Unlock()

	var err error
	for b, refs := range rt.cre {
		log.Debug().Str("type", b.Type()).Int("count", len(refs)).Msg("rolling back")
		bErr := b.DeleteBulk(ctx, refs)
		if bErr != nil {
			err = multierror.Append(err, bErr)
		}
	}

	return err
}

// Commit deletes all queued-for-deletion objects.
// Pass it a context without cancel.
func (rt *refTracker) Commit(ctx context.Context) error {
	rt.Lock()
	defer rt.Unlock()

	var err error
	for b, rs := range rt.del {
		bErr := b.DeleteBulk(ctx, rs)
		if bErr != nil {
			err = multierror.Append(err, bErr)
		}
	}

	if err != nil {
		return err
	}

	rt.reset()

	return nil
}

func (rt *refTracker) QueueDeleteAll(ar AudioRefList, callDate time.Time) error {
	for ben, loc := range ar {
		if ben == "" {
			continue
		}

		be := rt.ab.Backend(ben)
		if be == nil {
			return fmt.Errorf("queue delete all: no such backend '%s'", ben)
		}

		rt.QueueDelete(be, AbsoluteRef(loc.Ref(rt.st.PartMan(), callDate)))
	}

	return nil
}

func (rt *refTracker) QueueDelete(be AudioBackend, ar AbsoluteRef) {
	rt.Lock()
	defer rt.Unlock()

	rt.del[be] = append(rt.del[be], ar)
}

func (rt *refTracker) Created(be AudioBackend, ar AbsoluteRef) {
	rt.Lock()
	defer rt.Unlock()

	rt.cre[be] = append(rt.cre[be], ar)
}

// newRefTracker creates a new ref tracker. If journal is nil, journaling is disabled.
func newRefTracker(ab AudioBackends, st Store, journal RefJournal) *refTracker {
	return &refTracker{
		ab:      ab,
		cre:     make(beRefMap),
		del:     make(beRefMap),
		journal: journal,
		st:      st,
	}
}
