package robin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func expectVals(t *testing.T, r Robin[string], expect ...string) {
	compare := make([]string, 0, len(expect))
	for range expect {
		compare = append(compare, r.Next())
	}

	assert.Equal(t, expect, compare)
}

func TestRobin(t *testing.T) {
	r := New([]string{"one", "two", "three"})
	expectVals(t, r, "one", "two", "three", "one", "two", "three", "one")
}

func TestRobinSingle(t *testing.T) {
	r := New([]string{"one"})
	expectVals(t, r, "one", "one", "one", "one")
}

func TestOverflow(t *testing.T) {
	r := New([]string{"one", "two", "three", "four"})
	r.i = ^uint32(0)
	expectVals(t, r, "four", "one", "two", "three", "four", "one", "two", "three", "four")
	assert.Equal(t, r.i, uint32(8))
}
