// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package download

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
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
	Endpoint   string

	ErrorOnNotFound      error
	ErrorOnEmptyResponse error
}

// Option configures the download options.
type Option func(*downloadOptions)

func downloadDefaults() *downloadOptions {
	return &downloadOptions{
		Headers: make(map[string]string),
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
		d.Headers = headers
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

// WithEndpointFunc provides a function that sets the endpoint of the download options.
func WithEndpointFunc(endpointFunc func() string) Option {
	return func(d *downloadOptions) {
		d.Endpoint = endpointFunc()
	}
}

// Download downloads a config.
//
//nolint:gocyclo
func Download(ctx context.Context, endpoint string, opts ...Option) (b []byte, err error) {
	dlOpts := downloadDefaults()

	dlOpts.Endpoint = endpoint

	err = retry.Exponential(
		180*time.Second,
		retry.WithUnits(time.Second),
		retry.WithJitter(time.Second),
		retry.WithErrorLogging(true),
	).RetryWithContext(ctx, func(ctx context.Context) error {
		dlOpts = downloadDefaults()

		dlOpts.Endpoint = endpoint

		for _, opt := range opts {
			opt(dlOpts)
		}

		var u *url.URL
		u, err = url.Parse(dlOpts.Endpoint)
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

		for k, v := range dlOpts.Headers {
			req.Header.Set(k, v)
		}

		b, err = download(req, dlOpts)

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download config from %q: %w", dlOpts.Endpoint, err)
	}

	if dlOpts.Format == b64 {
		var b64 []byte

		b64, err = base64.StdEncoding.DecodeString(string(b))
		if err != nil {
			return nil, err
		}

		b = b64
	}

	return b, nil
}

func download(req *http.Request, dlOpts *downloadOptions) (data []byte, err error) {
	transport := httpdefaults.PatchTransport(cleanhttp.DefaultTransport())
	transport.RegisterProtocol("tftp", NewTFTPTransport())

	if dlOpts.LowSrcPort {
		port := 100 + rand.Intn(512)

		localTCPAddr, tcperr := net.ResolveTCPAddr("tcp", ":"+strconv.Itoa(port))
		if tcperr != nil {
			return nil, retry.ExpectedError(fmt.Errorf("resolving source tcp address: %s", tcperr.Error()))
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

	if resp.StatusCode == http.StatusNotFound && dlOpts.ErrorOnNotFound != nil {
		return data, dlOpts.ErrorOnNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return data, retry.ExpectedError(fmt.Errorf("failed to download config, received %d", resp.StatusCode))
	}

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return data, retry.ExpectedError(fmt.Errorf("read config: %s", err.Error()))
	}

	if len(data) == 0 && dlOpts.ErrorOnEmptyResponse != nil {
		return data, dlOpts.ErrorOnEmptyResponse
	}

	return data, nil
}
