// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package endpoint_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
