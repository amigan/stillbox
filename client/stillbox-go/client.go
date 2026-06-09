package stillbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"dynatron.me/x/stillbox/internal/version"
	"dynatron.me/x/stillbox/pkg/authn"
)

var (
	APIClientUA = version.HttpString("go-api-client")
)

type RestClient interface {
	BaseURL() *url.URL
	HTTPClient() *http.Client
	adminClient
	Login(ctx context.Context, username, password string) (*JWT, error)
}

type restClient struct {
	unixSocket *string
	baseURL    *url.URL
	hc         http.Client
	headers    http.Header
	debug      io.Writer
}

func (c *restClient) HTTPClient() *http.Client {
	return &c.hc
}

func (c *restClient) BaseURL() *url.URL {
	return c.baseURL
}

type ClientOption func(*restClient)

func UnixSocket(p string) ClientOption {
	return func(c *restClient) {
		c.unixSocket = &p
		baseURL, err := url.Parse("http://unix")
		if err != nil {
			panic(err)
		}

		c.baseURL = baseURL
	}
}

func BaseURL(u *url.URL) ClientOption {
	return func(c *restClient) {
		c.baseURL = u
	}
}

func Debug(w io.Writer) ClientOption {
	return func(c *restClient) {
		c.debug = w
	}
}

func WithCookieJar(jar http.CookieJar) ClientOption {
	return func(c *restClient) {
		c.hc.Jar = jar
	}
}

func WithAuthBearer(token string) ClientOption {
	return func(c *restClient) {
		cj, err := cookiejar.New(nil)
		if err != nil {
			panic(err)
		}

		cj.SetCookies(c.baseURL, []*http.Cookie{
			{
				Name:  authn.CookieName,
				Value: token,
			},
		})
		c.hc.Jar = cj
	}
}

type debugCloser struct {
	io.Reader
	cl io.ReadCloser
}

func (d *debugCloser) Close() error {
	return d.cl.Close()
}

func (c *restClient) debugTee(resp *http.Response) {
	if c.debug != nil {
		cl := &debugCloser{
			cl: resp.Body,
		}
		cl.Reader = io.TeeReader(resp.Body, c.debug)
		resp.Body = cl
	}
}

// do fills in headers and executes the request.
func (c *restClient) do(req *http.Request) (*http.Response, error) {
	if c.headers != nil {
		for h, v := range c.headers {
			for _, ve := range v {
				req.Header.Add(h, ve)
			}
		}
	}

	return c.hc.Do(req)
}

func NewRESTClient(options ...ClientOption) (c *restClient, err error) {
	c = &restClient{
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

func (c *restClient) url(path string) (*url.URL, error) {
	u, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	return c.baseURL.ResolveReference(u), nil
}

func (c *restClient) POST(ctx context.Context, endpoint string, body any) (*http.Request, error) {
	return c.newRequest(ctx, http.MethodPost, endpoint, body)
}

func (c *restClient) newRequest(ctx context.Context, method, endpoint string, body any) (*http.Request, error) {
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
