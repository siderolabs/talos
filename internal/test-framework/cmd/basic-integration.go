// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/internal/test-framework/internal/pkg/runner"
	"github.com/talos-systems/talos/pkg/constants"
)

const (
	timeoutSeconds = 300
	tmpPath        = "/tmp/e2e"
)

var (
	cleanup    bool
	talosImage = "docker.io/autonomy/talos:" + os.Getenv("TAG")
	kubeImage  = constants.KubernetesImage + ":v" + constants.DefaultKubernetesVersion
)

// Add basic-integration command
var basicIntegrationCmd = &cobra.Command{
	Use:   "basic-integration",
	Short: "Runs the docker-based basic integration test",
	Run: func(cmd *cobra.Command, args []string) {
		if err := basicIntegration(); err != nil {
			panic(err)
		}
	},
}

// Stub out the basics for our runner configuration
var runnerConfig = &runner.ContainerConfigs{
	ContainerConfig: &container.Config{
		Image: kubeImage,
		Tty:   true,
		Env: []string{
			"TALOSCONFIG=" + filepath.Join(tmpPath, "talosconfig"),
			"KUBECONFIG=" + filepath.Join(tmpPath, "kubeconfig"),
		},
	},
	HostConfig: &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: tmpPath,
				Target: tmpPath,
			},
		},
	},
}

func init() {
	basicIntegrationCmd.Flags().BoolVar(&cleanup, "cleanup", false, "Cleanup the created cluster after completion")
	rootCmd.AddCommand(basicIntegrationCmd)
}

// basicIntegration creates a docker-based talos cluster
// nolint: gocyclo
func basicIntegration() error {
	// Ensure tmp dir and set env vars
	if err := os.MkdirAll("/tmp/e2e", os.ModePerm); err != nil {
		return err
	}

	if err := os.Setenv("TALOSCONFIG", filepath.Join(tmpPath, "talosconfig")); err != nil {
		return err
	}

	if err := os.Setenv("KUBECONFIG", filepath.Join(tmpPath, "kubeconfig")); err != nil {
		return err
	}

	// Create docker client and pull down hyperkube
	ctx := context.Background()

	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	_, err = cli.ImagePull(ctx, kubeImage, types.ImagePullOptions{})
	if err != nil {
		return err
	}

	// Discover proper path to osctl build and add it to runner mounts
	osctlBinary := "osctl-darwin-amd64"
	if runtime.GOOS == "linux" {
		osctlBinary = "osctl-linux-amd64"
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return err
	}

	osctlBinPath := filepath.Join(currentDir, "build", osctlBinary)
	integrationBinPath := filepath.Join(currentDir, "bin", "integration-test")

	runnerConfig.HostConfig.Mounts = append(runnerConfig.HostConfig.Mounts,
		mount.Mount{
			Type:   mount.TypeBind,
			Source: osctlBinPath,
			Target: "/bin/osctl",
		},
		mount.Mount{
			Type:   mount.TypeBind,
			Source: integrationBinPath,
			Target: "/bin/integration-test",
		},
	)

	// Create basic integration cluster and defer cleaning it up if the cleanup flag is set
	log.Println("deploying basic integration cluster")

	if err = runner.CommandLocal(osctlBinPath + " cluster create --name integration --masters=3 --mtu 1440 --cpus 4.0 --image " + talosImage); err != nil {
		return err
	}

	if cleanup {
		// nolint: errcheck
		defer runner.CommandLocal(osctlBinPath + " cluster destroy --name integration")
	}

	// Set osctl to talk to master-1
	log.Println("targeting master-1 in talosconfig")

	if err = runner.CommandLocal(osctlBinPath + " config target 10.5.0.2"); err != nil {
		return err
	}

	// Wait for bootkube completion
	log.Println("waiting for bootkube completion")

	if err = runner.CommandInContainerWithTimeout(ctx, cli, runnerConfig, "osctl service bootkube | grep Finished", timeoutSeconds); err != nil {
		return err
	}

	// Wait for kubeconfig and target master-1
	log.Println("waiting for kubeconfig")

	if err = runner.CommandInContainerWithTimeout(ctx, cli, runnerConfig, "osctl kubeconfig /tmp/e2e -f", timeoutSeconds); err != nil {
		return err
	}

	log.Println("targeting master-1 in kubeconfig")

	if err = runner.CommandInContainer(
		ctx,
		cli,
		runnerConfig,
		"kubectl --kubeconfig ${KUBECONFIG} config set-cluster local --server https://10.5.0.2:6443",
	); err != nil {
		return err
	}

	// Wait for all nodes to report in
	log.Println("waiting for nodes to report in")

	if err = runner.CommandInContainerWithTimeout(
		ctx,
		cli,
		runnerConfig,
		"kubectl get nodes -o go-template='{{ len .items }}' | grep 4 >/dev/null",
		timeoutSeconds,
	); err != nil {
		return err
	}

	// Check all nodes ready
	log.Println("waiting for all nodes to be ready")

	if err = runner.CommandInContainerWithTimeout(
		ctx,
		cli,
		runnerConfig,
		"kubectl wait --timeout="+strconv.Itoa(timeoutSeconds)+"s --for=condition=ready=true --all nodes",
		timeoutSeconds,
	); err != nil {
		return err
	}

	// Verify HA control plane
	log.Println("Waiting for all masters to become ready")

	if err = runner.CommandInContainerWithTimeout(
		ctx,
		cli,
		runnerConfig,
		"kubectl get nodes -l node-role.kubernetes.io/master='' -o go-template='{{ len .items }}' | grep 3 >/dev/null",
		timeoutSeconds,
	); err != nil {
		return err
	}

	// Show etcd running
	for _, ip := range []string{"10.5.0.2", "10.5.0.3", "10.5.0.4"} {
		if err = runner.CommandInContainer(
			ctx,
			cli,
			runnerConfig,
			"osctl config target "+ip+" && osctl -t "+ip+" service etcd | grep Running",
		); err != nil {
			return err
		}
	}

	// Show containers and svcs
	for _, desiredCommand := range []string{"containers", "services"} {
		if err = runner.CommandInContainer(
			ctx,
			cli,
			runnerConfig,
			"osctl --target 10.5.0.2,10.5.0.3,10.5.0.4,10.5.0.5 "+desiredCommand,
		); err != nil {
			return err
		}
	}

	// Run integration tests
	if err = runner.CommandInContainer(
		ctx,
		cli,
		runnerConfig,
		"integration-test -test.v -talos.target 10.5.0.2",
	); err != nil {
		return err
	}

	return nil
}
