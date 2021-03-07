// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cluster implements "cluster" subcommands.
package cluster

import (
	"path/filepath"

	"github.com/spf13/cobra"

	clientconfig "github.com/talos-systems/talos/pkg/machinery/client/config"
)

// Cmd represents the cluster command.
var Cmd = &cobra.Command{
	Use:   "cluster",
	Short: "A collection of commands for managing local docker-based or firecracker-based clusters",
	Long:  ``,
}

var (
	provisionerName string
	stateDir        string
	clusterName     string

	defaultStateDir string
	defaultCNIDir   string
)

func init() {
	talosDir, err := clientconfig.GetTalosDirectory()
	if err == nil {
		defaultStateDir = filepath.Join(talosDir, "clusters")
		defaultCNIDir = filepath.Join(talosDir, "cni")
	}

	Cmd.PersistentFlags().StringVar(&provisionerName, "provisioner", "docker", "Talos cluster provisioner to use")
	Cmd.PersistentFlags().StringVar(&stateDir, "state", defaultStateDir, "directory path to store cluster state")
	Cmd.PersistentFlags().StringVar(&clusterName, "name", "talos-default", "the name of the cluster")
}
