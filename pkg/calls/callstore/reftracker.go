package callstore

import (
	"context"
	"fmt"
	"sync"
)

type arTuple struct {
	b AudioBackend
	r AudioRef
}

type refTracker struct {
	sync.Mutex
	del     []arTuple    // deletes are queued until transaction commit
	cre     []AudioRef   // but creates are tracked for deletion on rollback
	dst     AudioBackend // cre all refers to one backend
	dstName string

	ab AudioBackends
}

func (rt *refTracker) reset() {
	rt.del = rt.del[:0]
	rt.cre = rt.cre[:0]
}

// Rollback deletes all created objects.
// Pass it a context without cancel.
func (rt *refTracker) Rollback(ctx context.Context) error {
	rt.Lock()
	defer rt.Unlock()

	if rt.dst == nil {
		return nil
	}

	return rt.dst.DeleteBulk(ctx, rt.cre)
}

// Commit deletes all queued-for-deletion objects.
// Pass it a context without cancel.
func (rt *refTracker) Commit(ctx context.Context) error {
	rt.Lock()
	defer rt.Unlock()

	m := make(map[AudioBackend][]AudioRef)
	for _, d := range rt.del {
		m[d.b] = append(m[d.b], d.r)
	}

	for b, rs := range m {
		err := b.DeleteBulk(ctx, rs)
		if err != nil {
			return err
		}
	}

	rt.reset()

	return nil
}

func (rt *refTracker) QueueDeleteAll(ar AudioRefList) error {
	for ben, loc := range ar {
		if ben == "" {
			continue
		}
		be := rt.ab.Backend(ben)
		if be == nil {
			return fmt.Errorf("queue delete all: no such backend '%s'", ben)
		}

		rt.QueueDelete(be, loc)
	}

	return nil
}

func (rt *refTracker) QueueDelete(ab AudioBackend, ar AudioRef) {
	rt.Lock()
	defer rt.Unlock()

	rt.del = append(rt.del, arTuple{ab, ar})
}

func (rt *refTracker) Created(ar AudioRef) {
	rt.Lock()
	defer rt.Unlock()

	rt.cre = append(rt.cre, ar)
}

func newRefTracker(ab AudioBackends, dstName string, dst AudioBackend) *refTracker {
	return &refTracker{
		ab:      ab,
		dst:     dst,
		dstName: dstName,
	}
}


