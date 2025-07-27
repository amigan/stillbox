package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

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

	setSSErequestHeaders(req)

	cb := func(m ProgressMsg) error {
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

	defer resp.Body.Close()

	//b := io.TeeReader(resp.Body, os.Stderr)
	_ = os.Stderr
	b := resp.Body

	if resp.StatusCode != 200 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		var m ProgressMsg
		err = json.Unmarshal(body, &m)
		if err != nil {
			return fmt.Errorf("decoding '%s': %w", string(body), err)
		}

		err = cb(m)
		return err
	}

	ch, err := sseSubscribe[ProgressMsg](b)
	if err != nil {
		return err
	}

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
