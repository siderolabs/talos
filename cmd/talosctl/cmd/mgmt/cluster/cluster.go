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
	Long:  ``,
}

// CmdOps are the options for the cluster command.
type CmdOps struct {
	ProvisionerName string
	StateDir        string
	ClusterName     string
}

var (
	defaultStateDir string

	// DefaultCNIDir is the default location of the cni binaries.
	DefaultCNIDir string
)

// Flags are the flags of the cluster command.
var Flags CmdOps

func init() {
	talosDir, err := clientconfig.GetTalosDirectory()
	if err == nil {
		defaultStateDir = filepath.Join(talosDir, "clusters")
		DefaultCNIDir = filepath.Join(talosDir, "cni")
	}

	Cmd.PersistentFlags().StringVar(&Flags.ProvisionerName, ProvisionerFlag, providers.DockerProviderName, "Talos cluster provisioner to use")
	Cmd.PersistentFlags().StringVar(&Flags.StateDir, "state", defaultStateDir, "directory path to store cluster state")
	Cmd.PersistentFlags().StringVar(&Flags.ClusterName, "name", "talos-default", "the name of the cluster")
}
