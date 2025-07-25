package client

import (
	"context"
	"encoding/json"
	"errors"

	"dynatron.me/x/stillbox/pkg/calls/callstore"
)

type adminClient interface {
	MoveCalls(ctx context.Context, p callstore.MoveCallParams) error
}

type ProgressMsg struct {
	Total     *int64  `json:"total,omitempty"`
	Final     *int64  `json:"final,omitempty"`
	Completed *int64  `json:"completed,omitempty"`
	Error     *string `json:"error,omitempty"`
}

type ProgressCallback func(ProgressMsg)

func (c *client) MoveCalls(ctx context.Context, p callstore.MoveCallParams, progressCb ProgressCallback) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err := c.POST(ctx, "/api/move-calls", p)
	if err != nil {
		return err
	}

	errCh := make(chan error)
	cb := func(msg []byte) {
		var m ProgressMsg
		err := json.Unmarshal(msg, &m)
		if err != nil {
			errCh <- err
			return
		}

		if m.Error != nil {
			errCh <- errors.New(*m.Error)
			return
		}

		progressCb(m)
	}

	err = c.sseSubscribe(req, cb)
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
	case err := <- errCh:
		return err
	}

	return nil
}
