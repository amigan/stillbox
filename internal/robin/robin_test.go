package robin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func expectVals(t *testing.T, r Round[string], expect ...string) {
	compare := make([]string, 0, len(expect))
	for range expect {
		compare = append(compare, r.Next())
	}

	assert.Equal(t, expect, compare)
}

func robinWith[E comparable](e ...E) Round[E] {
	r := New[E]()
	for _, el := range e {
		r.Add(el)
	}

	return r
}

func TestRobin(t *testing.T) {
	r := robinWith("one", "two", "three")
	expectVals(t, r, "one", "two", "three", "one", "two", "three", "one")
}

func TestDelete(t *testing.T) {
	r := robinWith("one", "two", "three")
	expectVals(t, r, "one", "two", "three", "one", "two", "three", "one")
	r.Delete("two")
	expectVals(t, r, "three", "one", "three", "one")
}

func TestRobinSingle(t *testing.T) {
	r := robinWith("one")
	expectVals(t, r, "one", "one", "one", "one")
}

func TestOverflow(t *testing.T) {
	r := robinWith("one", "two", "three", "four")
	rnd := r.(*round[string])
	rnd.i = ^uint(0)
	expectVals(t, r, "four", "one", "two", "three", "four", "one", "two", "three", "four")
	assert.Equal(t, uint(8), rnd.i)
}
