// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package proxmox

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/provision"
)

// Create Talos cluster as a set of Proxmox VMs.
//
//nolint:gocyclo,cyclop
func (p *provisioner) Create(ctx context.Context, request provision.ClusterRequest, opts ...provision.Option) (provision.Cluster, error) {
	options := provision.DefaultOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return nil, err
		}
	}

	statePath := filepath.Join(request.StateDirectory, request.Name)

	fmt.Fprintf(options.LogWriter, "creating state directory in %q\n", statePath)

	state, err := provision.NewState(
		statePath,
		p.Name,
		request.Name,
	)
	if err != nil {
		return nil, err
	}

	// Get Proxmox node if not specified
	node := p.config.Node
	if node == "" {
		fmt.Fprintf(options.LogWriter, "🔍 Auto-detecting Proxmox node (none specified in config)\n")
		var nodes []NodeStatus
		if err := p.client.Get(ctx, "/nodes", &nodes); err != nil {
			fmt.Fprintf(options.LogWriter, "❌ Failed to get node list from Proxmox API\n")
			fmt.Fprintf(options.LogWriter, "❌ Error: %v\n", err)
			fmt.Fprintf(options.LogWriter, "❌ Possible causes:\n")
			fmt.Fprintf(options.LogWriter, "   • Proxmox API authentication failed\n")
			fmt.Fprintf(options.LogWriter, "   • PROXMOX_URL is incorrect\n")
			fmt.Fprintf(options.LogWriter, "   • Network connectivity issues\n")
			return nil, fmt.Errorf("failed to get Proxmox nodes list: %w", err)
		}
		if len(nodes) == 0 {
			fmt.Fprintf(options.LogWriter, "❌ No Proxmox nodes found in the cluster\n")
			fmt.Fprintf(options.LogWriter, "❌ This usually means the Proxmox cluster is not properly configured\n")
			return nil, fmt.Errorf("no Proxmox nodes found - check cluster configuration")
		}
		node = nodes[0].Node
		fmt.Fprintf(options.LogWriter, "✅ Using Proxmox node: %s (auto-detected from %d available nodes)\n", node, len(nodes))
	} else {
		fmt.Fprintf(options.LogWriter, "🔍 Using configured Proxmox node: %s\n", node)
	}

	// Get storage information (handles selection and ISO upload storage finding)
	storage, uploadStorage, err := p.GetStorageInfo(ctx, node)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage info: %w", err)
	}
	fmt.Fprintf(options.LogWriter, "using storage: %s\n", storage)
	if uploadStorage == storage {
		fmt.Fprintf(options.LogWriter, "warning: no storage with ISO support found, using %s (ISO upload may fail)\n", uploadStorage)
	} else {
		fmt.Fprintf(options.LogWriter, "using ISO storage: %s\n", uploadStorage)
	}

	// Validate storage capabilities
	if err := validateStorageCapabilities(ctx, p.client, node, storage, uploadStorage, options.LogWriter); err != nil {
		return nil, fmt.Errorf("storage validation failed: %w", err)
	}

	// Set bridge name for DHCP/TFTP servers (use Proxmox bridge)
	// Note: For Proxmox, the bridge is on the Proxmox node, not on the host machine
	// Since pfSense is the DHCP server on 10.10.10.0/24, we need to run the DHCP server
	// on the Proxmox node itself to reach the VMs on 10.5.0.0/24
	state.BridgeName = p.config.Bridge

	var nodeInfo []provision.NodeInfo

	fmt.Fprintln(options.LogWriter, "creating controlplane nodes")

	if nodeInfo, err = p.createNodes(ctx, state, request, request.Nodes.ControlPlaneNodes(), &options, node, storage, uploadStorage); err != nil {
		return nil, err
	}

	// Start DHCP/TFTP servers for PXE boot if enabled
	// For Proxmox, we run the DHCP server on the Proxmox node via SSH
	// This allows it to access the bridge directly and receive broadcasts from VMs
	// Following QEMU pattern: create control plane nodes first, then start DHCP server
	if request.IPXEBootScript != "" {
		fmt.Fprintln(options.LogWriter, "creating dhcpd")

		if err = p.CreateDHCPd(ctx, state, request); err != nil {
			return nil, fmt.Errorf("error creating dhcpd: %w", err)
		}
	}

	fmt.Fprintln(options.LogWriter, "creating worker nodes")

	var workerNodeInfo []provision.NodeInfo

	if workerNodeInfo, err = p.createNodes(ctx, state, request, request.Nodes.WorkerNodes(), &options, node, storage, uploadStorage); err != nil {
		return nil, err
	}

	var pxeNodeInfo []provision.NodeInfo

	pxeNodes := request.Nodes.PXENodes()
	if len(pxeNodes) > 0 {
		fmt.Fprintln(options.LogWriter, "creating PXE nodes")

		if pxeNodeInfo, err = p.createNodes(ctx, state, request, pxeNodes, &options, node, storage, uploadStorage); err != nil {
			return nil, err
		}
	}

	nodeInfo = append(nodeInfo, workerNodeInfo...)

	lbPort := constants.DefaultControlPlanePort

	if len(request.Network.LoadBalancerPorts) > 0 {
		lbPort = request.Network.LoadBalancerPorts[0]
	}

	state.ClusterInfo = provision.ClusterInfo{
		ClusterName: request.Name,
		Network: provision.NetworkInfo{
			Name:              request.Network.Name,
			CIDRs:             request.Network.CIDRs,
			NoMasqueradeCIDRs: request.Network.NoMasqueradeCIDRs,
			GatewayAddrs:      request.Network.GatewayAddrs,
			MTU:               request.Network.MTU,
		},
		Nodes:              nodeInfo,
		ExtraNodes:         pxeNodeInfo,
		KubernetesEndpoint: p.GetExternalKubernetesControlPlaneEndpoint(request.Network, lbPort),
	}

	err = state.Save()
	if err != nil {
		return nil, err
	}

	return state, nil
}

