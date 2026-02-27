// Package robin implements a simplistic atomic round robin dispatcher.
package robin

import (
	"errors"
	"sync"
)

var (
	ErrExists = errors.New("item already exists")
)

type Round[E comparable] interface {
	Next() E
	Delete(E)
	Add(E) error
}

type round[E comparable] struct {
	sync.Mutex

	m     map[E]struct{}
	items []E

	i uint
}

func (r *round[E]) Next() E {
	r.Lock()
	defer r.Unlock()

	var zero E

	if len(r.items) == 0 {
		return zero
	}

	r.i++
	return r.items[r.i%uint(len(r.items))]
}

func New[E comparable]() Round[E] {
	return &round[E]{
		m: make(map[E]struct{}),
	}
}

func (r *round[E]) has(e E) bool {
	_, has := r.m[e]

	return has
}

func (r *round[E]) Has(e E) bool {
	r.Lock()
	defer r.Unlock()

	return r.has(e)
}

func (r *round[E]) Add(e E) error {
	r.Lock()
	defer r.Unlock()

	if r.has(e) {
		return ErrExists
	}

	r.m[e] = struct{}{}
	r.items = append(r.items, e)

	return nil
}

func (r *round[E]) Delete(e E) {
	r.Lock()
	defer r.Unlock()

	if _, has := r.m[e]; !has {
		return
	}

	for i := range r.items {
		if r.items[i] == e {
			r.items[i] = r.items[len(r.items)-1]
			r.items = r.items[:len(r.items)-1]
			delete(r.m, e)

			return
		}
	}
}
