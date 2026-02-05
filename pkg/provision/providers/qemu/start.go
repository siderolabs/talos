// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/hashicorp/go-multierror"

	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

// Start restarts an existing cluster that was previously created but is now stopped.
// This recreates the network bridge and restarts all helper services and VM nodes.
func (p *provisioner) Start(ctx context.Context, cluster provision.Cluster, opts ...provision.Option) error {
	options := provision.DefaultOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return err
		}
	}

	state, ok := cluster.(*provision.State)
	if !ok {
		return fmt.Errorf("cluster is not a *provision.State")
	}

	fmt.Fprintln(options.LogWriter, "recreating network bridge", state.BridgeName)

	if err := p.RecreateNetwork(ctx, state, options); err != nil {
		return fmt.Errorf("error recreating network: %w", err)
	}

	fmt.Fprintln(options.LogWriter, "starting load balancer")

	if err := p.StartLoadBalancer(state); err != nil {
		return fmt.Errorf("error starting loadbalancer: %w", err)
	}

	fmt.Fprintln(options.LogWriter, "starting dnsd")

	if err := p.StartDNSd(state); err != nil {
		return fmt.Errorf("error starting dnsd: %w", err)
	}

	fmt.Fprintln(options.LogWriter, "starting nodes")

	if err := p.startNodes(ctx, state, &options); err != nil {
		return err
	}

	fmt.Fprintln(options.LogWriter, "starting dhcpd")

	if err := p.StartDHCPd(state); err != nil {
		return fmt.Errorf("error starting dhcpd: %w", err)
	}

	return nil
}

// startNodes starts all nodes from saved state.
func (p *provisioner) startNodes(ctx context.Context, state *provision.State, options *provision.Options) error {
	errCh := make(chan error)
	nodes := state.ClusterInfo.Nodes

	for _, node := range nodes {
		go func(node provision.NodeInfo) {
			errCh <- p.startNode(ctx, state, node, options)
		}(node)
	}

	var multiErr *multierror.Error

	for range nodes {
		multiErr = multierror.Append(multiErr, <-errCh)
	}

	return multiErr.ErrorOrNil()
}

// startNode starts a single node from saved state.
func (p *provisioner) startNode(_ context.Context, state *provision.State, node provision.NodeInfo, options *provision.Options) error {
	pidPath := state.GetRelativePath(fmt.Sprintf("%s.pid", node.Name))

	// Check if already running
	if vm.IsProcessRunning(pidPath) {
		fmt.Fprintf(options.LogWriter, "node %s already running\n", node.Name)

		return nil
	}

	// Read the saved launch config
	configPath := state.GetRelativePath(fmt.Sprintf("%s.config", node.Name))

	configFile, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("error opening config file for %s: %w", node.Name, err)
	}

	defer configFile.Close() //nolint:errcheck

	// Verify the config is valid JSON
	var launchConfig LaunchConfig
	if err := json.NewDecoder(configFile).Decode(&launchConfig); err != nil {
		return fmt.Errorf("error decoding config file for %s: %w", node.Name, err)
	}

	// Seek back to beginning for stdin
	if _, err := configFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("error seeking config file for %s: %w", node.Name, err)
	}

	logFile, err := os.OpenFile(state.GetRelativePath(fmt.Sprintf("%s.log", node.Name)), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return fmt.Errorf("error opening log file for %s: %w", node.Name, err)
	}

	defer logFile.Close() //nolint:errcheck

	fmt.Fprintf(options.LogWriter, "starting node %s\n", node.Name)

	cmd := exec.Command(state.SelfExecutable, "qemu-launch") //nolint:noctx // runs in background
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = configFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // daemonize
	}

	if err = cmd.Start(); err != nil {
		return fmt.Errorf("error starting node %s: %w", node.Name, err)
	}

	if err = os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), os.ModePerm); err != nil {
		return fmt.Errorf("error writing PID file for %s: %w", node.Name, err)
	}

	return nil
}
