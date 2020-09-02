// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package download

import (
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

// Download downloads a config.
// nolint: gocyclo
func Download(endpoint string, opts ...Option) (b []byte, err error) {
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

	if req, err = http.NewRequest(http.MethodGet, u.String(), nil); err != nil {
		return b, err
	}

	for k, v := range dlOpts.Headers {
		req.Header.Set(k, v)
	}

	err = retry.Exponential(60*time.Second, retry.WithUnits(time.Second), retry.WithJitter(time.Second)).Retry(func() error {
		b, err = download(req)
		if err != nil {
			return retry.ExpectedError(err)
		}

		return nil
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

func download(req *http.Request) (data []byte, err error) {
	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return data, err
	}
	// nolint: errcheck
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return data, fmt.Errorf("failed to download config, received %d", resp.StatusCode)
	}

	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return data, fmt.Errorf("read config: %s", err.Error())
	}

	return data, err
}

func init() {
	transport := (http.DefaultTransport.(*http.Transport))
	transport.RegisterProtocol("tftp", NewTFTPTransport())
}
