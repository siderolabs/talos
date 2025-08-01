// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mgmt

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/debug"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/gen"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/inject"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/machineconfig"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/global"
)

// Commands is a list of commands published by the package.
var Commands []*cobra.Command

// GenV1Alpha1Config generates the Talos config bundle
//
// Kept with this name in this package for backwards-compatibility.
var GenV1Alpha1Config = gen.GenerateConfigBundle

var persistentFlags = global.GetPersistentFlags(&global.Args{})

func addCommand(cmd *cobra.Command) {
	cmd.PersistentFlags().AddFlagSet(persistentFlags)
	Commands = append(Commands, cmd)
}

func init() {
	persistentFlags.VisitAll(func(f *pflag.Flag) { f.Hidden = true })

	addCommand(cluster.Cmd)
	addCommand(gen.Cmd)
	addCommand(debug.Cmd)
	addCommand(inject.Cmd)
	addCommand(machineconfig.Cmd)
}
