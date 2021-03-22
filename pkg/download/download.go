// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package download

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/talos-systems/go-retry/retry"
)

const b64 = "base64"

type downloadOptions struct {
	Headers map[string]string
	Format  string

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

// Download downloads a config.
func Download(ctx context.Context, endpoint string, opts ...Option) (b []byte, err error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return b, err
	}

	if u.Scheme == "file" {
		return ioutil.ReadFile(u.Path)
	}

	dlOpts := downloadDefaults()

	for _, opt := range opts {
		opt(dlOpts)
	}

	var req *http.Request

	if req, err = http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil); err != nil {
		return b, err
	}

	for k, v := range dlOpts.Headers {
		req.Header.Set(k, v)
	}

	err = retry.Exponential(180*time.Second, retry.WithUnits(time.Second), retry.WithJitter(time.Second), retry.WithErrorLogging(true)).Retry(func() error {
		select {
		case <-ctx.Done():
			return retry.UnexpectedError(context.Canceled)
		default:
		}

		b, err = download(req, dlOpts)

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download config from %q: %w", u.String(), err)
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
	client := &http.Client{}

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

	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return data, retry.ExpectedError(fmt.Errorf("read config: %s", err.Error()))
	}

	if len(data) == 0 && dlOpts.ErrorOnEmptyResponse != nil {
		return data, dlOpts.ErrorOnEmptyResponse
	}

	return data, nil
}

func init() {
	transport := (http.DefaultTransport.(*http.Transport))
	transport.RegisterProtocol("tftp", NewTFTPTransport())
}
