// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package machineconfig_test

import (
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/gen"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/machineconfig"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
)

func TestGenerateConfig(t *testing.T) {
	for _, tt := range []struct {
		name       string
		command    *cobra.Command
		subcommand string
	}{
		{name: "gen config", command: gen.Cmd, subcommand: "config"},
		{name: "machineconfig gen", command: machineconfig.Cmd, subcommand: "gen"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			output := t.TempDir()

			const image = "ghcr.io/siderolabs/installer:v1.14.5"

			tt.command.SetArgs([]string{
				tt.subcommand, "test-cluster", "https://192.0.2.1:6443",
				"--output", output,
				"--install-image", image, "--force",
				"--with-docs=false", "--with-examples=false",
			})

			_, err := tt.command.ExecuteC()
			require.NoError(t, err)

			cfg, err := configloader.NewFromFile(filepath.Join(output, "worker.yaml"))
			require.NoError(t, err)
			require.Equal(t, machine.TypeWorker, cfg.Machine().Type())
			require.Equal(t, image, cfg.UnattendedInstallConfig().InstallerImage())
		})
	}
}
