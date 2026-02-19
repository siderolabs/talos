// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package download provides a download with retries for machine configuration and userdata.
package download

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"maps"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/siderolabs/go-retry/retry"

	"github.com/siderolabs/talos/pkg/httpdefaults"
)

const b64 = "base64"

type downloadOptions struct {
	Headers    map[string]string
	Format     string
	LowSrcPort bool

	EndpointFunc func(context.Context) (string, error)

	ErrorOnNotFound      error
	ErrorOnBadRequest    error
	ErrorOnEmptyResponse error

	Timeout      time.Duration
	RetryOptions []retry.Option
}

// Option configures the download options.
type Option func(*downloadOptions)

func downloadDefaults(endpoint string) *downloadOptions {
	return &downloadOptions{
		EndpointFunc: func(context.Context) (string, error) {
			return endpoint, nil
		},
		Headers: make(map[string]string),
		Timeout: 3 * time.Minute,
	}
}

// WithFormat specifies the source format. This ultimately will be a yaml
// but may be represented in different formats. For example, the config
// may be base64 encoded.
func WithFormat(format string) Option {
	return func(d *downloadOptions) {
		switch format {
		case b64:
			d.Format = b64
		default:
			d.Format = "yaml"
		}
	}
}

// WithHeaders specifies any http headers that are needed for downloading
// the config.
func WithHeaders(headers map[string]string) Option {
	return func(d *downloadOptions) {
		if headers == nil {
			return
		}

		if d.Headers == nil {
			d.Headers = map[string]string{}
		}

		maps.Copy(d.Headers, headers)
	}
}

// WithLowSrcPort sets low source port to download
// the config.
func WithLowSrcPort() Option {
	return func(d *downloadOptions) {
		d.LowSrcPort = true
	}
}

// WithErrorOnNotFound provides specific error to return when response has HTTP 404 error.
func WithErrorOnNotFound(e error) Option {
	return func(d *downloadOptions) {
		d.ErrorOnNotFound = e
	}
}

// WithErrorOnEmptyResponse provides specific error to return when response is empty.
func WithErrorOnEmptyResponse(e error) Option {
	return func(d *downloadOptions) {
		d.ErrorOnEmptyResponse = e
	}
}

// WithErrorOnBadRequest provides specific error to return when response has HTTP 400 error.
func WithErrorOnBadRequest(e error) Option {
	return func(d *downloadOptions) {
		d.ErrorOnBadRequest = e
	}
}

// WithEndpointFunc provides a function that sets the endpoint of the download options.
func WithEndpointFunc(endpointFunc func(context.Context) (string, error)) Option {
	return func(d *downloadOptions) {
		d.EndpointFunc = endpointFunc
	}
}

// WithTimeout sets the timeout for the download.
func WithTimeout(timeout time.Duration) Option {
	return func(d *downloadOptions) {
		d.Timeout = timeout
	}
}

// WithRetryOptions sets the retry options for the download.
func WithRetryOptions(opts ...retry.Option) Option {
	return func(d *downloadOptions) {
		d.RetryOptions = append(d.RetryOptions, opts...)
	}
}

// Download downloads a config.
//
//nolint:gocyclo
func Download(ctx context.Context, endpoint string, opts ...Option) (b []byte, err error) {
	options := downloadDefaults(endpoint)

	for _, opt := range opts {
		opt(options)
	}

	err = retry.Exponential(
		options.Timeout,
		append([]retry.Option{
			retry.WithUnits(time.Second),
			retry.WithJitter(time.Second),
			retry.WithErrorLogging(true),
		},
			options.RetryOptions...,
		)...,
	).RetryWithContext(ctx, func(ctx context.Context) error {
		var attemptEndpoint string

		attemptEndpoint, err = options.EndpointFunc(ctx)
		if err != nil {
			return err
		}

		if err = func() error {
			var u *url.URL

			u, err = url.Parse(attemptEndpoint)
			if err != nil {
				return err
			}

			if u.Scheme == "file" {
				var fileContent []byte

				fileContent, err = os.ReadFile(u.Path)
				if err != nil {
					return err
				}

				b = fileContent

				return nil
			}

			var req *http.Request

			if req, err = http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil); err != nil {
				return err
			}

			for k, v := range options.Headers {
				req.Header.Set(k, v)
			}

			b, err = download(req, options)

			return err
		}(); err != nil {
			return fmt.Errorf("failed to download config from %q: %w", endpoint, err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if options.Format == b64 {
		var b64 []byte

		b64, err = base64.StdEncoding.DecodeString(string(b))
		if err != nil {
			return nil, err
		}

		b = b64
	}

	return b, nil
}

//nolint:gocyclo
func download(req *http.Request, options *downloadOptions) (data []byte, err error) {
	transport := httpdefaults.PatchTransport(cleanhttp.DefaultTransport())
	transport.RegisterProtocol("tftp", NewTFTPTransport())

	if options.LowSrcPort {
		port := 100 + rand.IntN(512)

		localTCPAddr, tcperr := net.ResolveTCPAddr("tcp", ":"+strconv.Itoa(port))
		if tcperr != nil {
			return nil, retry.ExpectedErrorf("resolving source tcp address: %s", tcperr.Error())
		}

		d := (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
			LocalAddr: localTCPAddr,
		}).DialContext

		transport.DialContext = d
	}

	client := &http.Client{
		Transport: transport,
	}

	resp, err := client.Do(req)
	if err != nil {
		return data, retry.ExpectedError(err)
	}
	//nolint:errcheck
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound && options.ErrorOnNotFound != nil {
		return data, options.ErrorOnNotFound
	}

	if resp.StatusCode == http.StatusBadRequest && options.ErrorOnBadRequest != nil {
		return data, options.ErrorOnBadRequest
	}

	// 204 - StatusNoContent is also a successful response, signaling  that there is no body
	if resp.StatusCode == http.StatusNoContent {
		return data, options.ErrorOnEmptyResponse
	}

	if resp.StatusCode != http.StatusOK {
		// try to read first 32 bytes of the response body
		// to provide more context in case of error
		data, _ = io.ReadAll(io.LimitReader(resp.Body, 32)) //nolint:errcheck // as error already happened, we don't care much about this one
		data = bytes.ToValidUTF8(data, nil)

		return data, retry.ExpectedErrorf("failed to download config, status code %d, body %q", resp.StatusCode, string(data))
	}

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return data, retry.ExpectedErrorf("read config: %s", err.Error())
	}

	if len(data) == 0 && options.ErrorOnEmptyResponse != nil {
		return data, options.ErrorOnEmptyResponse
	}

	return data, nil
}
