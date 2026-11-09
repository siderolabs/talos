// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package endpoint_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/endpoint"
)

func TestParseEndpoint(t *testing.T) {
	t.Run("parses a join token from a complete URL without error", func(t *testing.T) {
		// when
		endpoint, err := endpoint.Parse("grpc://10.5.0.2:3445?jointoken=ttt")

		// then
		assert.NoError(t, err)
		assert.Equal(t, "10.5.0.2:3445", endpoint.Host)
		assert.True(t, endpoint.Insecure)
		assert.Equal(t, "ttt", endpoint.GetParam("jointoken"))
	})

	t.Run("parses a join token from a secure URL without error", func(t *testing.T) {
		// when
		endpoint, err := endpoint.Parse("https://10.5.0.2:3445?jointoken=ttt&jointoken=xxx")

		// then
		assert.NoError(t, err)
		assert.Equal(t, "10.5.0.2:3445", endpoint.Host)
		assert.False(t, endpoint.Insecure)
		assert.Equal(t, "ttt", endpoint.GetParam("jointoken"))
	})

	t.Run("parses a join token from a secure URL without port", func(t *testing.T) {
		// when
		endpoint, err := endpoint.Parse("https://10.5.0.2?jointoken=ttt&jointoken=xxx")

		// then
		assert.NoError(t, err)
		assert.Equal(t, "10.5.0.2:443", endpoint.Host)
		assert.False(t, endpoint.Insecure)
		assert.Equal(t, "ttt", endpoint.GetParam("jointoken"))
	})

	t.Run("parses a join token from an URL without a scheme", func(t *testing.T) {
		// when
		endpoint, err := endpoint.Parse("10.5.0.2:3445?jointoken=ttt")

		// then
		assert.NoError(t, err)
		assert.Equal(t, "10.5.0.2:3445", endpoint.Host)
		assert.True(t, endpoint.Insecure)
		assert.Equal(t, "ttt", endpoint.GetParam("jointoken"))
	})

	t.Run("does not error if there is no join token in a complete URL", func(t *testing.T) {
		// when
		endpoint, err := endpoint.Parse("grpc://10.5.0.2:3445")

		// then
		assert.NoError(t, err)
		assert.Equal(t, "10.5.0.2:3445", endpoint.Host)
		assert.True(t, endpoint.Insecure)
		assert.Equal(t, "", endpoint.GetParam("jointoken"))
	})

	t.Run("does not error if there is no join token in an URL without a scheme", func(t *testing.T) {
		// when
		endpoint, err := endpoint.Parse("10.5.0.2:3445")

		// then
		assert.NoError(t, err)
		assert.Equal(t, "10.5.0.2:3445", endpoint.Host)
		assert.True(t, endpoint.Insecure)
		assert.Equal(t, "", endpoint.GetParam("jointoken"))
	})
}

func TestParseEndpointErrorRedacted(t *testing.T) {
	t.Parallel()

	_, err := endpoint.Parse("https://10.5.0.2:3445/\x7f?jointoken=secret")
	require.Error(t, err)

	assert.NotContains(t, err.Error(), "secret")
	assert.Contains(t, err.Error(), "jointoken=redacted")
}

func TestRedact(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no query",
			input:    "https://10.5.0.2:3445",
			expected: "https://10.5.0.2:3445",
		},
		{
			name:     "no scheme",
			input:    "10.5.0.2:3445?jointoken=secret",
			expected: "10.5.0.2:3445?jointoken=redacted",
		},
		{
			name:     "omni",
			input:    "https://instance.siderolink.omni.example.com?jointoken=secret",
			expected: "https://instance.siderolink.omni.example.com?jointoken=redacted",
		},
		{
			name:     "multiple values",
			input:    "https://10.5.0.2:3445?jointoken=secret&jointoken=secret2&grpc_tunnel=true",
			expected: "https://10.5.0.2:3445?grpc_tunnel=true&jointoken=redacted",
		},
		{
			name:     "no token",
			input:    "grpc://10.5.0.2:3445?grpc_tunnel=true",
			expected: "grpc://10.5.0.2:3445?grpc_tunnel=true",
		},
		{
			name:     "invalid query",
			input:    "https://10.5.0.2:3445?jointoken=secret%zz",
			expected: "https://10.5.0.2:3445?redacted",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, endpoint.Redact(test.input))
		})
	}
}
