// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"slices"
	"testing"

	ctrdcontainers "github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	containersadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/containers"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

func TestOCISpecOptsSysfs(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name       string
		privileged bool
		expect     string
	}{
		{name: "privileged mounts sysfs read-write", privileged: true, expect: "rw"},
		{name: "restricted mounts sysfs read-only", privileged: false, expect: "ro"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := containersadapter.ContainerSecuritySpec(&containers.ContainerSecuritySpec{
				Privileged: tt.privileged,
			}).OCISpecOpts([]string{"CAP_CHOWN"})

			spec, err := oci.GenerateSpec(namespaces.WithNamespace(t.Context(), "test"), nil, &ctrdcontainers.Container{ID: "test"}, opts...)
			require.NoError(t, err)

			idx := slices.IndexFunc(spec.Mounts, func(m specs.Mount) bool { return m.Destination == "/sys" })
			require.NotEqual(t, -1, idx, "no /sys mount in the generated spec")

			assert.Contains(t, spec.Mounts[idx].Options, tt.expect)
		})
	}
}

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
			expectOpts: 3,
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

func TestCgroupResources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		spec         containers.ContainerResourcesSpec
		expectMemory bool
		expectMax    *int64
		expectSwap   *int64
	}{
		{
			name: "no limits",
		},
		{
			name:         "memory only",
			spec:         containers.ContainerResourcesSpec{MemoryLimit: 1 << 29},
			expectMemory: true,
			expectMax:    new(int64(1 << 29)),
		},
		{
			// The swap ceiling needs no memory ceiling to go with it.
			name:         "swap only",
			spec:         containers.ContainerResourcesSpec{SwapLimit: new(uint64(0))},
			expectMemory: true,
			expectSwap:   new(int64(0)),
		},
		{
			name:         "memory and swap",
			spec:         containers.ContainerResourcesSpec{MemoryLimit: 1 << 29, SwapLimit: new(uint64(1 << 28))},
			expectMemory: true,
			expectMax:    new(int64(1 << 29)),
			expectSwap:   new(int64(1 << 28)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resources := containersadapter.ContainerResourcesSpec(&tt.spec).CgroupResources()

			if !tt.expectMemory {
				assert.Nil(t, resources.Memory)

				return
			}

			if assert.NotNil(t, resources.Memory) {
				assert.Equal(t, tt.expectMax, resources.Memory.Max)
				assert.Equal(t, tt.expectSwap, resources.Memory.Swap)
			}
		})
	}
}
