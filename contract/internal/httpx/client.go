// Package httpx is a small net/http client for the contract suite.
package httpx

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Client does not follow redirects. A 302 is a response to record, not a hop.
// Jar, when non-nil, stores Set-Cookie values and sends them back. A nil Jar
// leaves cookies untouched, which is what the transport tests want.
type Client struct {
	HTTP *http.Client
	Jar  map[string]string
}

// New returns a client with a bounded timeout.
func New() *Client {
	return &Client{HTTP: &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
}

// Exchange is one HTTP round trip.
type Exchange struct {
	Status int
	Header http.Header
	Body   []byte
}

// Do sends one request. A "Host" header overrides the URL host, which is how
// the suite checks the MCP host allowlist.
func (c *Client) Do(ctx context.Context, method, url string, headers map[string]string, body []byte) (*Exchange, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, r)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		if k == "Host" {
			req.Host = v
			continue
		}
		req.Header.Set(k, v)
	}
	if c.Jar != nil && req.Header.Get("Cookie") == "" {
		if cookie := c.cookieHeader(); cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	buf, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	c.absorbCookies(res.Header)
	return &Exchange{Status: res.StatusCode, Header: res.Header.Clone(), Body: buf}, nil
}

func (c *Client) absorbCookies(h http.Header) {
	if c.Jar == nil {
		return
	}
	for _, line := range h.Values("Set-Cookie") {
		pair, _, _ := strings.Cut(line, ";")
		name, value, ok := strings.Cut(pair, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			continue
		}
		c.Jar[name] = strings.TrimSpace(value)
	}
}

func (c *Client) cookieHeader() string {
	if len(c.Jar) == 0 {
		return ""
	}
	names := make([]string, 0, len(c.Jar))
	for name := range c.Jar {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, len(names))
	for i, name := range names {
		parts[i] = name + "=" + c.Jar[name]
	}
	return strings.Join(parts, "; ")
}
