// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
)

func TestPromotionEndpoints(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		selfEndpoints         []string
		votingMemberEndpoints []string
		discoveredEndpoints   []string

		expected []string
	}{
		{
			name:                  "voting members first, self dropped",
			selfEndpoints:         []string{"https://172.20.1.3:2379"},
			votingMemberEndpoints: []string{"https://172.20.1.2:2379"},
			// as GetEndpoints() returns them: sorted, host:port, including the Kubernetes control
			// plane endpoint (a load balancer which doesn't serve etcd) and this node itself
			discoveredEndpoints: []string{"172.20.1.1:2379", "172.20.1.2:2379", "172.20.1.3:2379", "172.20.1.4:2379"},
			expected:            []string{"172.20.1.2:2379", "172.20.1.1:2379", "172.20.1.4:2379"},
		},
		{
			name:                "no voting member endpoints known, fall back to discovery",
			selfEndpoints:       []string{"https://172.20.1.3:2379"},
			discoveredEndpoints: []string{"172.20.1.1:2379", "172.20.1.3:2379"},
			expected:            []string{"172.20.1.1:2379"},
		},
		{
			name:                  "IPv6 endpoints",
			selfEndpoints:         []string{"https://[fd00::3]:2379"},
			votingMemberEndpoints: []string{"https://[fd00::2]:2379", "https://[fd00::3]:2379"},
			discoveredEndpoints:   []string{"[fd00::2]:2379", "[fd00::3]:2379"},
			expected:              []string{"[fd00::2]:2379"},
		},
		{
			name:                  "multiple advertised addresses per node",
			selfEndpoints:         []string{"https://172.20.1.3:2379", "https://[fd00::3]:2379"},
			votingMemberEndpoints: []string{"https://172.20.1.2:2379", "https://[fd00::2]:2379"},
			discoveredEndpoints:   []string{"172.20.1.3:2379", "[fd00::3]:2379"},
			expected:              []string{"172.20.1.2:2379", "[fd00::2]:2379"},
		},
		{
			name:          "only self is known",
			selfEndpoints: []string{"https://172.20.1.3:2379"},
			// nothing to promote against: the caller retries until an endpoint shows up
			discoveredEndpoints: []string{"172.20.1.3:2379"},
			expected:            []string{},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, services.PromotionEndpoints(test.selfEndpoints, test.votingMemberEndpoints, test.discoveredEndpoints))
		})
	}
}