// validateStorageCapabilities validates that storage pools have the required capabilities.
func validateStorageCapabilities(ctx context.Context, client *Client, node, storage, uploadStorage string, logWriter io.Writer) error {
	// Validate main storage supports images
	var storages []StorageInfo
	if err := client.Get(ctx, fmt.Sprintf("/nodes/%s/storage", node), &storages); err != nil {
		return fmt.Errorf("failed to get storage info: %w", err)
	}

	var mainStorage *StorageInfo
	for i := range storages {
		if storages[i].Storage == storage {
			mainStorage = &storages[i]
			break
		}
	}

	if mainStorage == nil {
		return fmt.Errorf("storage %s not found on node %s", storage, node)
	}

	// Check if main storage supports images (required for VM disks)
	if !storageSupportsContent(*mainStorage, "images") {
		fmt.Fprintf(logWriter, "warning: storage %s may not support images (content: %s)\n", storage, mainStorage.Content)
	}

	// Validate upload storage supports ISO
	var uploadStorageInfo *StorageInfo
	for i := range storages {
		if storages[i].Storage == uploadStorage {
			uploadStorageInfo = &storages[i]
			break
		}
	}

	if uploadStorageInfo == nil {
		return fmt.Errorf("upload storage %s not found on node %s", uploadStorage, node)
	}

	// Check if upload storage supports ISO
	if !storageSupportsContent(*uploadStorageInfo, "iso") {
		fmt.Fprintf(logWriter, "warning: upload storage %s may not support ISO (content: %s)\n", uploadStorage, uploadStorageInfo.Content)
	}

	// Check available space (warn if low)
	if mainStorage.Total > 0 {
		usedPercent := float64(mainStorage.Used) / float64(mainStorage.Total) * 100
		if usedPercent > 90 {
			fmt.Fprintf(logWriter, "warning: storage %s is %0.1f%% full (%d/%d bytes)\n", storage, usedPercent, mainStorage.Used, mainStorage.Total)
		}
	}

	return nil
}

