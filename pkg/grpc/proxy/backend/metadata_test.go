// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package backend_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/grpc/proxy/backend"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

func TestOutgoingMetadata(t *testing.T) {
	t.Parallel()

	// metadata a hostile caller might send: authorization, routing, authentication and
	// the proxy's own loop marker, plus arbitrary keys
	callerMD := metadata.New(map[string]string{
		"talos-role": "os:admin",
		"token":      "some-secret",
		"node":       "10.0.0.1",
		"nodes":      "10.0.0.2",
		"proxyfrom":  "10.0.0.3",
		"peer":       "10.0.0.4",
		"whatever":   "value",
		":authority": "10.0.0.5",
		// allowlisted, purely informational
		"runtime": "Talos",
		"context": "cluster-1",
	})

	for _, test := range []struct {
		name          string
		roles         role.Set
		expectedRoles []string
	}{
		{
			name:          "roles resolved",
			roles:         role.MakeSet(role.Reader),
			expectedRoles: []string{"os:reader"},
		},
		{
			// the GHSA-rjwj-368c-f82r case: a credential naming no roles (e.g. a
			// certificate with an empty Subject Organization) must not keep the
			// 'talos-role' it sent, or the backend authorizes it as os:admin
			name:          "no roles resolved",
			roles:         role.Zero,
			expectedRoles: nil,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := metadata.NewIncomingContext(authz.ContextWithRoles(t.Context(), test.roles), callerMD)

			md := backend.OutgoingMetadata(ctx)

			assert.Equal(t, test.expectedRoles, md.Get("talos-role"))

			// forwarded: informational only
			assert.Equal(t, []string{"Talos"}, md.Get("runtime"))
			assert.Equal(t, []string{"cluster-1"}, md.Get("context"))

			// dropped: owned by the proxy, or meaningless to the backend
			for _, key := range []string{"token", "node", "nodes", "proxyfrom", "peer", "whatever", ":authority"} {
				assert.Empty(t, md.Get(key), "%q must not be proxied", key)
			}

			// the incoming metadata is never mutated
			assert.Equal(t, []string{"os:admin"}, callerMD.Get("talos-role"))
		})
	}
}
