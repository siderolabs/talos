// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package proxmox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cl "github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/provision"
)

// Destroy Talos cluster as set of Proxmox VMs.
//
//nolint:gocyclo
func (p *provisioner) Destroy(ctx context.Context, cluster provision.Cluster, opts ...provision.Option) error {
	options := provision.DefaultOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return err
		}
	}

	stateDirectoryPath, err := cluster.StatePath()
	if err != nil {
		return err
	}

	complete := false
	deleteStateDirectory := func(stateDir string, shouldDelete bool) error {
		if complete || !shouldDelete {
			return nil
		}

		complete = true

		return os.RemoveAll(stateDir)
	}

	defer deleteStateDirectory(stateDirectoryPath, options.DeleteStateOnErr) //nolint:errcheck

	if options.SaveSupportArchivePath != "" {
		fmt.Fprintf(options.LogWriter, "saving support archive to %s\n", options.SaveSupportArchivePath)

		cl.Crashdump(ctx, cluster, options.LogWriter, options.SaveSupportArchivePath)
	}

	fmt.Fprintln(options.LogWriter, "stopping VMs")

	// Get Proxmox node
	node := p.config.Node
	if node == "" {
		var nodes []NodeStatus
		if err := p.client.Get(ctx, "/nodes", &nodes); err != nil {
			return fmt.Errorf("failed to get nodes: %w", err)
		}
		if len(nodes) == 0 {
			return fmt.Errorf("no Proxmox nodes found")
		}
		node = nodes[0].Node
	}

	// Destroy all nodes
	for _, nodeInfo := range cluster.Info().Nodes {
		if err := p.destroyNode(ctx, node, nodeInfo, stateDirectoryPath, &options); err != nil {
			fmt.Fprintf(options.LogWriter, "warning: failed to destroy node %s: %v\n", nodeInfo.Name, err)
		}
	}

	// Destroy extra nodes (PXE nodes)
	for _, nodeInfo := range cluster.Info().ExtraNodes {
		if err := p.destroyNode(ctx, node, nodeInfo, stateDirectoryPath, &options); err != nil {
			fmt.Fprintf(options.LogWriter, "warning: failed to destroy node %s: %v\n", nodeInfo.Name, err)
		}
	}

	state, ok := cluster.(*provision.State)
	if ok {
		fmt.Fprintln(options.LogWriter, "removing dhcpd")

		if err = p.DestroyDHCPd(state); err != nil {
			return fmt.Errorf("error stopping dhcpd: %w", err)
		}
	} else {
		fmt.Fprintln(options.LogWriter, "skipping dhcpd removal (no state available)")
	}

	if options.SaveClusterLogsArchivePath != "" {
		fmt.Fprintf(options.LogWriter, "saving cluster logs archive to %s\n", options.SaveClusterLogsArchivePath)

		cl.SaveClusterLogsArchive(stateDirectoryPath, options.SaveClusterLogsArchivePath)
	}

	fmt.Fprintln(options.LogWriter, "removing state directory")

	return deleteStateDirectory(stateDirectoryPath, true)
}

// destroyNode destroys a single Proxmox VM.
func (p *provisioner) destroyNode(ctx context.Context, defaultNode string, nodeInfo provision.NodeInfo, stateDirectoryPath string, opts *provision.Options) error {
	// Get VM ID from node ID (stored as VM ID)
	vmID, err := strconv.Atoi(nodeInfo.ID)
	if err != nil {
		return fmt.Errorf("invalid VM ID: %w", err)
	}

	// Try to get the actual Proxmox node from state if available
	// This supports multi-node clusters where VMs might be on different nodes
	actualNode := defaultNode
	nodeFile := filepath.Join(stateDirectoryPath, fmt.Sprintf("%s.node", nodeInfo.Name))
	if nodeData, err := os.ReadFile(nodeFile); err == nil {
		actualNode = strings.TrimSpace(string(nodeData))
	}

	fmt.Fprintf(opts.LogWriter, "stopping VM %d (%s) on node %s\n", vmID, nodeInfo.Name, actualNode)

	// Stop VM if running
	var status VMStatus
	if err := p.client.Get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/status/current", actualNode, vmID), &status); err == nil {
		if status.Status == "running" {
			var taskID string
			if err := p.client.Post(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/status/stop", actualNode, vmID), nil, &taskID); err == nil {
				// Wait for stop (with timeout)
				if !p.client.WaitForTask(ctx, actualNode, taskID, 30*time.Second) {
					fmt.Fprintf(opts.LogWriter, "warning: VM stop task timed out, continuing with deletion\n")
				}
			} else {
				fmt.Fprintf(opts.LogWriter, "warning: failed to stop VM: %v, continuing with deletion\n", err)
			}
		}
	} else {
		// VM might not exist, log but continue
		fmt.Fprintf(opts.LogWriter, "warning: could not get VM status: %v, continuing with deletion\n", err)
	}

	fmt.Fprintf(opts.LogWriter, "deleting VM %d (%s) from node %s\n", vmID, nodeInfo.Name, actualNode)

	// Delete VM
	if err := p.client.Delete(ctx, fmt.Sprintf("/nodes/%s/qemu/%d", actualNode, vmID), nil); err != nil {
		// Check if VM doesn't exist (already deleted)
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "404") {
			fmt.Fprintf(opts.LogWriter, "VM %d already deleted\n", vmID)
			return nil
		}
		return fmt.Errorf("failed to delete VM: %w", err)
	}

	return nil
}

