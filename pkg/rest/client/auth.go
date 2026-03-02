package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

type JWT struct {
	JWT string `json:"jwt"`
}

func (c *client) Login(ctx context.Context, username, password string) (*JWT, error) {
	form := url.Values{}
	form.Add("username", username)
	form.Add("password", password)

	req, err := c.POST(ctx, "/api/login", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if c.hc.Jar == nil {
		jar, err := cookiejar.New(nil)
		if err != nil {
			return nil, err
		}
		c.hc.Jar = jar
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}

	c.debugTee(resp)

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("login response %s: %s", resp.Status, string(msg))
	}

	jwt := new(JWT)

	err = json.NewDecoder(resp.Body).Decode(jwt)
	if err != nil {
		return nil, err
	}

	return jwt, nil
}
