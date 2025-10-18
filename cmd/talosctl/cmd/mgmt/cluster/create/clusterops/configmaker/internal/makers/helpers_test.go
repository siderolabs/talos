// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops/configmaker/internal/makers"
)

func TestParseOmniAPIUrl(t *testing.T) {
	t.Run("valid grpc", func(t *testing.T) {
		u, err := makers.ParseOmniAPIUrl("grpc://10.5.0.1:8090?jointoken=abc")
		assert.NoError(t, err)

		if assert.NotNil(t, u) {
			assert.Equal(t, "grpc", u.Scheme)
			assert.Equal(t, "10.5.0.1:8090", u.Host)
			assert.Equal(t, "abc", u.Query().Get("jointoken"))
		}
	})

	t.Run("valid https", func(t *testing.T) {
		u, err := makers.ParseOmniAPIUrl("https://example.com:443?jointoken=token123")
		assert.NoError(t, err)

		if assert.NotNil(t, u) {
			assert.Equal(t, "https", u.Scheme)
			assert.Equal(t, "example.com:443", u.Host)
			assert.Equal(t, "token123", u.Query().Get("jointoken"))
		}
	})

	t.Run("invalid scheme", func(t *testing.T) {
		u, err := makers.ParseOmniAPIUrl("http://10.5.0.1:8090?jointoken=abc")
		assert.Error(t, err)
		assert.Nil(t, u)
	})

	t.Run("missing jointoken", func(t *testing.T) {
		u, err := makers.ParseOmniAPIUrl("grpc://10.5.0.1:8090")
		assert.Error(t, err)
		assert.Nil(t, u)
	})

	t.Run("missing port", func(t *testing.T) {
		u, err := makers.ParseOmniAPIUrl("grpc://10.5.0.1?jointoken=abc")
		assert.Error(t, err)
		assert.Nil(t, u)
	})
}
