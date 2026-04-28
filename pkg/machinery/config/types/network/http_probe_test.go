// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	_ "embed"
	"net/url"
	"testing"
	"time"

	"github.com/siderolabs/gen/ensure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
)

//go:embed testdata/httpprobeconfig.yaml
var expectedHTTPProbeConfigDocument []byte

const exampleHTTPURL = "https://example.com"

func TestHTTPProbeConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewHTTPProbeConfigV1Alpha1("http-check")
	cfg.CommonProbeConfig = network.CommonProbeConfig{
		ProbeInterval:         time.Second,
		ProbeFailureThreshold: 3,
	}
	cfg.HTTPEndpoint = meta.URL{URL: ensure.Value(url.Parse(exampleHTTPURL))}
	cfg.HTTPTimeout = 10 * time.Second

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedHTTPProbeConfigDocument, marshaled)
}

//nolint:dupl
func TestHTTPProbeConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedHTTPProbeConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.HTTPProbeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.HTTPProbeKind,
		},
		MetaName: "http-check",
		CommonProbeConfig: network.CommonProbeConfig{
			ProbeInterval:         time.Second,
			ProbeFailureThreshold: 3,
		},
		HTTPEndpoint: meta.URL{URL: ensure.Value(url.Parse(exampleHTTPURL))},
		HTTPTimeout:  10 * time.Second,
	}, docs[0])
}

func TestHTTPProbeConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.HTTPProbeConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "valid config",
			cfg: func() *network.HTTPProbeConfigV1Alpha1 {
				c := network.NewHTTPProbeConfigV1Alpha1("test-probe")
				c.CommonProbeConfig = network.CommonProbeConfig{
					ProbeInterval:         time.Second,
					ProbeFailureThreshold: 3,
				}
				c.HTTPEndpoint = meta.URL{URL: ensure.Value(url.Parse(exampleHTTPURL))}

				return c
			},
		},
		{
			name: "valid http url",
			cfg: func() *network.HTTPProbeConfigV1Alpha1 {
				c := network.NewHTTPProbeConfigV1Alpha1("test-probe")
				c.HTTPEndpoint = meta.URL{URL: ensure.Value(url.Parse("http://proxy.example.com:3128/health"))}

				return c
			},
		},
		{
			name: "missing name",
			cfg: func() *network.HTTPProbeConfigV1Alpha1 {
				c := network.NewHTTPProbeConfigV1Alpha1("")
				c.HTTPEndpoint = meta.URL{URL: ensure.Value(url.Parse(exampleHTTPURL))}

				return c
			},
			expectedError: "probe name is required",
		},
		{
			name: "missing URL",
			cfg: func() *network.HTTPProbeConfigV1Alpha1 {
				c := network.NewHTTPProbeConfigV1Alpha1("probe44")

				return c
			},
			expectedError: "HTTP probe URL is required",
		},
		{
			name: "invalid scheme",
			cfg: func() *network.HTTPProbeConfigV1Alpha1 {
				c := network.NewHTTPProbeConfigV1Alpha1("probe-scheme")
				c.HTTPEndpoint = meta.URL{URL: ensure.Value(url.Parse("ftp://example.com"))}

				return c
			},
			expectedError: `HTTP probe URL scheme must be http or https, got "ftp"`,
		},
		{
			name: "negative timeout",
			cfg: func() *network.HTTPProbeConfigV1Alpha1 {
				c := network.NewHTTPProbeConfigV1Alpha1("probe33")
				c.HTTPEndpoint = meta.URL{URL: ensure.Value(url.Parse(exampleHTTPURL))}
				c.HTTPTimeout = -5 * time.Second

				return c
			},
			expectedError: "HTTP probe timeout cannot be negative: -5s",
		},
		{
			name: "negative values",
			cfg: func() *network.HTTPProbeConfigV1Alpha1 {
				c := network.NewHTTPProbeConfigV1Alpha1("probe33")
				c.CommonProbeConfig.ProbeFailureThreshold = -1
				c.CommonProbeConfig.ProbeInterval = -time.Second
				c.HTTPTimeout = -5 * time.Second
				c.HTTPEndpoint = meta.URL{URL: ensure.Value(url.Parse(exampleHTTPURL))}

				return c
			},
			expectedError: "HTTP probe timeout cannot be negative: -5s\nprobe interval cannot be negative: -1s\nprobe failure threshold cannot be negative: -1",
		},
		{
			name: "empty",
			cfg: func() *network.HTTPProbeConfigV1Alpha1 {
				return network.NewHTTPProbeConfigV1Alpha1("")
			},
			expectedError: "probe name is required\nHTTP probe URL is required",
		},
		{
			name: "url without host",
			cfg: func() *network.HTTPProbeConfigV1Alpha1 {
				c := network.NewHTTPProbeConfigV1Alpha1("no-host")
				c.HTTPEndpoint = meta.URL{URL: ensure.Value(url.Parse("https://"))}

				return c
			},
			expectedError: "HTTP probe URL must be an absolute http or https URL with a non-empty host",
		},
		{
			name: "opaque url",
			cfg: func() *network.HTTPProbeConfigV1Alpha1 {
				c := network.NewHTTPProbeConfigV1Alpha1("opaque")
				c.HTTPEndpoint = meta.URL{URL: ensure.Value(url.Parse("http:opaque-url"))}

				return c
			},
			expectedError: "HTTP probe URL must be an absolute http or https URL with a non-empty host",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})

			assert.Equal(t, test.expectedWarnings, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHTTPProbeConfigMethods(t *testing.T) {
	t.Parallel()

	t.Run("URL", func(t *testing.T) {
		t.Parallel()

		cfg := network.NewHTTPProbeConfigV1Alpha1("test")
		cfg.HTTPEndpoint = meta.URL{URL: ensure.Value(url.Parse(exampleHTTPURL))}

		assert.Equal(t, meta.URL{URL: ensure.Value(url.Parse(exampleHTTPURL))}, cfg.URL())
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		probeName := "my-probe"
		cfg := network.NewHTTPProbeConfigV1Alpha1(probeName)

		assert.Equal(t, probeName, cfg.Name())
	})

	t.Run("Timeout with default", func(t *testing.T) {
		t.Parallel()

		cfg := network.NewHTTPProbeConfigV1Alpha1("test")
		cfg.HTTPTimeout = 0

		assert.Equal(t, 10*time.Second, cfg.Timeout())
	})

	t.Run("Timeout with custom value", func(t *testing.T) {
		t.Parallel()

		cfg := network.NewHTTPProbeConfigV1Alpha1("test")
		cfg.HTTPTimeout = 5 * time.Second

		assert.Equal(t, 5*time.Second, cfg.Timeout())
	})

	t.Run("Clone", func(t *testing.T) {
		t.Parallel()

		cfg := network.NewHTTPProbeConfigV1Alpha1("clone-test")
		cfg.CommonProbeConfig = network.CommonProbeConfig{
			ProbeInterval:         500 * time.Millisecond,
			ProbeFailureThreshold: 5,
		}
		cfg.HTTPEndpoint = meta.URL{URL: ensure.Value(url.Parse(exampleHTTPURL))}
		cfg.HTTPTimeout = 15 * time.Second

		cloned := cfg.Clone()

		assert.Equal(t, cfg, cloned)
		assert.NotSame(t, cfg, cloned)
	})
}
