// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package basicintegration

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/internal/test-framework/pkg/checker"
	"github.com/talos-systems/talos/internal/test-framework/pkg/runner"

	"github.com/talos-systems/talos/internal/test-framework/pkg/runner/container"
	containerRunner "github.com/talos-systems/talos/internal/test-framework/pkg/runner/container"
	localRunner "github.com/talos-systems/talos/internal/test-framework/pkg/runner/local"
)

// BasicIntegration holds data relating to the current run of the
// basic integration test.
type BasicIntegration struct {
	clusterName     string
	containerImage  string
	integrationTest string
	osctl           string
	kubeConfig      string
	talosConfig     string
	talosImage      string
	tmpDir          string
	cleanup         bool
}

// New instantiates a basic integration tester.
func New(setters ...Option) (*BasicIntegration, error) {
	bi := defaultOptions()

	var result *multierror.Error
	for _, setter := range setters {
		result = multierror.Append(result, setter(bi))
	}

	return bi, result.ErrorOrNil()
}

// Run performs a set of basic integration tests against a talos
// cluster. If necessary, a docker-based talos cluster will be
// created.
func (b *BasicIntegration) Run(ctx context.Context) (err error) {
	var cRunner runner.Runner

	// Create container runner to execute checks
	if cRunner, err = b.createRunnerContainer(); err != nil {
		log.Fatal(err)
	}

	// Create basic integration cluster
	if err = b.createCluster(ctx, cRunner); err != nil {
		log.Fatal(err)
	}

	// nolint: errcheck
	defer cRunner.Cleanup(ctx)

	// TODO readd
	/*
		if b.cleanup {
			// nolint: errcheck
			defer destroyCluster(ctx, b.osctl, b.clusterName)

			// We don't want to do a filepath.Dir(talosConfig) in case
			// someone is actually using ~/talosconfig.
			// nolint: errcheck
			//defer os.RemoveAll(b.talosConfig)
		}
	*/

	for _, check := range b.retryChecks(ctx) {
		if err = cRunner.Check(ctx, check); err != nil {
			return err
		}
	}

	for _, check := range b.oneShotChecks(ctx) {
		if err = cRunner.Run(ctx, check); err != nil {
			return err
		}
	}

	return nil
}

// createCluster runs through `osctl cluster create` to prepare a cluster for
// basic integration tests.
func (b *BasicIntegration) createCluster(ctx context.Context, cRunner runner.Runner) (err error) {
	// Skip basic integration cluster creation if it already exists
	// and we have a valid talosconfig
	if b.clusterExists(ctx, cRunner) {
		_, err = os.Stat(b.talosConfig)
		if os.IsNotExist(err) {
			return fmt.Errorf("talosconfig %q does not exist but %s cluster already running", b.talosConfig, b.clusterName)
		}

		return err
	}

	runner, err := localRunner.New()
	if err != nil {
		return err
	}

	args := []string{
		"--talosconfig",
		b.talosConfig,
		"cluster",
		"create",
		"--name",
		b.clusterName,
		"--masters",
		"3",
		"--mtu",
		"1440",
		"--cpus",
		"4.0",
	}

	if b.talosImage != "" {
		args = append(args, "--image", b.talosImage)
	}

	clusterCreate := checker.Check{
		Command: exec.CommandContext(ctx, b.osctl, args...),
		Name:    "Creating cluster",
	}

	if err = runner.Run(ctx, clusterCreate); err != nil {
		return err
	}

	args = []string{
		"--talosconfig",
		b.talosConfig,
		"config",
		"endpoint",
		"10.5.0.2",
	}

	osctlEndpoint := checker.Check{
		Command: exec.CommandContext(ctx, b.osctl, args...),
		Name:    "Set osctl endpoint",
	}

	if err = runner.Run(ctx, osctlEndpoint); err != nil {
		return err
	}

	return nil
}

// createRunnerContainer creates a container to use for invoking the commands
// to run our checks against the specified cluster.
func (b *BasicIntegration) createRunnerContainer() (cRunner runner.Runner, err error) {
	opts := []container.Option{
		containerRunner.WithImage(b.containerImage),
		containerRunner.WithClient(client.FromEnv),
		containerRunner.WithTTY(true),
		containerRunner.WithLabel("talos.owned", "true"),
		containerRunner.WithLabel("talos.integration.name", b.clusterName),
		containerRunner.WithClusterName(b.clusterName),
		containerRunner.WithMount(mount.Mount{
			Type:   mount.TypeBind,
			Source: b.osctl,
			Target: "/bin/osctl",
		},
		),
		containerRunner.WithMount(mount.Mount{
			Type:   mount.TypeBind,
			Source: b.integrationTest,
			Target: "/bin/integration-test",
		},
		),
	}

	opts = append(opts, containerRunner.WithMount(mount.Mount{
		Type:   mount.TypeBind,
		Source: filepath.Dir(b.talosConfig),
		Target: filepath.Dir(b.talosConfig),
	},
	),
	)

	if filepath.Dir(b.talosConfig) != filepath.Dir(b.kubeConfig) {
		opts = append(opts, containerRunner.WithMount(mount.Mount{
			Type:   mount.TypeBind,
			Source: filepath.Dir(b.kubeConfig),
			Target: filepath.Dir(b.kubeConfig),
		},
		),
		)
	}

	return containerRunner.New(opts...)
}

// clusterExists attempst to discover if the cluster is already running by
// making use of the `talos.owned` and `talos.cluster.name` labels.
func (b *BasicIntegration) clusterExists(ctx context.Context, cRunner runner.Runner) bool {
	filters := filters.NewArgs()
	filters.Add("label", fmt.Sprintf("%s=%s", "talos.owned", "true"))
	filters.Add("label", fmt.Sprintf("%s=%s", "talos.cluster.name", b.clusterName))

	containers, err := cRunner.(*container.ContainerRunner).Client.ContainerList(ctx, types.ContainerListOptions{Filters: filters})
	if err != nil {
		// Treat this as a fatal error since we have no idea what the system
		// state will be if we failed to list containers.
		log.Fatal(err)
	}

	if len(containers) > 0 {
		return true
	}

	return false
}

// Destroy tears down an integration cluster.
func (b *BasicIntegration) Destroy(ctx context.Context) (err error) {
	runner, err := localRunner.New()
	if err != nil {
		log.Fatal(err)
	}

	clusterDestroy := checker.Check{
		Command: exec.CommandContext(ctx, b.osctl, "cluster", "destroy", "--name", b.clusterName),
		Name:    "Destroying cluster",
	}

	return runner.Run(ctx, clusterDestroy)
}
