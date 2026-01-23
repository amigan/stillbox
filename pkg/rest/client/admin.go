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
	CallsGC(ctx context.Context) error
	CallsFsck(ctx context.Context) error 
}

type ProgressCallback func(callstore.MoveProgressMsg)

func (c *client) MoveCalls(ctx context.Context, p *callstore.MoveCallParams, progressCb ProgressCallback) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err := c.POST(ctx, "/admin/move-calls", p)
	if err != nil {
		return err
	}

	setSSErequestHeaders(req)

	cb := func(m callstore.MoveProgressMsg) error {
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

	_ = os.Stderr
	b := resp.Body

	if resp.StatusCode != 200 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		var m callstore.MoveProgressMsg
		err = json.Unmarshal(body, &m)
		if err != nil {
			return fmt.Errorf("decoding '%s': %w", string(body), err)
		}

		err = cb(m)
		return err
	}

	ch, err := sseSubscribe[callstore.MoveProgressMsg](b)
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

func (c *client) CallsGC(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err := c.POST(ctx, "/admin/callsgc", "")
	if err != nil {
		return err
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		return errors.New(string(body))
	}

	return nil
}
