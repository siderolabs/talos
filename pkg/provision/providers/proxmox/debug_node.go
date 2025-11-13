// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package proxmox

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/siderolabs/talos/pkg/provision"
)

// DebugCreateNode creates a single VM step by step with detailed logging.
// This is a debugging function to understand and fix boot issues.
func (p *provisioner) DebugCreateNode(ctx context.Context, state *provision.State, clusterReq provision.ClusterRequest, nodeReq provision.NodeRequest, opts *provision.Options) error {
	logWriter := opts.LogWriter
	if logWriter == nil {
		logWriter = os.Stderr
	}

	fmt.Fprintf(logWriter, "\n")
	fmt.Fprintf(logWriter, "═══════════════════════════════════════════════════════════════\n")
	fmt.Fprintf(logWriter, "🔧 DEBUG: STEP-BY-STEP VM CREATION FOR %s\n", nodeReq.Name)
	fmt.Fprintf(logWriter, "═══════════════════════════════════════════════════════════════\n")
	fmt.Fprintf(logWriter, "\n")

	// Step 1: Get Proxmox node
	fmt.Fprintf(logWriter, "📋 STEP 1: Getting Proxmox node...\n")
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
	fmt.Fprintf(logWriter, "   ✅ Using node: %s\n", node)
	fmt.Fprintf(logWriter, "\n")

	// Step 2: Get storage
	fmt.Fprintf(logWriter, "📋 STEP 2: Getting storage pools...\n")
	storage := p.config.Storage
	var storages []StorageInfo
	if storage == "" {
		if err := p.client.Get(ctx, fmt.Sprintf("/nodes/%s/storage", node), &storages); err != nil {
			return fmt.Errorf("failed to get storage: %w", err)
		}
		// Find storage with 'images' content
		for _, s := range storages {
			if storageSupportsContent(s, "images") {
				storage = s.Storage
				break
			}
		}
		if storage == "" {
			return fmt.Errorf("no storage with 'images' content found")
		}
	}

	// Find ISO storage
	var uploadStorage string
	if err := p.client.Get(ctx, fmt.Sprintf("/nodes/%s/storage", node), &storages); err != nil {
		return fmt.Errorf("failed to get storage: %w", err)
	}
	uploadStorage = findUploadStorage(storages, storage)

	fmt.Fprintf(logWriter, "   ✅ VM disk storage: %s\n", storage)
	fmt.Fprintf(logWriter, "   ✅ ISO upload storage: %s\n", uploadStorage)
	fmt.Fprintf(logWriter, "\n")

	// Step 3: Find available VM ID
	fmt.Fprintf(logWriter, "📋 STEP 3: Finding available VM ID...\n")
	vmID, err := p.findAvailableVMID(ctx, node)
	if err != nil {
		return fmt.Errorf("failed to find available VM ID: %w", err)
	}
	fmt.Fprintf(logWriter, "   ✅ Found available VM ID: %d\n", vmID)
	fmt.Fprintf(logWriter, "\n")

	// Step 4: Create cloud-init ISO
	fmt.Fprintf(logWriter, "📋 STEP 4: Creating cloud-init ISO...\n")
	isoFilename := fmt.Sprintf("%s-cloud-init.iso", nodeReq.Name)

	// Get Talos configuration
	var nodeConfig string
	if !nodeReq.SkipInjectingConfig {
		nodeConfig, err = nodeReq.Config.EncodeString()
		if err != nil {
			return fmt.Errorf("failed to encode config: %w", err)
		}
	}

	isoPath, err := p.createCloudInitISO(state, nodeReq.Name, nodeConfig, nodeReq, clusterReq.Network)
	if err != nil {
		return fmt.Errorf("failed to create cloud-init ISO: %w", err)
	}
	fmt.Fprintf(logWriter, "   ✅ Cloud-init ISO created: %s\n", isoPath)
	fmt.Fprintf(logWriter, "\n")

	// Step 5: Upload cloud-init ISO
	fmt.Fprintf(logWriter, "📋 STEP 5: Uploading cloud-init ISO to Proxmox storage...\n")
	isoFile, err := os.Open(isoPath)
	if err != nil {
		return fmt.Errorf("failed to open cloud-init ISO: %w", err)
	}
	defer isoFile.Close()

	taskID, err := p.client.UploadFile(ctx, node, uploadStorage, isoFilename, isoFile)
	if err != nil {
		return fmt.Errorf("failed to upload cloud-init ISO: %w", err)
	}
	fmt.Fprintf(logWriter, "   ✅ Upload task started: %s\n", taskID)
	fmt.Fprintf(logWriter, "   ⏳ Waiting for upload to complete...\n")

	if !p.client.WaitForTask(ctx, node, taskID, 5*time.Minute) {
		return fmt.Errorf("cloud-init ISO upload task failed or timed out")
	}
	fmt.Fprintf(logWriter, "   ✅ Cloud-init ISO uploaded successfully\n")
	fmt.Fprintf(logWriter, "\n")

	// Step 6: Upload Talos ISO (if provided)
	var talosISOFilename string
	if clusterReq.ISOPath != "" {
		fmt.Fprintf(logWriter, "📋 STEP 6: Uploading Talos ISO to Proxmox storage...\n")
		talosISOFilename = filepath.Base(clusterReq.ISOPath)

		// Check if ISO already exists in storage (avoid re-uploading)
		talosISOFile, err := os.Open(clusterReq.ISOPath)
		if err != nil {
			return fmt.Errorf("failed to open Talos ISO: %w", err)
		}
		defer talosISOFile.Close()

		fmt.Fprintf(logWriter, "   📤 Uploading: %s\n", talosISOFilename)
		fmt.Fprintf(logWriter, "   ⏳ This may take a few minutes...\n")
		taskID, err := p.client.UploadFile(ctx, node, uploadStorage, talosISOFilename, talosISOFile)
		if err != nil {
			return fmt.Errorf("failed to upload Talos ISO: %w", err)
		}
		fmt.Fprintf(logWriter, "   ✅ Upload task started: %s\n", taskID)
		fmt.Fprintf(logWriter, "   ⏳ Waiting for upload to complete...\n")

		if !p.client.WaitForTask(ctx, node, taskID, 10*time.Minute) {
			return fmt.Errorf("Talos ISO upload task failed or timed out")
		}
		fmt.Fprintf(logWriter, "   ✅ Talos ISO uploaded successfully\n")
		fmt.Fprintf(logWriter, "\n")
	} else {
		fmt.Fprintf(logWriter, "📋 STEP 6: Skipping Talos ISO upload (not provided)\n")
		fmt.Fprintf(logWriter, "\n")
	}

	// Step 7: Prepare VM configuration
	fmt.Fprintf(logWriter, "📋 STEP 7: Preparing VM configuration...\n")

	// Generate MAC address
	macAddress := p.generateMACAddress()
	fmt.Fprintf(logWriter, "   ✅ Generated MAC address: %s\n", macAddress)

	// Calculate disk size
	diskSizeGB := nodeReq.Disks[0].Size / 1024 / 1024 / 1024
	if diskSizeGB < 10 {
		diskSizeGB = 10 // Minimum 10GB
	}
	fmt.Fprintf(logWriter, "   ✅ Disk size: %d GB\n", diskSizeGB)

	// Calculate CPU and memory
	vcpuCount := int64(nodeReq.NanoCPUs / 1000 / 1000 / 1000)
	if vcpuCount < 2 {
		vcpuCount = 2
	}
	memSize := nodeReq.Memory / 1024 / 1024
	fmt.Fprintf(logWriter, "   ✅ CPU cores: %d\n", vcpuCount)
	fmt.Fprintf(logWriter, "   ✅ Memory: %d MB\n", memSize)
	fmt.Fprintf(logWriter, "\n")

	// Step 8: Build VM creation parameters
	fmt.Fprintf(logWriter, "📋 STEP 8: Building VM creation parameters...\n")

	params := url.Values{}
	params.Set("vmid", strconv.Itoa(vmID))
	params.Set("name", nodeReq.Name)
	params.Set("cores", strconv.FormatInt(vcpuCount, 10))
	params.Set("memory", strconv.FormatInt(memSize, 10))
	params.Set("net0", fmt.Sprintf("virtio=%s,bridge=%s", macAddress, p.config.Bridge))
	params.Set("virtio0", fmt.Sprintf("%s:%d,format=raw,iothread=1", storage, diskSizeGB))
	params.Set("ostype", "l26")
	params.Set("machine", "q35")
	params.Set("bios", "ovmf")
	params.Set("efidisk0", fmt.Sprintf("%s:1,format=raw", storage))
	params.Set("cpu", "host")
	params.Set("balloon", "0")
	params.Set("rng0", "source=/dev/urandom")

	fmt.Fprintf(logWriter, "   📝 Basic VM parameters:\n")
	fmt.Fprintf(logWriter, "      - vmid: %d\n", vmID)
	fmt.Fprintf(logWriter, "      - name: %s\n", nodeReq.Name)
	fmt.Fprintf(logWriter, "      - cores: %d\n", vcpuCount)
	fmt.Fprintf(logWriter, "      - memory: %d MB\n", memSize)
	fmt.Fprintf(logWriter, "      - machine: q35\n")
	fmt.Fprintf(logWriter, "      - bios: ovmf (UEFI)\n")
	fmt.Fprintf(logWriter, "      - efidisk0: %s:1,format=raw\n", storage)
	fmt.Fprintf(logWriter, "\n")

	// Step 9: Attach Talos ISO for booting
	if talosISOFilename != "" {
		fmt.Fprintf(logWriter, "📋 STEP 9: Attaching Talos ISO for booting...\n")
		talosISOVolID := fmt.Sprintf("%s:iso/%s", uploadStorage, talosISOFilename)
		// Use SATA for CDROM - better UEFI detection
		params.Set("sata0", fmt.Sprintf("%s,media=cdrom", talosISOVolID))
		params.Set("boot", "order=sata0;virtio0")
		params.Set("bootdisk", "sata0")

		fmt.Fprintf(logWriter, "   ✅ Talos ISO attached to sata0: %s\n", talosISOVolID)
		fmt.Fprintf(logWriter, "   ✅ Boot order set to: sata0;virtio0\n")
		fmt.Fprintf(logWriter, "   ✅ Bootdisk set to: sata0\n")
		fmt.Fprintf(logWriter, "\n")
	} else {
		fmt.Fprintf(logWriter, "📋 STEP 9: Skipping Talos ISO attachment (not provided)\n")
		fmt.Fprintf(logWriter, "\n")
	}

	// Step 10: Attach cloud-init ISO
	fmt.Fprintf(logWriter, "📋 STEP 10: Attaching cloud-init ISO...\n")
	isoVolID := fmt.Sprintf("%s:iso/%s", uploadStorage, isoFilename)
	// Use SATA for cloud-init ISO as well for consistency
	params.Set("sata2", fmt.Sprintf("%s,media=cdrom", isoVolID))
	fmt.Fprintf(logWriter, "   ✅ Cloud-init ISO attached to sata2: %s\n", isoVolID)
	fmt.Fprintf(logWriter, "\n")

	// Step 11: Display full configuration
	fmt.Fprintf(logWriter, "📋 STEP 11: Full VM configuration:\n")
	fmt.Fprintf(logWriter, "   ┌─────────────────────────────────────────────────────────┐\n")
	for key, values := range params {
		if len(values) > 0 {
			fmt.Fprintf(logWriter, "   │ %-20s: %-40s │\n", key, values[0])
		}
	}
	fmt.Fprintf(logWriter, "   └─────────────────────────────────────────────────────────┘\n")
	fmt.Fprintf(logWriter, "\n")

	// Step 12: Create VM
	fmt.Fprintf(logWriter, "📋 STEP 12: Creating VM on Proxmox...\n")
	var createTaskID string
	if err := p.client.Post(ctx, fmt.Sprintf("/nodes/%s/qemu", node), params, &createTaskID); err != nil {
		return fmt.Errorf("failed to create VM: %w", err)
	}
	fmt.Fprintf(logWriter, "   ✅ VM creation task started: %s\n", createTaskID)
	fmt.Fprintf(logWriter, "   ⏳ Waiting for VM creation to complete...\n")

	if !p.client.WaitForTask(ctx, node, createTaskID, 2*time.Minute) {
		var task TaskStatus
		if err := p.client.Get(ctx, fmt.Sprintf("/nodes/%s/tasks/%s/status", node, createTaskID), &task); err == nil {
			return fmt.Errorf("VM creation task failed: status=%s, exitstatus=%s", task.Status, task.ExitStatus)
		}
		return fmt.Errorf("VM creation task failed or timed out")
	}
	fmt.Fprintf(logWriter, "   ✅ VM %d created successfully\n", vmID)
	fmt.Fprintf(logWriter, "\n")

	// Step 13: Verify VM configuration
	fmt.Fprintf(logWriter, "📋 STEP 13: Verifying VM configuration...\n")
	var vmConfig map[string]interface{}
	if err := p.client.Get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/config", node, vmID), &vmConfig); err != nil {
		fmt.Fprintf(logWriter, "   ⚠️  Warning: Failed to get VM config: %v\n", err)
	} else {
		fmt.Fprintf(logWriter, "   ✅ VM configuration verified:\n")
		if boot, ok := vmConfig["boot"].(string); ok {
			fmt.Fprintf(logWriter, "      - boot: %s\n", boot)
		}
		if bootdisk, ok := vmConfig["bootdisk"].(string); ok {
			fmt.Fprintf(logWriter, "      - bootdisk: %s\n", bootdisk)
		}
		if bios, ok := vmConfig["bios"].(string); ok {
			fmt.Fprintf(logWriter, "      - bios: %s\n", bios)
		}
		if sata0, ok := vmConfig["sata0"].(string); ok {
			fmt.Fprintf(logWriter, "      - sata0 (Talos ISO): %s\n", sata0)
		}
		if sata2, ok := vmConfig["sata2"].(string); ok {
			fmt.Fprintf(logWriter, "      - sata2 (cloud-init ISO): %s\n", sata2)
		}
	}
	fmt.Fprintf(logWriter, "\n")

	// Step 14: Start VM
	fmt.Fprintf(logWriter, "📋 STEP 14: Starting VM...\n")
	var startTaskID string
	if err := p.client.Post(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/status/start", node, vmID), nil, &startTaskID); err != nil {
		return fmt.Errorf("failed to start VM: %w", err)
	}
	fmt.Fprintf(logWriter, "   ✅ VM start task started: %s\n", startTaskID)
	fmt.Fprintf(logWriter, "   ⏳ Waiting for VM to start...\n")

	if !p.client.WaitForTask(ctx, node, startTaskID, 30*time.Second) {
		var task TaskStatus
		if err := p.client.Get(ctx, fmt.Sprintf("/nodes/%s/tasks/%s/status", node, startTaskID), &task); err == nil {
			fmt.Fprintf(logWriter, "   ⚠️  Warning: VM start task status: %s, exitstatus: %s\n", task.Status, task.ExitStatus)
		} else {
			fmt.Fprintf(logWriter, "   ⚠️  Warning: VM start task timed out, but VM may still be starting\n")
		}
	} else {
		fmt.Fprintf(logWriter, "   ✅ VM started successfully\n")
	}
	fmt.Fprintf(logWriter, "\n")

	// Step 15: Wait and check VM status
	fmt.Fprintf(logWriter, "📋 STEP 15: Checking VM status...\n")
	time.Sleep(5 * time.Second) // Wait a bit for VM to initialize

	var vmStatus map[string]interface{}
	if err := p.client.Get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/status/current", node, vmID), &vmStatus); err != nil {
		fmt.Fprintf(logWriter, "   ⚠️  Warning: Failed to get VM status: %v\n", err)
	} else {
		if status, ok := vmStatus["status"].(string); ok {
			fmt.Fprintf(logWriter, "   ✅ VM status: %s\n", status)
		}
		if qmpstatus, ok := vmStatus["qmpstatus"].(string); ok {
			fmt.Fprintf(logWriter, "   ✅ QMP status: %s\n", qmpstatus)
		}
	}
	fmt.Fprintf(logWriter, "\n")

	// Step 16: Summary and next steps
	fmt.Fprintf(logWriter, "═══════════════════════════════════════════════════════════════\n")
	fmt.Fprintf(logWriter, "✅ VM CREATION COMPLETE\n")
	fmt.Fprintf(logWriter, "═══════════════════════════════════════════════════════════════\n")
	fmt.Fprintf(logWriter, "\n")
	fmt.Fprintf(logWriter, "📊 SUMMARY:\n")
	fmt.Fprintf(logWriter, "   - VM ID: %d\n", vmID)
	fmt.Fprintf(logWriter, "   - Name: %s\n", nodeReq.Name)
	fmt.Fprintf(logWriter, "   - Node: %s\n", node)
	fmt.Fprintf(logWriter, "   - Talos ISO: %s\n", talosISOFilename)
	fmt.Fprintf(logWriter, "   - Cloud-init ISO: %s\n", isoFilename)
	fmt.Fprintf(logWriter, "\n")
	fmt.Fprintf(logWriter, "🔍 NEXT STEPS:\n")
	fmt.Fprintf(logWriter, "   1. Check VM console in Proxmox web UI\n")
	fmt.Fprintf(logWriter, "   2. If booting to UEFI shell:\n")
	fmt.Fprintf(logWriter, "      a. Press ESC during boot to enter UEFI menu\n")
	fmt.Fprintf(logWriter, "      b. Navigate to Boot Maintenance Manager > Boot Options\n")
	fmt.Fprintf(logWriter, "      c. Add Boot Option > Select CDROM with Talos ISO\n")
	fmt.Fprintf(logWriter, "      d. Select EFI/BOOT/BOOTX64.EFI\n")
	fmt.Fprintf(logWriter, "      e. Set as first boot option\n")
	fmt.Fprintf(logWriter, "   3. If Secure Boot is blocking:\n")
	fmt.Fprintf(logWriter, "      a. Navigate to Device Manager > Secure Boot Configuration\n")
	fmt.Fprintf(logWriter, "      b. Uncheck 'Attempt SecureBoot'\n")
	fmt.Fprintf(logWriter, "   4. Wait for Talos to boot and check API connectivity\n")
	fmt.Fprintf(logWriter, "\n")
	fmt.Fprintf(logWriter, "💡 UEFI BOOT BEHAVIOR (from Talos/Proxmox/QEMU documentation):\n")
	fmt.Fprintf(logWriter, "   - Talos ISO contains EFI/BOOT/BOOTX64.EFI (standard UEFI bootloader)\n")
	fmt.Fprintf(logWriter, "   - With OVMF (UEFI), boot order is managed by UEFI firmware\n")
	fmt.Fprintf(logWriter, "   - Proxmox 'boot' parameter mainly affects legacy BIOS (SeaBIOS)\n")
	fmt.Fprintf(logWriter, "   - UEFI should auto-detect EFI/BOOT/BOOTX64.EFI on CDROM\n")
	fmt.Fprintf(logWriter, "   - If auto-detection fails, manual UEFI boot entry configuration is needed\n")
	fmt.Fprintf(logWriter, "   - Secure Boot may block unsigned bootloaders (disable if needed)\n")
	fmt.Fprintf(logWriter, "\n")
	fmt.Fprintf(logWriter, "📚 REFERENCES:\n")
	fmt.Fprintf(logWriter, "   - Talos Proxmox Guide: https://www.talos.dev/v1.9/talos-guides/install/virtualized-platforms/proxmox/\n")
	fmt.Fprintf(logWriter, "   - Proxmox OVMF/UEFI: https://pve.proxmox.com/wiki/OVMF/UEFI_Boot_Entries\n")
	fmt.Fprintf(logWriter, "\n")

	return nil
}

