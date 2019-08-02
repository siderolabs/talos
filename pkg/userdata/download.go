/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/url"
	"time"

	yaml "gopkg.in/yaml.v2"
)

const b64 = "base64"

type downloadOptions struct {
	Headers map[string]string
	Format  string
	Retries int
	Wait    float64
}

// Option configures the download options
type Option func(*downloadOptions)

func downloadDefaults() *downloadOptions {
	return &downloadOptions{
		Headers: make(map[string]string),
		Retries: 10,
		Wait:    float64(64),
	}
}

// WithFormat specifies the source format. This ultimately will be a yaml
// but may be represented in different formats. For example, the userdata
// may be base64 encoded
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
// the userdata
func WithHeaders(headers map[string]string) Option {
	return func(d *downloadOptions) {
		d.Headers = headers
	}
}

// WithRetries specifies how many times download is retried before failing
func WithRetries(retries int) Option {
	return func(d *downloadOptions) {
		d.Retries = retries
	}
}

// WithMaxWait specifies the maximum amount of time to wait between download
// attempts
func WithMaxWait(wait float64) Option {
	return func(d *downloadOptions) {
		d.Wait = wait
	}
}

// Download initializes a UserData struct from a remote URL.
// nolint: gocyclo
func Download(udURL string, opts ...Option) (data *UserData, err error) {
	u, err := url.Parse(udURL)
	if err != nil {
		return data, err
	}

	dlOpts := downloadDefaults()
	for _, opt := range opts {
		opt(dlOpts)
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return data, err
	}

	for k, v := range dlOpts.Headers {
		req.Header.Set(k, v)
	}

	var dataBytes []byte
	for attempt := 0; attempt < dlOpts.Retries; attempt++ {
		dataBytes, err = download(req)
		if err != nil {
			log.Printf("download failed: %+v", err)
			backoff(float64(attempt), dlOpts.Wait)
			continue
		}

		// Only need to do something 'extra' if base64
		// nolint: gocritic
		switch dlOpts.Format {
		case b64:
			var baseBytes []byte
			baseBytes, err = base64.StdEncoding.DecodeString(string(dataBytes))
			if err != nil {
				return data, err
			}
			dataBytes = baseBytes
		}

		data = &UserData{}
		if err := yaml.Unmarshal(dataBytes, data); err != nil {
			return data, fmt.Errorf("unmarshal user data: %s", err.Error())
		}
		return data, data.Validate()
	}

	return data, fmt.Errorf("failed to download userdata from: %s", u.String())
}

// download handles the actual http request
func download(req *http.Request) (data []byte, err error) {
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return data, err
	}
	// nolint: errcheck
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return data, fmt.Errorf("failed to download userdata, received %d", resp.StatusCode)
	}

	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return data, fmt.Errorf("read user data: %s", err.Error())
	}

	return data, err
}

// backoff is a simple exponential sleep/backoff
func backoff(attempt float64, wait float64) {
	snooze := math.Pow(2, attempt)
	if snooze > wait {
		snooze = wait
	}
	log.Printf("download attempt %g failed, retrying in %g seconds", attempt, snooze)
	time.Sleep(time.Duration(snooze) * time.Second)
}
