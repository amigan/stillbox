// Package robin implements a simplistic atomic round robin dispatcher.
package robin

import (
	"sync/atomic"
)

type Robin[E any] interface {
	Next() E
}

type robin[E any] struct {
	items []E

	i uint32 // 32b so we don't hard depend on SIMD for atomic
}

func (r *robin[E]) Next() E {
	i := atomic.AddUint32(&r.i, 1) - 1
	return r.items[i%uint32(len(r.items))]
}

func New[E any](items []E) *robin[E] {
	return &robin[E]{
		items: items,
	}
}