// storageSupportsContent checks if a storage pool supports a specific content type.
// Content types include: "iso", "images", "vztmpl", "backup", etc.
func storageSupportsContent(storage StorageInfo, contentType string) bool {
	content := strings.Split(storage.Content, ",")
	for _, c := range content {
		if strings.TrimSpace(c) == contentType {
			return true
		}
	}
	return false
}

// findStorageByContent finds the first storage pool that supports the specified content type.
// Returns empty string if no storage supports the content type.
func findStorageByContent(storages []StorageInfo, contentType string) string {
	for _, s := range storages {
		if storageSupportsContent(s, contentType) {
			return s.Storage
		}
	}
	return ""
}

// findUploadStorage finds a suitable storage pool for ISO uploads.
// Tries "iso" content type first, then falls back to "images", then to the provided default storage.
func findUploadStorage(storages []StorageInfo, defaultStorage string) string {
	// Try to find storage that supports ISO uploads
	if uploadStorage := findStorageByContent(storages, "iso"); uploadStorage != "" {
		return uploadStorage
	}

	// Fallback to storage that supports images
	if uploadStorage := findStorageByContent(storages, "images"); uploadStorage != "" {
		return uploadStorage
	}

	// Final fallback to default storage
	return defaultStorage
}

// selectBestStorage selects the best storage pool for VM disks.
// Excludes small local storages (local) and prefers larger storage pools.
func selectBestStorage(storages []StorageInfo, logWriter io.Writer) string {
	// Storage names to exclude (typically small local storages)
	// Note: local-lvm can be large (often 500GB+), so we don't exclude it
	// We'll prefer larger storages based on available space instead
	excludedNames := map[string]bool{
		"local": true, // Small local storage (typically <100GB)
	}

	// Storage types to prefer (in order of preference)
	preferredTypes := []string{"zfspool", "lvmthin", "lvm", "dir", "nfs", "cifs"}

	var candidates []StorageInfo

	// Filter out excluded storages and storages without images support
	for _, s := range storages {
		// Skip excluded storages
		if excludedNames[s.Storage] {
			fmt.Fprintf(logWriter, "skipping storage %s (type: %s, size: %d/%d bytes) - excluded\n", s.Storage, s.Type, s.Used, s.Total)
			continue
		}

		// Check if storage supports images (required for VM disks)
		if !storageSupportsContent(s, "images") {
			fmt.Fprintf(logWriter, "skipping storage %s (type: %s) - does not support images\n", s.Storage, s.Type)
			continue
		}

		// Check if storage has reasonable size (at least 10GB free or unknown size)
		if s.Total > 0 {
			free := s.Total - s.Used
			minFree := uint64(10 * 1024 * 1024 * 1024) // 10GB minimum
			if free < minFree {
				fmt.Fprintf(logWriter, "skipping storage %s (type: %s) - insufficient space: %d bytes free (need at least %d)\n", s.Storage, s.Type, free, minFree)
				continue
			}
		}

		candidates = append(candidates, s)
	}

	if len(candidates) == 0 {
		return ""
	}

	// Sort by preference: preferred types first, then by available space
	var bestStorage *StorageInfo
	var bestScore int
	var bestFreeSpace uint64

	for i := range candidates {
		s := &candidates[i]
		score := 0

		// Score by type preference
		for j, preferredType := range preferredTypes {
			if s.Type == preferredType {
				score = len(preferredTypes) - j // Higher score for preferred types
				break
			}
		}

		// Calculate free space
		freeSpace := uint64(0)
		if s.Total > s.Used {
			freeSpace = s.Total - s.Used
		}

		// Prefer storage with more free space (if types are equal)
		if bestStorage == nil || score > bestScore || (score == bestScore && freeSpace > bestFreeSpace) {
			bestStorage = s
			bestScore = score
			bestFreeSpace = freeSpace
		}
	}

	if bestStorage != nil {
		fmt.Fprintf(logWriter, "selected storage %s (type: %s, free: %d bytes, total: %d bytes)\n", bestStorage.Storage, bestStorage.Type, bestFreeSpace, bestStorage.Total)
		return bestStorage.Storage
	}

	return ""
}

