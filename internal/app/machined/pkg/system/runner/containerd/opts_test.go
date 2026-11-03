// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd_test

import (
	"context"
	"testing"

	ctrdcontainers "github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	containersadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/containers"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	containerdrunner "github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

func TestOCISpecOptsAmbientCapabilities(t *testing.T) {
	t.Parallel()

	grantable := []string{"CAP_NET_BIND_SERVICE", "CAP_CHOWN", "CAP_SETFCAP"}

	withUID := func(uid uint32) oci.SpecOpts {
		return func(_ context.Context, _ oci.Client, _ *ctrdcontainers.Container, s *specs.Spec) error {
			s.Process.User.UID = uid

			return nil
		}
	}

	for _, test := range []struct {
		name     string
		uid      uint32
		security containers.ContainerSecuritySpec
		// userLast sets the user after the capability options, the way an option appended later
		// to the caller's chain would.
		userLast      bool
		disabled      bool
		expectAmbient []string
	}{
		{
			name: "non-root with a single added capability",
			uid:  65534,
			security: containers.ContainerSecuritySpec{
				CapabilitiesDrop: []string{"ALL"},
				CapabilitiesAdd:  []string{"NET_BIND_SERVICE"},
			},
			expectAmbient: []string{"CAP_NET_BIND_SERVICE"},
		},
		{
			name: "non-root user set after the capability options",
			uid:  65534,
			security: containers.ContainerSecuritySpec{
				CapabilitiesDrop: []string{"ALL"},
				CapabilitiesAdd:  []string{"NET_BIND_SERVICE"},
			},
			userLast:      true,
			expectAmbient: []string{"CAP_NET_BIND_SERVICE"},
		},
		{
			name:          "non-root privileged",
			uid:           1000,
			security:      containers.ContainerSecuritySpec{Privileged: true},
			expectAmbient: grantable,
		},
		{
			name: "non-root privileged with a partial drop",
			uid:  1000,
			security: containers.ContainerSecuritySpec{
				Privileged:       true,
				CapabilitiesDrop: []string{"CHOWN"},
			},
			expectAmbient: []string{"CAP_NET_BIND_SERVICE", "CAP_SETFCAP"},
		},
		{
			name:          "non-root restricted",
			uid:           1000,
			security:      containers.ContainerSecuritySpec{},
			expectAmbient: []string{},
		},
		{
			name:     "root privileged",
			uid:      0,
			security: containers.ContainerSecuritySpec{Privileged: true},
		},
		{
			name: "non-root with the option off",
			uid:  65534,
			security: containers.ContainerSecuritySpec{
				CapabilitiesDrop: []string{"ALL"},
				CapabilitiesAdd:  []string{"NET_BIND_SERVICE"},
			},
			disabled: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			callerOpts := containersadapter.ContainerSecuritySpec(&test.security).OCISpecOpts(grantable)

			if test.userLast {
				callerOpts = append(callerOpts, withUID(test.uid))
			} else {
				callerOpts = append([]oci.SpecOpts{withUID(test.uid)}, callerOpts...)
			}

			opts := containerdrunner.OCISpecOpts(
				&runner.Args{ID: "test", ProcessArgs: []string{"/bin/true"}},
				runner.WithOCISpecOpts(callerOpts...),
				runner.WithAmbientCapabilitiesForNonRoot(!test.disabled),
			)

			spec, err := oci.GenerateSpec(namespaces.WithNamespace(t.Context(), "test"), nil, &ctrdcontainers.Container{ID: "test"}, opts...)
			require.NoError(t, err)

			capabilities := spec.Process.Capabilities
			require.NotNil(t, capabilities)

			if test.expectAmbient == nil {
				assert.Empty(t, capabilities.Ambient)

				return
			}

			assert.ElementsMatch(t, test.expectAmbient, capabilities.Ambient)
			assert.ElementsMatch(t, capabilities.Permitted, capabilities.Ambient, "ambient differs from permitted")
			assert.ElementsMatch(t, capabilities.Permitted, capabilities.Inheritable, "inheritable differs from permitted")
		})
	}
}

func TestOCISpecOptsSeccomp(t *testing.T) {
	t.Parallel()

	// Grants every capability the way the privileged container profile does, ahead of the
	// runner's own seccomp option, which must still have the final say.
	privileged := runner.WithOCISpecOpts(oci.WithCapabilities([]string{"CAP_SYS_ADMIN"}))

	localhost := &specs.LinuxSeccomp{
		DefaultAction: specs.ActErrno,
		Syscalls: []specs.LinuxSyscall{
			{Names: []string{"read", "write", "exit_group"}, Action: specs.ActAllow},
		},
	}

	for _, test := range []struct {
		name       string
		setters    []runner.Option
		unconfined bool
		expected   *specs.LinuxSeccomp
	}{
		{
			name: "default profile",
		},
		{
			name:    "privileged keeps the default profile",
			setters: []runner.Option{privileged},
		},
		{
			name:    "explicitly confined",
			setters: []runner.Option{privileged, runner.WithSeccompUnconfined(false)},
		},
		{
			name:       "unconfined",
			setters:    []runner.Option{privileged, runner.WithSeccompUnconfined(true)},
			unconfined: true,
		},
		{
			name: "unconfined wins over a custom profile",
			setters: []runner.Option{
				runner.WithCustomSeccompProfile(func(*specs.LinuxSeccomp) {}),
				runner.WithSeccompUnconfined(true),
			},
			unconfined: true,
		},
		{
			name:     "localhost profile replaces the default one",
			setters:  []runner.Option{privileged, runner.WithSeccompProfile(localhost)},
			expected: localhost,
		},
		{
			name:    "nil localhost profile keeps the default one",
			setters: []runner.Option{runner.WithSeccompProfile(nil)},
		},
		{
			name: "localhost profile wins over a custom profile",
			setters: []runner.Option{
				runner.WithCustomSeccompProfile(func(*specs.LinuxSeccomp) {}),
				runner.WithSeccompProfile(localhost),
			},
			expected: localhost,
		},
		{
			name: "unconfined wins over a localhost profile",
			setters: []runner.Option{
				runner.WithSeccompProfile(localhost),
				runner.WithSeccompUnconfined(true),
			},
			unconfined: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			opts := containerdrunner.OCISpecOpts(&runner.Args{ID: "test", ProcessArgs: []string{"/bin/true"}}, test.setters...)

			spec, err := oci.GenerateSpec(namespaces.WithNamespace(t.Context(), "test"), nil, &ctrdcontainers.Container{ID: "test"}, opts...)
			require.NoError(t, err)

			switch {
			case test.unconfined:
				assert.Nil(t, spec.Linux.Seccomp)
			case test.expected != nil:
				assert.Equal(t, test.expected, spec.Linux.Seccomp)
				// The spec gets its own copy of the profile.
				assert.NotSame(t, test.expected, spec.Linux.Seccomp)
			default:
				assert.NotNil(t, spec.Linux.Seccomp)
				assert.Equal(t, specs.ActErrno, spec.Linux.Seccomp.DefaultAction, "expected the default profile")
			}

			// Opting out of seccomp must not take no-new-privileges along with it.
			assert.True(t, spec.Process.NoNewPrivileges)
		})
	}
}
