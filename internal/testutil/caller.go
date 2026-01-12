package testutil

import (
	"iter"
	"math/rand/v2"
	"testing"
	"time"

	"dynatron.me/x/stillbox/internal/common"
)

// StatCaller returns an iterator that spreads numCalls over the period of time of numPartitions*interval with jitter (typically 0.1 or 10%) jitter.
// The times will always be monotonically incrementing.
func StatCaller(t *testing.T, baseTime time.Time, numCalls, numPartitions int, jitter float32, interval common.Interval) iter.Seq2[int, time.Time] {
	rng := rand.New(rand.NewPCG(uint64(baseTime.UnixNano()), uint64(0xdeadbeefcafed00d)))
	curTime := baseTime
	callsPerPart := numCalls / numPartitions
	callIntUpper := interval.Duration() / time.Duration(callsPerPart)
	callIntLower := callIntUpper - time.Duration(float32(callIntUpper)*jitter)

	return func(yield func(int, time.Time) bool) {
		for i := range numCalls {
			ivl := time.Duration(rng.Int64N(int64(callIntUpper-callIntLower))) + callIntLower
			curTime = curTime.Add(ivl)
			if !yield(i, curTime) {
				return
			}
		}
	}
}
