package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"

	"dynatron.me/x/stillbox/internal/version"
)

var (
	APIClientUA = version.HttpString("go-api-client")
)

type Client interface {
	adminClient
}

type client struct {
	unixSocket *string
	baseURL    *url.URL
	hc         http.Client
}

type ClientOption func(*client)

func UnixSocket(p string) ClientOption {
	return func(c *client) {
		c.unixSocket = &p
		baseURL, err := url.Parse("http://unix")
		if err != nil {
			panic(err)
		}

		c.baseURL = baseURL
	}
}

func BaseURL(u *url.URL) ClientOption {
	return func(c *client) {
		c.baseURL = u
	}
}

func New(options ...ClientOption) (c *client, err error) {
	c = &client{
		hc: *http.DefaultClient,
	}

	for _, opt := range options {
		opt(c)
	}

	if c.unixSocket != nil {
		c.hc = controlSocketClient(*c.unixSocket)
	}

	if c.baseURL == nil {
		return nil, errors.New("no base url set")
	}

	return c, nil
}

func (c *client) url(path string) (*url.URL, error) {
	u, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	return c.baseURL.ResolveReference(u), nil
}

func (c *client) POST(ctx context.Context, endpoint string, body any) (*http.Request, error) {
	return c.newRequest(ctx, http.MethodPost, endpoint, body)
}

func (c *client) newRequest(ctx context.Context, method, endpoint string, body any) (*http.Request, error) {
	u, err := c.url(endpoint)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader

	switch br := body.(type) {
	case io.Reader:
		bodyReader = br
	default:
		if body == nil {
			bodyReader = http.NoBody
		} else {
			pay, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}

			bodyReader = bytes.NewReader(pay)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", APIClientUA)

	return req, nil
}

func controlSocketClient(socketPath string) http.Client {
	c := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				d := net.Dialer{}
				return d.DialContext(ctx, "unix", socketPath)
			},
		},
	}

	return c
}
