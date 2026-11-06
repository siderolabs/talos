// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package hcloud implements the small subset of the Hetzner Cloud API used to manage Virtual IPs.
//
// See https://docs.hetzner.cloud/ for the API reference.
//
// Requests are retried the same way the official client (hcloud-go) does: on conflicts,
// rate limiting, gateway errors and timeouts, with an exponential backoff.
package hcloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Endpoint is the default Hetzner Cloud API endpoint.
const Endpoint = "https://api.hetzner.cloud/v1"

// maxRetries is the number of retries for a failed request.
const maxRetries = 5

// maxResponseSize limits the size of the API response.
const maxResponseSize = 16 << 20

// perPage is the number of items requested per page.
const perPage = 50

// Client is a Hetzner Cloud API client.
type Client struct {
	httpClient *http.Client
	backoff    func(retries int) time.Duration
	endpoint   string
	token      string
	userAgent  string
}

// Option configures the client.
type Option func(*Client)

// WithEndpoint overrides the API endpoint.
func WithEndpoint(endpoint string) Option {
	return func(c *Client) {
		c.endpoint = endpoint
	}
}

// WithHTTPClient sets the HTTP client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithUserAgent sets the User-Agent header.
func WithUserAgent(userAgent string) Option {
	return func(c *Client) {
		c.userAgent = userAgent
	}
}

// WithBackoff overrides the backoff between retries.
func WithBackoff(backoff func(retries int) time.Duration) Option {
	return func(c *Client) {
		c.backoff = backoff
	}
}

// NewClient creates a new client with the API token.
func NewClient(token string, opts ...Option) *Client {
	c := &Client{
		httpClient: http.DefaultClient,
		backoff:    exponentialBackoff,
		endpoint:   Endpoint,
		token:      token,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// exponentialBackoff mirrors the hcloud-go default: 1s base, multiplier 2, capped at 1 minute, with jitter.
func exponentialBackoff(retries int) time.Duration {
	const (
		base       = 1.0
		maxBackoff = 60.0
	)

	backoff := math.Min(maxBackoff, base*math.Pow(2, float64(retries)))
	backoff = (backoff-base)*rand.Float64() + base //nolint:gosec

	return time.Duration(backoff * float64(time.Second))
}

// Error is an error returned by the API.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`

	StatusCode int `json:"-"`
}

// Error implements error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("%s (%s)", e.Message, e.Code)
}

// Error codes which are retried.
const (
	ErrorCodeConflict          = "conflict"
	ErrorCodeRateLimitExceeded = "rate_limit_exceeded"
	ErrorCodeBadGateway        = "bad_gateway"
	ErrorCodeTimeout           = "timeout"
)

func shouldRetry(err error) bool {
	if apiErr, ok := errors.AsType[*Error](err); ok {
		switch apiErr.Code {
		case ErrorCodeConflict, ErrorCodeRateLimitExceeded, ErrorCodeBadGateway, ErrorCodeTimeout:
			return true
		}

		return apiErr.Code == "" && (apiErr.StatusCode == http.StatusBadGateway || apiErr.StatusCode == http.StatusGatewayTimeout)
	}

	if netErr, ok := errors.AsType[net.Error](err); ok {
		return netErr.Timeout()
	}

	return false
}

// do performs the API request with retries, decoding the response into out.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reqBody []byte

	if body != nil {
		var err error

		if reqBody, err = json.Marshal(body); err != nil {
			return err
		}
	}

	for retries := 0; ; retries++ {
		err := c.doOnce(ctx, method, path, reqBody, out)
		if err == nil {
			return nil
		}

		if ctx.Err() != nil || retries >= maxRetries || !shouldRetry(err) {
			return err
		}

		select {
		case <-ctx.Done():
			return err
		case <-time.After(c.backoff(retries)):
		}
	}
}

func (c *Client) doOnce(ctx context.Context, method, path string, reqBody []byte, out any) error {
	req, err := c.newRequest(ctx, method, path, reqBody)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return parseError(resp.StatusCode, respBody)
	}

	if out == nil {
		return nil
	}

	if err = json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}

	return nil
}

