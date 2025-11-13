// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package proxmox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/proxmox/remote"
	"net/netip"
)

const (
	dhcpPid = "dhcpd.pid"
	dhcpLog = "dhcpd.log"
)

// CreateDHCPd creates a DHCP server on the Proxmox node via SSH.
// This allows the DHCP server to access the bridge directly and receive broadcasts from VMs.
// Following the same pattern as QEMU provider but adapted for remote execution.
func (p *provisioner) CreateDHCPd(ctx context.Context, state *provision.State, clusterReq provision.ClusterRequest) error {
	statePath, err := state.StatePath()
	if err != nil {
		return err
	}

	// Copy state directory to Proxmox node
	// We'll use a temporary directory on the Proxmox node
	remoteStatePath := fmt.Sprintf("/tmp/talos-cluster-%s", clusterReq.Name)
	fmt.Printf("copying state directory to Proxmox node %s\n", remoteStatePath)

	remoteConfig := &remote.Config{URL: p.config.URL}
	host := remote.ExtractHostFromURL(p.config.URL)
	if host == "" {
		host = p.config.Node
	}

	// Create remote state directory
	_, err = remote.RunCommandContext(ctx, remoteConfig, host, fmt.Sprintf("mkdir -p %s", remoteStatePath))
	if err != nil {
		return fmt.Errorf("failed to create remote state directory: %w", err)
	}

	// Copy state files to Proxmox node using scp
	scpCmd := exec.CommandContext(ctx, "scp", "-r", "-o", "StrictHostKeyChecking=no", statePath+"/", fmt.Sprintf("root@%s:%s/", host, remoteStatePath))
	if err := scpCmd.Run(); err != nil {
		return fmt.Errorf("failed to copy state directory to Proxmox node: %w", err)
	}

	gatewayAddrs := xslices.Map(clusterReq.Network.GatewayAddrs, netip.Addr.String)

	// Build command to run on Proxmox node
	// We need to find the talosctl binary path
	talosctlPath := clusterReq.SelfExecutable
	if talosctlPath == "" {
		// Try to find talosctl in PATH
		if path, err := exec.LookPath("talosctl"); err == nil {
			talosctlPath = path
		} else {
			return fmt.Errorf("talosctl binary not found")
		}
	}

	// Copy talosctl to Proxmox node
	remoteTalosctlPath := fmt.Sprintf("/tmp/talosctl-%s", clusterReq.Name)
	scpTalosctlCmd := exec.CommandContext(ctx, "scp", "-o", "StrictHostKeyChecking=no", talosctlPath, fmt.Sprintf("root@%s:%s", host, remoteTalosctlPath))
	if err := scpTalosctlCmd.Run(); err != nil {
		return fmt.Errorf("failed to copy talosctl to Proxmox node: %w", err)
	}

	// Make talosctl executable
	_, err = remote.RunCommandContext(ctx, remoteConfig, host, fmt.Sprintf("chmod +x %s", remoteTalosctlPath))
	if err != nil {
		return fmt.Errorf("failed to make talosctl executable: %w", err)
	}

	// Build command to run DHCP server in background on Proxmox node
	remoteLogPath := fmt.Sprintf("%s/%s", remoteStatePath, dhcpLog)
	remotePidPath := fmt.Sprintf("%s/%s", remoteStatePath, dhcpPid)

	// Build the command to run on Proxmox node
	// We'll use nohup to run it in the background and capture the PID properly
	dhcpCommand := fmt.Sprintf(
		"cd %s && nohup %s dhcpd-launch --state-path %s --addr %s --interface %s --ipxe-next-handler %s > %s 2>&1 & PID=$!; echo $PID > %s; echo $PID",
		remoteStatePath,
		remoteTalosctlPath,
		remoteStatePath,
		strings.Join(gatewayAddrs, ","),
		state.BridgeName,
		clusterReq.IPXEBootScript,
		remoteLogPath,
		remotePidPath,
	)

	// Run command via SSH and capture the PID
	pidOutput, err := remote.RunCommandContext(ctx, remoteConfig, host, dhcpCommand)
	if err != nil {
		return fmt.Errorf("failed to start DHCP server on Proxmox node: %w", err)
	}

	// Wait a moment for the process to start
	time.Sleep(2 * time.Second)

	remotePID := strings.TrimSpace(pidOutput)
	if remotePID == "" {
		// Try to read from PID file as fallback
		pidOutput, err := remote.RunCommandContext(ctx, remoteConfig, host, fmt.Sprintf("cat %s", remotePidPath))
		if err == nil {
			remotePID = strings.TrimSpace(pidOutput)
		}
		if remotePID == "" {
			return fmt.Errorf("DHCP server PID is empty")
		}
	}

	// Store remote PID locally (reusing vm package pattern)
	pidPath := state.GetRelativePath(dhcpPid)
	if err = os.WriteFile(pidPath, []byte(remotePID), os.ModePerm); err != nil {
		return fmt.Errorf("error writing dhcp PID file: %w", err)
	}

	fmt.Printf("started DHCP server on Proxmox node (PID: %s)\n", remotePID)

	return nil
}

// DestroyDHCPd stops the DHCP server running on the Proxmox node.
// Reuses vm.StopProcessByPidfile pattern but adapted for remote execution.
func (p *provisioner) DestroyDHCPd(state *provision.State) error {
	pidPath := state.GetRelativePath(dhcpPid)

	// Read PID from local file
	pidData, err := os.ReadFile(pidPath)
	if err != nil {
		// PID file doesn't exist, DHCP server might not be running
		return nil
	}

	remotePID := strings.TrimSpace(string(pidData))
	if remotePID == "" {
		return nil
	}

	// Parse PID
	pid, err := strconv.Atoi(remotePID)
	if err != nil {
		return fmt.Errorf("invalid PID in file: %w", err)
	}

	// Stop process on remote Proxmox node via SSH
	// Use SIGTERM first, then SIGKILL if needed (following vm.StopProcessByPidfile pattern)
	remoteConfig := &remote.Config{URL: p.config.URL}
	host := remote.ExtractHostFromURL(p.config.URL)
	if host == "" {
		host = p.config.Node
	}

	_, err = remote.RunCommand(remoteConfig, host, fmt.Sprintf("kill -TERM %s", remotePID))
	if err != nil {
		// Process might already be stopped
		return nil
	}

	// Wait for process to stop (with timeout)
	// Note: We can't use vm.StopProcessByPidfile directly since it's for local processes
	// But we can check if the process is still running
	time.Sleep(1 * time.Second)

	// Check if process is still running
	_, err = remote.RunCommand(remoteConfig, host, fmt.Sprintf("kill -0 %s", remotePID))
	if err == nil {
		// Process still running, send SIGKILL
		_, _ = remote.RunCommand(remoteConfig, host, fmt.Sprintf("kill -KILL %s", remotePID)) // Ignore error, process might have stopped
	}

	// Clean up remote state directory
	if state.ClusterInfo.ClusterName != "" {
		remoteStatePath := fmt.Sprintf("/tmp/talos-cluster-%s", state.ClusterInfo.ClusterName)
		_, _ = remote.RunCommand(remoteConfig, host, fmt.Sprintf("rm -rf %s", remoteStatePath)) // Ignore error, cleanup is best effort
	}

	fmt.Printf("DHCP server stopped on Proxmox node (PID: %d)\n", pid)

	return nil
}

