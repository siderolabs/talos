// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos //nolint:testpackage // to test unexported commands and helpers

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// namespaceSelectorCommands are the commands which let the namespace be chosen.
//
// Listed by name as well so that a failure says which command is missing the wiring rather than
// pointing at an anonymous entry.
func namespaceSelectorCommands() []struct {
	name string
	cmd  *cobra.Command
} {
	return []struct {
		name string
		cmd  *cobra.Command
	}{
		{"logs", logsCmd},
		{"containers", containersCmd},
		{"stats", statsCmd},
		{"restart", restartCmd},
	}
}

// resetNamespaceFlags restores the namespace selectors to their defaults once the test is done.
//
// The commands are package-level singletons whose flags are bound to package-level variables, so a
// parsed flag would otherwise leak into every test that runs afterwards. Setting each flag back to its
// default drives that through the bound variable as well, and clearing Changed is what makes a later
// parse look like a first one to the flag-group validation.
func resetNamespaceFlags(t *testing.T, cmd *cobra.Command) {
	t.Helper()

	t.Cleanup(func() {
		for _, name := range []string{"kubernetes", "namespace"} {
			flag := cmd.Flags().Lookup(name)
			require.NotNil(t, flag)
			require.NoError(t, flag.Value.Set(flag.DefValue))

			flag.Changed = false
		}
	})
}

// TestNamespaceFlagsAreMutuallyExclusive covers --kubernetes and --namespace being refused together.
//
// They are two ways of naming one containerd namespace, so accepting both would leave the command
// reading one namespace while the operator had asked for another. cobra checks flag groups after the
// arguments but before RunE, so validating the group directly is the same check the command performs,
// without connecting to anything.
func TestNamespaceFlagsAreMutuallyExclusive(t *testing.T) {
	for _, tc := range namespaceSelectorCommands() {
		t.Run(tc.name, func(t *testing.T) {
			resetNamespaceFlags(t, tc.cmd)

			require.NoError(t, tc.cmd.ParseFlags([]string{
				"--kubernetes",
				"--namespace", constants.TalosContainersContainerdNamespace,
			}))

			err := tc.cmd.ValidateFlagGroups()

			require.Error(t, err, "expected --kubernetes and --namespace to be refused together")
			assert.Contains(t, err.Error(), "kubernetes")
			assert.Contains(t, err.Error(), "namespace")
		})
	}
}

// TestNamespaceFlagsAreAcceptedSeparately is the other half: the exclusion has to reject only the
// combination, not either flag on its own.
//
// Asserting the rejection alone would also pass if the flags were somehow always in conflict.
func TestNamespaceFlagsAreAcceptedSeparately(t *testing.T) {
	for _, tc := range namespaceSelectorCommands() {
		for _, args := range [][]string{
			{"--kubernetes"},
			{"--namespace", constants.TalosContainersContainerdNamespace},
			{},
		} {
			t.Run(tc.name+" "+argsName(args), func(t *testing.T) {
				resetNamespaceFlags(t, tc.cmd)

				require.NoError(t, tc.cmd.ParseFlags(args))
				assert.NoError(t, tc.cmd.ValidateFlagGroups())
			})
		}
	}
}

func argsName(args []string) string {
	if len(args) == 0 {
		return "no flags"
	}

	return args[0]
}

// TestResolveContainerNamespace pins the namespace and driver each selector resolves to.
//
// The driver is the half that is easy to get wrong and hard to notice: asking for the taloscontainers
// namespace through the CRI driver is refused by the server outright, and the mapping is what the
// --namespace flag exists to reach.
func TestResolveContainerNamespace(t *testing.T) {
	for _, tc := range []struct {
		name          string
		flags         containerNamespaceFlag
		wantNamespace string
		wantDriver    common.ContainerDriver
		wantErr       bool
	}{
		{
			name:          "system namespace",
			flags:         containerNamespaceFlag{imageCmdFlagsType: imageCmdFlagsType{namespace: "system"}},
			wantNamespace: constants.SystemContainerdNamespace,
			wantDriver:    common.ContainerDriver_CONTAINERD,
		},
		{
			name:          "cri symbol resolves to the k8s.io namespace via the CRI driver",
			flags:         containerNamespaceFlag{imageCmdFlagsType: imageCmdFlagsType{namespace: "cri"}},
			wantNamespace: constants.K8sContainerdNamespace,
			wantDriver:    common.ContainerDriver_CRI,
		},
		{
			name:          "deprecated --kubernetes behaves like --namespace cri",
			flags:         containerNamespaceFlag{kubernetes: true},
			wantNamespace: constants.K8sContainerdNamespace,
			wantDriver:    common.ContainerDriver_CRI,
		},
		{
			name:          "taloscontainers is read through containerd",
			flags:         containerNamespaceFlag{imageCmdFlagsType: imageCmdFlagsType{namespace: constants.TalosContainersContainerdNamespace}},
			wantNamespace: constants.TalosContainersContainerdNamespace,
			wantDriver:    common.ContainerDriver_CONTAINERD,
		},
		{
			name:    "inmem is not supported by this command",
			flags:   containerNamespaceFlag{imageCmdFlagsType: imageCmdFlagsType{namespace: "inmem"}},
			wantErr: true,
		},
		{
			name:    "an unrecognized namespace errors",
			flags:   containerNamespaceFlag{imageCmdFlagsType: imageCmdFlagsType{namespace: "bogus"}},
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			namespace, driver, err := tc.flags.resolveContainerNamespace()

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantNamespace, namespace)
			assert.Equal(t, tc.wantDriver, driver)
		})
	}
}

// TestKubernetesFlagIsDeprecated covers --kubernetes being marked deprecated in favor of --namespace
// cri, on every command that carries the namespace selector.
func TestKubernetesFlagIsDeprecated(t *testing.T) {
	for _, tc := range namespaceSelectorCommands() {
		t.Run(tc.name, func(t *testing.T) {
			flag := tc.cmd.Flags().Lookup("kubernetes")

			require.NotNil(t, flag)
			assert.NotEmpty(t, helpers.DeprecationMessage(flag), "expected --kubernetes to be marked deprecated")
		})
	}
}

// TestStripTalosContainerLogPrefix covers turning taloscontainers log-buffer ids back into bare
// container names for shell completion.
func TestStripTalosContainerLogPrefix(t *testing.T) {
	for _, tc := range []struct {
		name string
		ids  []string
		want []string
	}{
		{
			name: "nil input",
			ids:  nil,
			want: nil,
		},
		{
			name: "mixed prefixed and unprefixed ids",
			ids:  []string{constants.TalosContainersLogPrefix + "foo", "apid", constants.TalosContainersLogPrefix + "bar"},
			want: []string{"foo", "bar"},
		},
		{
			name: "id equal to the bare prefix strips to an empty name",
			ids:  []string{constants.TalosContainersLogPrefix},
			want: []string{""},
		},
		{
			name: "no prefixed ids",
			ids:  []string{"apid", "kubelet"},
			want: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, stripTalosContainerLogPrefix(tc.ids))
		})
	}
}