func (c *Client) newRequest(ctx context.Context, method, path string, reqBody []byte) (*http.Request, error) {
	var bodyReader io.Reader

	if reqBody != nil {
		bodyReader = bytes.NewReader(reqBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

// parseError converts a non-successful response into an [Error].
func parseError(statusCode int, body []byte) *Error {
	var errResp struct {
		Error Error `json:"error"`
	}

	if json.Unmarshal(body, &errResp) != nil || errResp.Error.Code == "" {
		errResp.Error = Error{
			Message: fmt.Sprintf("server responded with status code %d", statusCode),
		}
	}

	errResp.Error.StatusCode = statusCode

	return &errResp.Error
}

type meta struct {
	Pagination struct {
		NextPage *int `json:"next_page"`
	} `json:"pagination"`
}

// all fetches all pages of a list endpoint.
func all[T any](ctx context.Context, c *Client, path, key string) ([]T, error) {
	var result []T

	page := 1

	for {
		var resp map[string]json.RawMessage

		query := url.Values{
			"page":     []string{strconv.Itoa(page)},
			"per_page": []string{strconv.Itoa(perPage)},
		}

		if err := c.do(ctx, http.MethodGet, path+"?"+query.Encode(), nil, &resp); err != nil {
			return nil, err
		}

		var items []T

		if err := json.Unmarshal(resp[key], &items); err != nil {
			return nil, fmt.Errorf("error decoding %s: %w", key, err)
		}

		result = append(result, items...)

		var m meta

		if raw, ok := resp["meta"]; ok {
			if err := json.Unmarshal(raw, &m); err != nil {
				return nil, fmt.Errorf("error decoding pagination: %w", err)
			}
		}

		// stop on the last page (or if the server misbehaves and doesn't advance)
		if m.Pagination.NextPage == nil || *m.Pagination.NextPage <= page {
			return result, nil
		}

		page = *m.Pagination.NextPage
	}
}

// Server is a Hetzner Cloud server.
type Server struct {
	PrivateNet []ServerPrivateNet `json:"private_net"`
	ID         int64              `json:"id"`
}

// ServerPrivateNet is a private network attachment of a server.
type ServerPrivateNet struct {
	IP       string   `json:"ip"`
	AliasIPs []string `json:"alias_ips"`
	Network  int64    `json:"network"`
}

// FloatingIP is a Hetzner Cloud floating IP.
//
// For IPv6 floating IPs, the IP is a network in CIDR notation.
type FloatingIP struct {
	Server *int64 `json:"server"`
	IP     string `json:"ip"`
	Type   string `json:"type"`
	ID     int64  `json:"id"`
}

// Network is a Hetzner Cloud private network.
type Network struct {
	IPRange string `json:"ip_range"`
	ID      int64  `json:"id"`
}

// Action is an asynchronous action started by the API.
type Action struct {
	Status  string `json:"status"`
	Command string `json:"command"`
	ID      int64  `json:"id"`
}

// AllServers lists all servers.
func (c *Client) AllServers(ctx context.Context) ([]Server, error) {
	return all[Server](ctx, c, "/servers", "servers")
}

// GetServer returns the server by ID.
func (c *Client) GetServer(ctx context.Context, id int64) (*Server, error) {
	var resp struct {
		Server Server `json:"server"`
	}

	if err := c.do(ctx, http.MethodGet, "/servers/"+strconv.FormatInt(id, 10), nil, &resp); err != nil {
		return nil, err
	}

	return &resp.Server, nil
}

// ChangeAliasIPs replaces alias IPs of the server in the network.
func (c *Client) ChangeAliasIPs(ctx context.Context, serverID, networkID int64, aliasIPs []string) (*Action, error) {
	var resp struct {
		Action Action `json:"action"`
	}

	if aliasIPs == nil {
		aliasIPs = []string{}
	}

	req := struct {
		AliasIPs []string `json:"alias_ips"`
		Network  int64    `json:"network"`
	}{
		Network:  networkID,
		AliasIPs: aliasIPs,
	}

	if err := c.do(ctx, http.MethodPost, "/servers/"+strconv.FormatInt(serverID, 10)+"/actions/change_alias_ips", req, &resp); err != nil {
		return nil, err
	}

	return &resp.Action, nil
}

// GetNetwork returns the network by ID.
func (c *Client) GetNetwork(ctx context.Context, id int64) (*Network, error) {
	var resp struct {
		Network Network `json:"network"`
	}

	if err := c.do(ctx, http.MethodGet, "/networks/"+strconv.FormatInt(id, 10), nil, &resp); err != nil {
		return nil, err
	}

	return &resp.Network, nil
}

// AllFloatingIPs lists all floating IPs.
func (c *Client) AllFloatingIPs(ctx context.Context) ([]FloatingIP, error) {
	return all[FloatingIP](ctx, c, "/floating_ips", "floating_ips")
}

// GetFloatingIP returns the floating IP by ID.
func (c *Client) GetFloatingIP(ctx context.Context, id int64) (*FloatingIP, error) {
	var resp struct {
		FloatingIP FloatingIP `json:"floating_ip"`
	}

	if err := c.do(ctx, http.MethodGet, "/floating_ips/"+strconv.FormatInt(id, 10), nil, &resp); err != nil {
		return nil, err
	}

	return &resp.FloatingIP, nil
}

// AssignFloatingIP assigns the floating IP to the server.
func (c *Client) AssignFloatingIP(ctx context.Context, floatingIPID, serverID int64) (*Action, error) {
	var resp struct {
		Action Action `json:"action"`
	}

	req := struct {
		Server int64 `json:"server"`
	}{
		Server: serverID,
	}

	if err := c.do(ctx, http.MethodPost, "/floating_ips/"+strconv.FormatInt(floatingIPID, 10)+"/actions/assign", req, &resp); err != nil {
		return nil, err
	}

	return &resp.Action, nil
}
