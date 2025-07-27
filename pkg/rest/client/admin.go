package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"

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

func (c *client) MoveCalls(ctx context.Context, p *callstore.MoveCallParams, progressCb ProgressCallback) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err := c.POST(ctx, "/admin/move-calls", p)
	if err != nil {
		return err
	}

	cb := func(msg []byte) error {
		var m ProgressMsg
		if len(msg) < 1 {
			return nil
		}

		err := json.Unmarshal(msg, &m)
		if err != nil {
			return err
		}

		if m.Error != nil {
			return errors.New(*m.Error)
		}

		progressCb(m)
		return nil
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		err = cb(body)
		return err
	}

	ch, err := c.sseSubscribe(resp)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return nil
			}

			err := cb(msg)
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return context.Canceled
		}
	}
}
