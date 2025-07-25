package client

import (
	"context"
	"errors"
	"net/http"

	"github.com/tmaxmax/go-sse"
)

type EventCallback func(msg []byte)

func (c *client) sseSubscribe(req *http.Request, cb EventCallback) error {
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")

	sseClient := *sse.DefaultClient
	sseClient.HTTPClient = &c.hc

	conn := sseClient.NewConnection(req)
	_ = conn.SubscribeToAll(func(ev sse.Event) {
		cb([]byte(ev.Data))
	})

	if err := conn.Connect(); !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}
