// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/talos"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestLogsKindValidation(t *testing.T) {
	for _, cmd := range talos.Commands {
		if cmd.Name() != "logs" {
			continue
		}

		for _, test := range []struct {
			args []string
			want string
		}{
			{
				args: []string{"--kind=auto"},
				want: "invalid --kind",
			},
			{
				args: []string{"--kind="},
				want: "--kind must be",
			},
			{
				args: []string{"--kind=vm", "--namespace=cri"},
				want: "requires --namespace=system",
			},
			{
				args: []string{"--kind=service", "--namespace=" + constants.TalosContainersContainerdNamespace},
				want: "requires --namespace=system",
			},
			{
				args: []string{"--kind=vm", "-k"},
				want: "requires --namespace=system",
			},
		} {
			t.Run(test.args[0]+test.want, func(t *testing.T) {
				for _, name := range []string{"kind", "namespace", "kubernetes"} {
					flag := cmd.Flags().Lookup(name)
					value, changed := flag.Value.String(), flag.Changed

					t.Cleanup(func() {
						require.NoError(t, flag.Value.Set(value))
						flag.Changed = changed
					})
				}

				require.NoError(t, cmd.ParseFlags(test.args))
				err := cmd.RunE(cmd, []string{"guest"})
				require.ErrorContains(t, err, test.want)
			})
		}

		completion, ok := cmd.GetFlagCompletionFunc("kind")
		require.True(t, ok)

		names, directive := completion(cmd, nil, "")
		require.ElementsMatch(t, []string{"container", "vm", "service"}, names)
		require.NotZero(t, directive)

		return
	}

	t.Fatal("logs command not registered")
}

func TestVMLogsHelp(t *testing.T) {
	for _, cmd := range talos.Commands {
		if cmd.Name() != "logs" {
			continue
		}

		require.NotContains(t, cmd.Use, "vm:")
		require.NotNil(t, cmd.Flags().Lookup("kind"))
		require.Contains(t, cmd.Long, "--kind=vm")
		require.Contains(t, cmd.Long, "Admin")
		require.Contains(t, cmd.Long, "active file")
		require.Contains(t, cmd.Long, "one target node")

		return
	}

	t.Fatal("logs command not registered")
}
