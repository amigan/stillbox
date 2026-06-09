package stillbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"dynatron.me/x/stillbox/pkg/calls/callstore"
)

type adminClient interface {
	MoveCalls(ctx context.Context, p *callstore.MoveCallParams, progressCb MoveProgressCallback) error
	CallsGC(ctx context.Context) error
	CallsFsck(ctx context.Context, p *callstore.FsckParams, progressCb FsckProgressCallback) error
}

type MoveProgressCallback func(callstore.MoveProgressMsg)

func (c *restClient) MoveCalls(ctx context.Context, p *callstore.MoveCallParams, progressCb MoveProgressCallback) error {
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

	resp, err := c.do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close() //nolint:errcheck

	b := resp.Body

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(b)
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

func (c *restClient) CallsGC(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err := c.POST(ctx, "/admin/callsgc", "")
	if err != nil {
		return err
	}

	resp, err := c.do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != 200 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		return errors.New(string(body))
	}

	return nil
}

type FsckProgressCallback func(callstore.FsckReport)

func (c *restClient) CallsFsck(ctx context.Context, p *callstore.FsckParams, progressCb FsckProgressCallback) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err := c.POST(ctx, "/admin/callsfsck", p)
	if err != nil {
		return err
	}

	cb := func(m callstore.FsckReport) error {
		if m.Error != nil {
			return errors.New(*m.Error)
		}

		progressCb(m)
		return nil
	}

	setSSErequestHeaders(req)

	resp, err := c.do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		var m callstore.FsckReport
		err = json.Unmarshal(body, &m)
		if err != nil {
			return fmt.Errorf("decoding '%s': %w", string(body), err)
		}

		if m.Error == nil {
			return fmt.Errorf("unknown error: %s", string(body))
		}

		return cb(m)
	}

	ch, err := sseSubscribe[callstore.FsckReport](resp.Body)
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
