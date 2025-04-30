// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cluster implements "cluster" subcommands.
package cluster

import (
	"path/filepath"

	"github.com/spf13/cobra"

	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/provision/providers"
)

const (
	// ProvisionerFlag is the flag with which the provisioner is configured.
	ProvisionerFlag = "provisioner"
)

// Cmd represents the cluster command.
var Cmd = &cobra.Command{
	Use:   "cluster",
	Short: "A collection of commands for managing local docker-based or QEMU-based clusters",
}

// CmdOps are the options for the cluster command.
type CmdOps struct {
	ProvisionerName string
	StateDir        string
	ClusterName     string
}

// Flags are the flags of the cluster command.
var Flags CmdOps

var (
	// DefaultStateDir is the default location of the cluster related file state.
	DefaultStateDir string
	// DefaultCNIDir is the default location of the CNI binaries.
	DefaultCNIDir string
)

func init() {
	talosDir, err := clientconfig.GetTalosDirectory()
	if err == nil {
		DefaultStateDir = filepath.Join(talosDir, "clusters")
		DefaultCNIDir = filepath.Join(talosDir, "cni")
	}

	Cmd.PersistentFlags().StringVar(&Flags.ProvisionerName, "provisioner", providers.DockerProviderName, "Talos cluster provisioner to use")
	Cmd.PersistentFlags().StringVar(&Flags.StateDir, "state", DefaultStateDir, "directory path to store cluster state")
	Cmd.PersistentFlags().StringVar(&Flags.ClusterName, "name", "talos-default", "the name of the cluster")
}
