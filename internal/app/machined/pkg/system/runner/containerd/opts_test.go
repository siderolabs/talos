// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd_test

import (
	"testing"

	ctrdcontainers "github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	containerdrunner "github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/containerd"
)

func TestOCISpecOptsSeccomp(t *testing.T) {
	t.Parallel()

	// Grants every capability the way the privileged container profile does, ahead of the
	// runner's own seccomp option, which must still have the final say.
	privileged := runner.WithOCISpecOpts(oci.WithCapabilities([]string{"CAP_SYS_ADMIN"}))

	for _, test := range []struct {
		name       string
		setters    []runner.Option
		unconfined bool
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
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			opts := containerdrunner.OCISpecOpts(&runner.Args{ID: "test", ProcessArgs: []string{"/bin/true"}}, test.setters...)

			spec, err := oci.GenerateSpec(namespaces.WithNamespace(t.Context(), "test"), nil, &ctrdcontainers.Container{ID: "test"}, opts...)
			require.NoError(t, err)

			if test.unconfined {
				assert.Nil(t, spec.Linux.Seccomp)
			} else {
				assert.NotNil(t, spec.Linux.Seccomp)
			}

			// Opting out of seccomp must not take no-new-privileges along with it.
			assert.True(t, spec.Process.NoNewPrivileges)
		})
	}
}
