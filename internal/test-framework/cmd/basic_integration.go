// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"context"
	"log"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/internal/test-framework/pkg/basicintegration"
	"github.com/talos-systems/talos/pkg/constants"
)

var (
	cleanup     bool
	clusterName string
	kubeConfig  string
	runnerImage string
	talosImage  string
	talosConfig string
)

// Add basic-integration command
var basicIntegrationCmd = &cobra.Command{
	Use:   "basic-integration run|destroy",
	Short: "Runs the docker-based basic integration test",
}

var runBasicIntegrationCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs the docker-based basic integration test",
	Run: func(cmd *cobra.Command, args []string) {
		bi, err := basicintegration.New(
			basicintegration.WithTalosConfig(talosConfig),
			basicintegration.WithKubeConfig(kubeConfig),
			basicintegration.WithContainerImage(runnerImage),
			basicintegration.WithCleanup(cleanup),
			basicintegration.WithClusterName(clusterName),
			basicintegration.WithTalosImage(talosImage),
		)
		if err != nil {
			log.Fatal(err)
		}

		if err = bi.Run(context.Background()); err != nil {
			log.Fatal(err)
		}
	},
}

var destroyBasicIntegrationCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroys the docker-based basic integration test",
	Run: func(cmd *cobra.Command, args []string) {
		bi, err := basicintegration.New()
		if err != nil {
			log.Fatal(err)
		}

		if err = bi.Destroy(context.Background()); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	kubeContainer := constants.KubernetesImage + ":v" + constants.DefaultKubernetesVersion

	basicIntegrationCmd.Flags().BoolVar(&cleanup, "cleanup", false, "Cleanup the created cluster after completion")
	basicIntegrationCmd.Flags().StringVarP(&talosImage, "talos-image", "t", "", "Talos container to use for cluster creation")
	basicIntegrationCmd.PersistentFlags().StringVarP(&runnerImage, "runner-image", "r", kubeContainer, "Container to run tests from")
	basicIntegrationCmd.PersistentFlags().StringVarP(&clusterName, "cluster-name", "n", "integration", "Name of the cluster to create")
	basicIntegrationCmd.PersistentFlags().StringVarP(&talosConfig, "talosconfig", "c", "<tmpdir>/e2e*/talosconfig", "Path to talos config file")
	basicIntegrationCmd.PersistentFlags().StringVarP(&kubeConfig, "kubeconfig", "k", "<tmpdir>/e2e*/kubeconfig", "Path to the kubeconfig file")

	basicIntegrationCmd.AddCommand(runBasicIntegrationCmd)
	basicIntegrationCmd.AddCommand(destroyBasicIntegrationCmd)
	rootCmd.AddCommand(basicIntegrationCmd)
}
