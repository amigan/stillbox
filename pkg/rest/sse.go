package rest

// This file implements server-side events progress for long-running
// operations.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
)

type progressSender[T any] struct {
	progDone chan bool
	progCh chan T
	sentSSE atomic.Bool
	r *http.Request
	w http.ResponseWriter
	rc *http.ResponseController
}

// Chan returns the progress channel for use by the routine doing work.
func (ps *progressSender[T]) Chan() chan T {
	if ps == nil {
		return nil
	}

	return ps.progCh
}

// SSEBegun returns whether the first SSE message has been sent yet.
func (ps *progressSender[T]) SSEBegun() bool {
	if ps == nil {
		return false
	}

	return ps.sentSSE.Load()
}

// SendErr attempts to send an error in the event stream. If the event stream has not yet begun, it returns sent == false.
// newErr is any error resulting from the sending.
func (ps *progressSender[T]) SendErr(err error) (sent bool, newErr error) {
	if !ps.SSEBegun() {
		return false, nil
	}
	b, newErr := json.Marshal(map[string]string{"error": err.Error()})
	if err != nil {
		return false, newErr
	}
	_, newErr = fmt.Fprintf(ps.w, "data:%s\n", string(b))

	return true, newErr
}

func (ps *progressSender[T]) writeMsg(msg T) {
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		panic(err)
	}
	msgSt := "data:" + string(msgJSON) + "\n\n"
	_, _ = ps.w.Write([]byte(msgSt))
	ps.rc.Flush()
}

func (ps *progressSender[T]) connWorker() {
	defer close(ps.progDone)

	totalMsg, ok := <-ps.progCh
	if !ok {
		return
	}

	ps.sentSSE.Store(true)

	ps.w.Header().Set("Content-Type", "text/event-stream")
	ps.w.Header().Set("Cache-Control", "no-cache")
	ps.w.Header().Set("Connection", "keep-alive")

	ps.writeMsg(totalMsg)

	for msg := range ps.progCh {
		ps.writeMsg(msg)
	}

	ps.progDone <- true
}

// Close cleans up a progressSender. It returns whether final progress was sent.
func (ps *progressSender[T]) Close(finalCompleted T) bool {
	if ps == nil {
		return false
	}

	close(ps.progCh)
	<-ps.progDone
	ps.writeMsg(finalCompleted)

	return true
}

// NewProgressSender creates and starts a progressSender. The bool return is whether the connection accepts SSE.
func NewProgressSender[T any](w http.ResponseWriter, r *http.Request) (*progressSender[T], bool) {
	progress := strings.Contains(r.Header.Get("Accept"), "text/event-stream")
	if !progress {
		return nil, false
	}

	ps := &progressSender[T]{
		w: w,
		r: r,
		rc: http.NewResponseController(w),
		progDone: make(chan bool),
		progCh: make(chan T, 8),
	}

	go ps.connWorker()

	return ps, true
}
