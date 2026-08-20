// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	containersadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/containers"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

func TestOCISpecOpts(t *testing.T) {
	t.Parallel()

	grantable := []string{"CAP_NET_BIND_SERVICE", "CAP_CHOWN", "CAP_SETFCAP"}

	tests := []struct {
		name       string
		spec       containers.ContainerSecuritySpec
		expectOpts int
	}{
		{
			name: "privileged mode",
			spec: containers.ContainerSecuritySpec{
				Privileged: true,
			},
			expectOpts: 2,
		},
		{
			name: "restricted mode",
			spec: containers.ContainerSecuritySpec{
				Privileged: false,
			},
			expectOpts: 2,
		},
		{
			name: "drop capabilities",
			spec: containers.ContainerSecuritySpec{
				CapabilitiesDrop: []string{"NET_RAW"},
			},
			expectOpts: 3,
		},
		{
			name: "drop all capabilities",
			spec: containers.ContainerSecuritySpec{
				CapabilitiesDrop: []string{"ALL"},
			},
			expectOpts: 3,
		},
		{
			name: "add capabilities",
			spec: containers.ContainerSecuritySpec{
				CapabilitiesAdd: []string{"NET_BIND_SERVICE", "CHOWN"},
			},
			expectOpts: 3,
		},
		{
			name: "drop and add capabilities",
			spec: containers.ContainerSecuritySpec{
				CapabilitiesDrop: []string{"NET_RAW", "SETFCAP"},
				CapabilitiesAdd:  []string{"NET_BIND_SERVICE"},
			},
			expectOpts: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := containersadapter.ContainerSecuritySpec(&tt.spec).OCISpecOpts(grantable)
			assert.Equal(t, tt.expectOpts, len(opts))
		})
	}
}

func TestPrefixCapabilities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    []string
		expected []string
	}{
		{
			input:    []string{},
			expected: []string{},
		},
		{
			input:    []string{"NET_BIND_SERVICE"},
			expected: []string{"CAP_NET_BIND_SERVICE"},
		},
		{
			input:    []string{"CHOWN", "SETFCAP", "NET_RAW"},
			expected: []string{"CAP_CHOWN", "CAP_SETFCAP", "CAP_NET_RAW"},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()

			result := containersadapter.PrefixCapabilities(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
