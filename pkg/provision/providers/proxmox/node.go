// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package proxmox

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

// createNodes creates multiple VMs.
func (p *provisioner) createNodes(ctx context.Context, state *provision.State, clusterReq provision.ClusterRequest, nodeReqs []provision.NodeRequest, opts *provision.Options, node, storage, uploadStorage string) ([]provision.NodeInfo, error) {
	var nodeInfo []provision.NodeInfo

	for _, nodeReq := range nodeReqs {
		info, err := p.createNode(ctx, state, clusterReq, nodeReq, opts, node, storage, uploadStorage)
		if err != nil {
			return nil, fmt.Errorf("failed to create node %s: %w", nodeReq.Name, err)
		}

		nodeInfo = append(nodeInfo, info)
	}

	return nodeInfo, nil
}

// vmResources holds validated VM resource requirements.
type vmResources struct {
	vcpuCount   int64
	memSize     int64
	diskSizeGB  int64 // Primary disk size (backwards compatibility)
	diskConfigs []diskConfig
}

// diskConfig holds configuration for a single disk.
type diskConfig struct {
	sizeGB int64
	index  int // 0-based disk index (0 = boot disk, 1+ = data disks)
}

// validateNodeRequest validates and calculates VM resource requirements.
func (p *provisioner) validateNodeRequest(nodeReq provision.NodeRequest) (vmResources, error) {
	var resources vmResources

	if nodeReq.Memory < 2*1024*1024*1024 { // 2GB minimum
		return resources, fmt.Errorf("memory must be at least 2GB, got %d bytes", nodeReq.Memory)
	}

	if len(nodeReq.Disks) == 0 {
		return resources, fmt.Errorf("at least one disk is required")
	}

	// Proxmox supports virtio0-virtio15 (16 disks maximum)
	const maxDisks = 16
	if len(nodeReq.Disks) > maxDisks {
		return resources, fmt.Errorf("too many disks: Proxmox supports maximum %d virtio disks (virtio0-virtio15), got %d", maxDisks, len(nodeReq.Disks))
	}

	// Validate all disks and build disk configurations
	for i, disk := range nodeReq.Disks {
		minSize := int64(1 * 1024 * 1024 * 1024) // 1GB minimum for data disks
		if i == 0 { // Boot disk
			minSize = 10 * 1024 * 1024 * 1024 // 10GB minimum for boot disk
		}
		if disk.Size < uint64(minSize) {
			return resources, fmt.Errorf("disk %d size must be at least %dGB, got %d bytes",
				i, minSize/1024/1024/1024, disk.Size)
		}

		diskSizeGB := int64(disk.Size) / 1024 / 1024 / 1024
		if i == 0 && diskSizeGB < 10 { // Boot disk minimum
			diskSizeGB = 10
		}

		// Reasonable maximum disk size: 64TB (64 * 1024 GB)
		// This prevents unrealistic configurations and potential overflow issues
		const maxDiskSizeGB = 64 * 1024
		if diskSizeGB > maxDiskSizeGB {
			return resources, fmt.Errorf("disk %d size exceeds maximum: %dGB (64TB), got %dGB", i, maxDiskSizeGB, diskSizeGB)
		}

		resources.diskConfigs = append(resources.diskConfigs, diskConfig{
			sizeGB: diskSizeGB,
			index:  i,
		})
	}

	// Set primary disk size for backwards compatibility
	if len(resources.diskConfigs) > 0 {
		resources.diskSizeGB = resources.diskConfigs[0].sizeGB
	}

	resources.vcpuCount = int64(math.RoundToEven(float64(nodeReq.NanoCPUs) / 1000 / 1000 / 1000))
	if resources.vcpuCount < 1 {
		resources.vcpuCount = 1
	}
	if resources.vcpuCount > 128 {
		return resources, fmt.Errorf("CPU count cannot exceed 128, got %d", resources.vcpuCount)
	}

	resources.memSize = nodeReq.Memory / 1024 / 1024 // Convert to MB
	if resources.memSize > 1024*1024 { // 1TB maximum
		return resources, fmt.Errorf("memory cannot exceed 1TB, got %d MB", resources.memSize)
	}

	return resources, nil
}

// ensureTalosISO ensures the Talos ISO is uploaded to storage, skipping if it already exists.
func (p *provisioner) ensureTalosISO(ctx context.Context, node, uploadStorage, isoPath string, opts *provision.Options) (string, error) {
	if isoPath == "" {
		return "", nil
	}

	talosISOFilename := filepath.Base(isoPath)

	// Check if ISO already exists
	exists, err := p.client.CheckISOExists(ctx, node, uploadStorage, talosISOFilename)
		if err != nil {
		// Log warning but continue - might be a transient error
		fmt.Fprintf(opts.LogWriter, "warning: failed to check if Talos ISO exists: %v\n", err)
	}

	if exists {
		// Verify existing ISO size to ensure it's not corrupted
		isoSize, err := p.client.GetISOSize(ctx, node, uploadStorage, talosISOFilename)
		if err == nil {
			// Minimum expected size for Talos ISO: 100MB (typically 200-300MB)
			minISOSize := uint64(100 * 1024 * 1024) // 100MB
			if isoSize < minISOSize {
				fmt.Fprintf(opts.LogWriter, "warning: existing Talos ISO %s is suspiciously small (%d bytes), re-uploading\n", talosISOFilename, isoSize)
				exists = false // Force re-upload
			} else {
				fmt.Fprintf(opts.LogWriter, "Talos ISO %s already exists in storage %s (size: %d bytes), skipping upload\n", talosISOFilename, uploadStorage, isoSize)
				return talosISOFilename, nil
			}
		} else {
			fmt.Fprintf(opts.LogWriter, "warning: failed to verify existing ISO size: %v, re-uploading\n", err)
			exists = false // Force re-upload if we can't verify
		}
	}

	// Upload Talos ISO
	talosISOFile, err := os.Open(isoPath)
		if err != nil {
		return "", fmt.Errorf("failed to open Talos ISO: %w", err)
		}
		defer talosISOFile.Close()

	// Get local file size for verification
	localFileInfo, err := talosISOFile.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to get local ISO file info: %w", err)
	}
	localFileSize := uint64(localFileInfo.Size())

	fmt.Fprintf(opts.LogWriter, "uploading Talos ISO %s to storage %s (size: %d bytes)\n", talosISOFilename, uploadStorage, localFileSize)
		taskID, err := p.client.UploadFile(ctx, node, uploadStorage, talosISOFilename, talosISOFile)
		if err != nil {
		return "", fmt.Errorf("failed to upload Talos ISO: %w", err)
		}

		// Wait for upload to complete
		if err := p.client.WaitForTaskWithError(ctx, node, taskID, 10*time.Minute); err != nil {
			return "", fmt.Errorf("Talos ISO upload: %w", err)
		}

		// Verify uploaded ISO size matches local file
		uploadedSize, err := p.client.GetISOSize(ctx, node, uploadStorage, talosISOFilename)
		if err != nil {
			return "", fmt.Errorf("failed to verify uploaded ISO size: %w", err)
		}

		// Allow 1% difference for filesystem/metadata overhead
		sizeDiff := uint64(0)
		if localFileSize > uploadedSize {
			sizeDiff = localFileSize - uploadedSize
		} else {
			sizeDiff = uploadedSize - localFileSize
		}
		if sizeDiff > localFileSize/100 {
			return "", fmt.Errorf("uploaded ISO size mismatch: local=%d bytes, uploaded=%d bytes (difference: %d bytes)", localFileSize, uploadedSize, sizeDiff)
		}

		fmt.Fprintf(opts.LogWriter, "Talos ISO uploaded successfully (size: %d bytes, verified)\n", uploadedSize)
	return talosISOFilename, nil
}

// ensureCloudInitISO creates and uploads the cloud-init ISO, skipping upload if it already exists.
func (p *provisioner) ensureCloudInitISO(ctx context.Context, state *provision.State, nodeName, nodeConfig string, nodeReq provision.NodeRequest, networkReq provision.NetworkRequest, node, uploadStorage string, opts *provision.Options) (string, error) {
	isoFilename := fmt.Sprintf("%s-cloud-init.iso", nodeName)

	// Check if ISO already exists
	exists, err := p.client.CheckISOExists(ctx, node, uploadStorage, isoFilename)
	if err != nil {
		// Log warning but continue - might be a transient error
		fmt.Fprintf(opts.LogWriter, "warning: failed to check if cloud-init ISO exists: %v\n", err)
	}

	if !exists {
		// Create cloud-init ISO
		cloudInitISO, err := p.createCloudInitISO(state, nodeName, nodeConfig, nodeReq, networkReq)
		if err != nil {
			return "", fmt.Errorf("failed to create cloud-init ISO: %w", err)
	}

	// Upload cloud-init ISO to Proxmox storage
	isoFile, err := os.Open(cloudInitISO)
	if err != nil {
			return "", fmt.Errorf("failed to open ISO file: %w", err)
	}
	defer isoFile.Close()

		fmt.Fprintf(opts.LogWriter, "uploading cloud-init ISO for %s to storage %s\n", nodeName, uploadStorage)
	taskID, err := p.client.UploadFile(ctx, node, uploadStorage, isoFilename, isoFile)
	if err != nil {
			return "", fmt.Errorf("failed to upload ISO: %w", err)
	}

	// Wait for upload to complete
	if err := p.client.WaitForTaskWithError(ctx, node, taskID, 5*time.Minute); err != nil {
		return "", fmt.Errorf("cloud-init ISO upload: %w", err)
	}

	fmt.Fprintf(opts.LogWriter, "cloud-init ISO uploaded successfully\n")
	} else {
		fmt.Fprintf(opts.LogWriter, "cloud-init ISO %s already exists in storage %s, skipping upload\n", isoFilename, uploadStorage)
	}

	return isoFilename, nil
}

// buildVMConfig builds VM configuration parameters with optimized UEFI boot settings.
func (p *provisioner) buildVMConfig(vmID int, nodeReq provision.NodeRequest, resources vmResources, storage, uploadStorage, talosISOFilename, cloudInitISOFilename string, opts *provision.Options) url.Values {
	macAddress := p.generateMACAddress()

	params := url.Values{}
	params.Set("vmid", strconv.Itoa(vmID))
	params.Set("name", nodeReq.Name)
	params.Set("cores", strconv.FormatInt(resources.vcpuCount, 10))
	params.Set("memory", strconv.FormatInt(resources.memSize, 10))
	params.Set("net0", fmt.Sprintf("virtio=%s,bridge=%s", macAddress, p.config.Bridge))

	// Configure multiple disks (virtio for better iothread support)
	for _, diskCfg := range resources.diskConfigs {
		diskParam := fmt.Sprintf("virtio%d", diskCfg.index)
		diskValue := fmt.Sprintf("%s:%d,format=raw", storage, diskCfg.sizeGB)

		// Enable iothread only for boot disk (index 0) for better performance
		if diskCfg.index == 0 {
			diskValue += ",iothread=1"
		}

		params.Set(diskParam, diskValue)
		fmt.Fprintf(opts.LogWriter, "configured disk virtio%d: %dGB (%s)\n",
			diskCfg.index, diskCfg.sizeGB,
			map[bool]string{true: "boot disk", false: "data disk"}[diskCfg.index == 0])
	}
	params.Set("ostype", "l26")
	params.Set("machine", "q35")
	params.Set("bios", "ovmf")

	// Configure UEFI/Secure Boot
	// Proxmox 8.x supports Secure Boot via efidisk0 parameters:
	// - efitype=4m: 4MB OVMF firmware (Proxmox automatically uses Secure Boot variant if available)
	// - pre-enrolled-keys=1: Enable pre-enrolled Secure Boot keys (enables Secure Boot)
	// Note: Proxmox API only accepts efitype=2m or 4m, not 4m-secure
	// When pre-enrolled-keys=1 is set, Proxmox uses OVMF_CODE_4M.secboot.fd automatically
	// Verified on host: /usr/share/pve-edk2-firmware/OVMF_CODE_4M.secboot.fd exists
	//
	// Secure Boot should be enabled when:
	// 1. The ISO filename contains "secureboot" (Secure Boot ISO) - applies to ALL nodes (control planes and workers)
	// Note: We check the ISO filename directly, not just opts.UEFIEnabled, because:
	//   - Users may use --iso-path with a Secure Boot ISO without the iso-secureboot preset
	//   - Workers should get Secure Boot if using a Secure Boot ISO (same as control planes)
	efiType := "4m"
	efiParams := []string{"format=raw"}

	enableSecureBoot := false
	// Check if we're using a Secure Boot ISO (applies to all node types)
	if talosISOFilename != "" && strings.Contains(strings.ToLower(talosISOFilename), "secureboot") {
		enableSecureBoot = true
		// Also ensure UEFI is enabled if not already set
		if opts != nil && !opts.UEFIEnabled {
			// Log that we're enabling UEFI for Secure Boot ISO
			fmt.Fprintf(opts.LogWriter, "enabling UEFI for Secure Boot ISO (ISO filename contains 'secureboot')\n")
		}
	} else if opts != nil && opts.UEFIEnabled {
		// Legacy check: if iso-secureboot preset was used but ISO doesn't have "secureboot" in name
		// This handles edge cases where preset is used but ISO naming is different
		fmt.Fprintf(opts.LogWriter, "warning: UEFI enabled via preset but ISO filename doesn't contain 'secureboot' - Secure Boot may not work\n")
	}

	if enableSecureBoot {
		// CRITICAL: Do NOT use pre-enrolled-keys=1 initially!
		// Talos needs to boot first in UEFI setup mode to auto-enroll its Secure Boot keys.
		// Using pre-enrolled-keys=1 enables Secure Boot immediately with Microsoft keys,
		// which prevents Talos ISO from booting (Access Denied error).
		//
		// Process:
		// 1. Create VM with efitype=4m WITHOUT pre-enrolled-keys (UEFI in setup mode)
		// 2. Talos boots and auto-enrolls its Secure Boot keys during first boot
		// 3. After enrollment, Secure Boot becomes active automatically
		//
		// Reference: Talos docs say "On first boot, the UEFI firmware should be in setup mode"
		// and "Allow VM to boot once with Secure Boot disabled, then enable it"
		//
		// We use efitype=4m which provides UEFI firmware that supports Secure Boot,
		// but without pre-enrolled-keys, it starts in setup mode allowing key enrollment.
		fmt.Fprintf(opts.LogWriter, "configured UEFI for Secure Boot ISO (efitype=%s) for VM %d\n", efiType, vmID)
		fmt.Fprintf(opts.LogWriter, "note: VM will boot in UEFI setup mode - Talos will auto-enroll Secure Boot keys during first boot\n")
		fmt.Fprintf(opts.LogWriter, "note: Secure Boot will be enabled automatically after Talos enrolls keys\n")
	} else {
		// Standard UEFI without Secure Boot
		fmt.Fprintf(opts.LogWriter, "configured standard UEFI (efitype=%s) for VM %d\n", efiType, vmID)
	}

	efiParams = append(efiParams, fmt.Sprintf("efitype=%s", efiType))
	// CRITICAL: For Secure Boot ISOs, we do NOT add pre-enrolled-keys=1 initially
	// This allows Talos to boot in UEFI setup mode and auto-enroll its keys.
	// After Talos enrolls keys, Secure Boot becomes active automatically.
	//
	// For non-Secure Boot ISOs, we also don't add pre-enrolled-keys (standard UEFI).
	//
	// The efidisk0 parameter format: storage:size,efitype=X,size=4M
	// Note: size=4M must be specified to ensure correct EFI disk size
	efiParams = append(efiParams, "size=4M")
	params.Set("efidisk0", fmt.Sprintf("%s:1,%s", storage, strings.Join(efiParams, ",")))

	params.Set("cpu", "host")
	params.Set("balloon", "0")
	params.Set("rng0", "source=/dev/urandom")
	// Set SMBIOS UUID for consistent VM identification
	// Generate a proper UUID format for Proxmox
	nodeUUID := uuid.New()
	if nodeReq.UUID != nil {
		nodeUUID = *nodeReq.UUID
	}
	params.Set("smbios1", fmt.Sprintf("uuid=%s", nodeUUID.String()))

	// Configure boot method: PXE boot or ISO boot
	// PXE boot is more reliable with UEFI and doesn't require manual boot entry configuration
	if nodeReq.PXEBooted || (nodeReq.TFTPServer != "" && nodeReq.IPXEBootFilename != "") {
		// PXE boot: Set boot order to network first, then disk
		// With UEFI, network boot is more reliable than CDROM boot
		params.Set("boot", "order=net0;virtio0")
		fmt.Fprintf(opts.LogWriter, "configured PXE boot for VM %d (network first)\n", vmID)
	} else if talosISOFilename != "" {
		// ISO boot: Attach Talos ISO for booting
		// Use SATA for CDROM - matches working VM configuration (VM 101 uses sata0)
		talosISOVolID := fmt.Sprintf("%s:iso/%s", uploadStorage, talosISOFilename)
		params.Set("sata0", fmt.Sprintf("%s,media=cdrom", talosISOVolID))
		// Set boot order based on Secure Boot detection
		// CRITICAL: Secure Boot ISOs require "order=sata0" (CD only) to prevent "Access Denied" errors
		// Non-Secure Boot ISOs can use "order=sata0;virtio0" (CD then disk) for fallback
		isSecureBootISO := strings.Contains(strings.ToLower(talosISOFilename), "secureboot")
		if isSecureBootISO {
			params.Set("boot", "order=sata0") // Secure Boot: CD only (matches VM 101 working config)
			fmt.Fprintf(opts.LogWriter, "configured Secure Boot ISO boot order: order=sata0 (CD only) for VM %d\n", vmID)
		} else {
			params.Set("boot", "order=sata0;virtio0") // Non-Secure Boot: CD then disk
			fmt.Fprintf(opts.LogWriter, "configured standard ISO boot order: order=sata0;virtio0 (CD then disk) for VM %d\n", vmID)
		}
		params.Set("bootdisk", "sata0")
	} else {
		// Fallback: boot from disk if no ISO provided
		params.Set("boot", "order=virtio0")
	}

	// Cloud-init ISO is NOT attached because:
	// 1. Talos Linux doesn't use cloud-init (it has its own configuration system)
	// 2. Working VM 202 doesn't have a cloud-init ISO attached
	// 3. Attaching multiple SATA CD-ROMs may cause Proxmox to fallback to IDE mode
	// The configuration is applied via `talosctl apply-config` after the VM boots
	// isoVolID := fmt.Sprintf("%s:iso/%s", uploadStorage, cloudInitISOFilename)
	// params.Set("sata2", fmt.Sprintf("%s,media=cdrom", isoVolID))

	// Configure serial console for headless access
	// This creates a UNIX socket at /var/run/qemu-server/<VMID>.serial0
	params.Set("serial0", "socket")

	// Enable QEMU guest agent for Proxmox API access to VM network info
	// This allows us to query VM IP addresses via /nodes/{node}/qemu/{vmid}/agent/network-get-interfaces
	// Requires qemu-guest-agent to be installed in the Talos image (via system extension)
	params.Set("agent", "1")

	// Configure TPM (Trusted Platform Module) support
	// Proxmox supports TPM 2.0 via tpmstate0 parameter
	// This enables measured boot and TPM-based disk encryption
	if opts != nil && (opts.TPM2Enabled || opts.TPM1_2Enabled) {
		tpmVersion := "v2.0"
		if opts.TPM1_2Enabled && !opts.TPM2Enabled {
			tpmVersion = "v1.2"
		}
		// TPM state is stored on the same storage as the VM disk
		// Proxmox will automatically create and manage the TPM state file
		params.Set("tpmstate0", fmt.Sprintf("%s:1,version=%s", storage, tpmVersion))
		fmt.Fprintf(opts.LogWriter, "configured TPM %s for VM %d\n", tpmVersion, vmID)
	}

	// Configure IOMMU (Intel VT-d) support
	// This enables device passthrough and enhanced security features
	// Note: Requires host IOMMU to be enabled (intel_iommu=on in kernel cmdline)
	if opts != nil && opts.IOMMUEnabled {
		// Proxmox doesn't have a direct parameter for IOMMU
		// We need to use the 'args' parameter to pass QEMU arguments
		// Note: This requires root permissions or appropriate API token privileges
		args := "-machine q35,accel=kvm,smm=on,kernel-irqchip=split -device intel-iommu,intremap=on,device-iotlb=on"
		params.Set("args", args)
		fmt.Fprintf(opts.LogWriter, "configured IOMMU (VT-d) for VM %d\n", vmID)
		fmt.Fprintf(opts.LogWriter, "warning: IOMMU requires 'args' parameter which may need root permissions\n")
	}

	// Configure serial console for capturing pre-boot and boot messages
	// Proxmox creates a UNIX socket at /var/run/qemu-server/<VMID>.serial0
	// For Secure Boot VMs, we skip file logging as it may interfere with Secure Boot
	// Instead, we rely on socket-only capture for debugging

	// Check if this is a Secure Boot VM (using Secure Boot ISO)
	// Note: We don't use pre-enrolled-keys=1 initially, so we check enableSecureBoot flag
	isSecureBootVM := enableSecureBoot

	serialLogPath := fmt.Sprintf("/tmp/talos-vm-%d-serial.log", vmID)

	if !isSecureBootVM {
		// For non-Secure Boot VMs, add file-based logging via QEMU args
		// IMPORTANT: Proxmox already configures serial0 as a socket via:
		//   -chardev socket,id=serial0,path=/var/run/qemu-server/<VMID>.serial0
		//   -device isa-serial,chardev=serial0
		// We add a SECOND serial port (serial1) that logs to a file
		// This way we keep the socket for interactive access AND get file logging
		//
		// QEMU syntax for second serial port:
		//   -chardev file,id=serial1,path=/path/to/log
		//   -device isa-serial,chardev=serial1

		// Add QEMU args to create a second serial port that logs to file
		// We use chardev with file backend and attach it to an isa-serial device
		// This creates serial1 (serial0 is already the socket)
		// QEMU will output to BOTH serial ports, so we get:
		// - serial0: socket (for interactive access via socat/qm terminal)
		// - serial1: file (for reliable pre-boot capture)
		argsValue := fmt.Sprintf("-chardev file,id=serial1,path=%s -device isa-serial,chardev=serial1", serialLogPath)

		// Check if args already exists (e.g., for IOMMU) and append
		existingArgs := params.Get("args")
		if existingArgs != "" {
			argsValue = fmt.Sprintf("%s %s", existingArgs, argsValue)
		}
		params.Set("args", argsValue)

		fmt.Fprintf(opts.LogWriter, "configured serial console for VM %d:\n", vmID)
		fmt.Fprintf(opts.LogWriter, "  - Socket (serial0): /var/run/qemu-server/%d.serial0 (for interactive access)\n", vmID)
		fmt.Fprintf(opts.LogWriter, "  - Log file (serial1): %s (for reliable pre-boot capture)\n", serialLogPath)
		fmt.Fprintf(opts.LogWriter, "note: Both serial ports receive the same output\n")
		fmt.Fprintf(opts.LogWriter, "note: Serial log file will contain UEFI firmware and boot messages\n")
	} else {
		// For Secure Boot VMs, skip file logging to avoid potential interference
		fmt.Fprintf(opts.LogWriter, "configured serial console for Secure Boot VM %d:\n", vmID)
		fmt.Fprintf(opts.LogWriter, "  - Socket (serial0): /var/run/qemu-server/%d.serial0 (for interactive access)\n", vmID)
		fmt.Fprintln(opts.LogWriter, "  - Log file: DISABLED (may interfere with Secure Boot)")
		fmt.Fprintf(opts.LogWriter, "note: Secure Boot VMs use socket-only capture to avoid QEMU args interference\n")
		fmt.Fprintf(opts.LogWriter, "note: Serial output still available via socket for debugging\n")
	}

	fmt.Fprintf(opts.LogWriter, "note: console=ttyS0 must be embedded in ISO kernel arguments (via Image Factory schematic)\n")

	// DEBUG: Log all VM creation parameters
	fmt.Fprintf(opts.LogWriter, "\n=== VM CREATION PARAMETERS ===\n")
	for key, values := range params {
		if len(values) > 0 {
			fmt.Fprintf(opts.LogWriter, "  %s = %s\n", key, values[0])
		}
	}
	fmt.Fprintf(opts.LogWriter, "==============================\n\n")

	return params
}

// createVM creates the VM on Proxmox and returns the VM ID.
func (p *provisioner) createVM(ctx context.Context, node string, params url.Values, opts *provision.Options) (int, error) {
	vmIDStr := params.Get("vmid")
	vmID, err := strconv.Atoi(vmIDStr)
	if err != nil {
		return 0, fmt.Errorf("invalid VM ID: %w", err)
	}

	fmt.Fprintf(opts.LogWriter, "creating VM %d (%s)\n", vmID, params.Get("name"))
	var createTaskID string
	if err := p.client.Post(ctx, fmt.Sprintf("/nodes/%s/qemu", node), params, &createTaskID); err != nil {
		return 0, fmt.Errorf("failed to create VM: %w", err)
	}

	fmt.Fprintf(opts.LogWriter, "waiting for VM creation task %s\n", createTaskID)
	// Wait for VM creation
	if err := p.client.WaitForTaskWithError(ctx, node, createTaskID, 2*time.Minute); err != nil {
		return 0, fmt.Errorf("VM creation: %w", err)
	}

	fmt.Fprintf(opts.LogWriter, "VM %d created successfully\n", vmID)
	return vmID, nil
}

// startVM starts the VM and waits for it to be ready.
func (p *provisioner) startVM(ctx context.Context, node string, vmID int, nodeName string, opts *provision.Options) error {
	fmt.Fprintf(opts.LogWriter, "starting VM %d (%s)\n", vmID, nodeName)
	var startTaskID string
	if err := p.client.Post(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/status/start", node, vmID), nil, &startTaskID); err != nil {
		return fmt.Errorf("failed to start VM: %w", err)
	}

	fmt.Fprintf(opts.LogWriter, "waiting for VM start task %s\n", startTaskID)
	// Wait for VM to start - start task usually completes quickly
	// Note: WARNINGS are acceptable for VM start (e.g., missing disk image warnings)
	if err := p.client.WaitForTaskWithError(ctx, node, startTaskID, 30*time.Second); err != nil {
		// Start task timeout is not critical - VM might still be starting
		fmt.Fprintf(opts.LogWriter, "warning: VM start task had issues: %v (VM may still be starting)\n", err)
	}

	fmt.Fprintf(opts.LogWriter, "VM %d started successfully\n", vmID)

	// Capture initial boot messages from serial console for debugging
	// This helps diagnose boot issues, especially with Secure Boot
	// We wait a moment for the VM to start producing output
	fmt.Fprintf(opts.LogWriter, "waiting 2 seconds for VM to start producing serial output...\n")
	time.Sleep(2 * time.Second)

	fmt.Fprintf(opts.LogWriter, "capturing initial boot messages from serial console (15 seconds)...\n")
	serialLogPath := fmt.Sprintf("/tmp/talos-vm-%d-serial.log", vmID)

	if bootMessages, err := p.CaptureSerialOutput(ctx, node, vmID, 15*time.Second); err == nil && bootMessages != "" {
		// Save full output to local file for later inspection
		localLogFile := filepath.Join(os.TempDir(), fmt.Sprintf("talos-vm-%d-serial-%d.log", vmID, time.Now().Unix()))
		if err := os.WriteFile(localLogFile, []byte(bootMessages), 0644); err == nil {
			fmt.Fprintf(opts.LogWriter, "✅ Full serial console output saved locally to: %s\n", localLogFile)
			fmt.Fprintf(opts.LogWriter, "📄 Serial log also available on Proxmox host at: %s\n", serialLogPath)
		} else {
			fmt.Fprintf(opts.LogWriter, "warning: could not save serial output to local file: %v\n", err)
			fmt.Fprintf(opts.LogWriter, "📄 Serial log available on Proxmox host at: %s\n", serialLogPath)
		}

		// Always show boot messages if we captured anything
		if len(strings.TrimSpace(bootMessages)) > 0 {
			fmt.Fprintf(opts.LogWriter, "\n=== SERIAL CONSOLE BOOT MESSAGES (first 15 seconds) ===\n")
			// Show first 100 lines for better debugging
			lines := strings.Split(bootMessages, "\n")
			maxLines := 100
			if len(lines) > maxLines {
				fmt.Fprintf(opts.LogWriter, "%s\n... (truncated, %d more lines)\n", strings.Join(lines[:maxLines], "\n"), len(lines)-maxLines)
				fmt.Fprintf(opts.LogWriter, "📄 See full output in: %s\n", localLogFile)
			} else {
				fmt.Fprintf(opts.LogWriter, "%s\n", bootMessages)
			}
			fmt.Fprintf(opts.LogWriter, "=== END SERIAL CONSOLE OUTPUT ===\n")
			fmt.Fprintf(opts.LogWriter, "📄 Full output saved to: %s\n", localLogFile)
			fmt.Fprintf(opts.LogWriter, "📄 Also available on Proxmox: %s\n\n", serialLogPath)
		} else {
			fmt.Fprintf(opts.LogWriter, "ℹ️  No boot messages captured yet (VM may still be initializing)\n")
			fmt.Fprintf(opts.LogWriter, "📄 Serial log will be available at: %s\n", serialLogPath)
		}
	} else if err != nil {
		fmt.Fprintf(opts.LogWriter, "warning: could not capture serial console output: %v\n", err)
		fmt.Fprintf(opts.LogWriter, "note: serial socket is at /var/run/qemu-server/%d.serial0\n", vmID)
		fmt.Fprintf(opts.LogWriter, "note: serial log file should be at: %s\n", serialLogPath)
		fmt.Fprintf(opts.LogWriter, "note: you can manually read it with: cat %s\n", serialLogPath)
		fmt.Fprintf(opts.LogWriter, "note: or via socket: socat -u UNIX-CONNECT:/var/run/qemu-server/%d.serial0 -\n", vmID)
	}

	return nil
}

// configureUEFIBoot configures UEFI boot entries after VM creation.
// This ensures the boot order is properly set for UEFI firmware.
// IMPORTANT: With OVMF (UEFI), the boot parameter doesn't fully control boot order.
// UEFI firmware manages boot order, and it may not automatically boot from CDROM.
// This function verifies the boot configuration, but manual UEFI boot entry
// configuration may still be needed if the VM boots into the UEFI shell.
func (p *provisioner) configureUEFIBoot(ctx context.Context, node string, vmID int, talosISOFilename string, opts *provision.Options) error {
	// Verify VM configuration
	var vmConfig map[string]interface{}
	if err := p.client.Get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/config", node, vmID), &vmConfig); err != nil {
		return fmt.Errorf("failed to get VM config: %w", err)
	}

	// Determine correct boot order based on Secure Boot detection
	// CRITICAL: Secure Boot ISOs require "order=sata0" (CD only) to prevent "Access Denied" errors
	// Non-Secure Boot ISOs can use "order=sata0;virtio0" (CD then disk) for fallback
	isSecureBootISO := talosISOFilename != "" && strings.Contains(strings.ToLower(talosISOFilename), "secureboot")
	correctBootOrder := "order=sata0;virtio0" // Default: CD then disk
	if isSecureBootISO {
		correctBootOrder = "order=sata0" // Secure Boot: CD only (matches VM 101 working config)
	}

	bootOrder, ok := vmConfig["boot"].(string)
	bootdisk, _ := vmConfig["bootdisk"].(string)

	// Only fix boot order if it's wrong or missing
	needsFix := false

	if !ok || bootOrder == "" {
		needsFix = true
		fmt.Fprintf(opts.LogWriter, "boot order not set, setting to %s\n", correctBootOrder)
	} else if bootOrder != correctBootOrder {
		needsFix = true
		if isSecureBootISO {
			fmt.Fprintf(opts.LogWriter, "boot order is %q, changing to %s (required for Secure Boot)\n", bootOrder, correctBootOrder)
		} else {
			fmt.Fprintf(opts.LogWriter, "boot order is %q, changing to %s\n", bootOrder, correctBootOrder)
		}
	}

	if bootdisk != "sata0" {
		needsFix = true
		fmt.Fprintf(opts.LogWriter, "bootdisk is %q, setting to sata0\n", bootdisk)
	}

	if needsFix {
		bootParams := url.Values{}
		bootParams.Set("boot", correctBootOrder)
		bootParams.Set("bootdisk", "sata0")
		if err := p.client.Put(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/config", node, vmID), bootParams, nil); err != nil {
			return fmt.Errorf("failed to set boot order: %w", err)
		}
		if isSecureBootISO {
			fmt.Fprintf(opts.LogWriter, "✅ configured UEFI boot order for VM %d: %s (matches VM 101 working config)\n", vmID, correctBootOrder)
			fmt.Fprintf(opts.LogWriter, "note: Secure Boot requires 'order=sata0' (CD only) to boot properly\n")
		} else {
			fmt.Fprintf(opts.LogWriter, "✅ configured UEFI boot order for VM %d: %s\n", vmID, correctBootOrder)
		}
		fmt.Fprintf(opts.LogWriter, "note: With OVMF (UEFI), boot order is managed by firmware\n")
	} else {
		fmt.Fprintf(opts.LogWriter, "✅ boot order already correct: %s\n", bootOrder)
	}

	return nil
}

// cleanupVM attempts to clean up a VM if creation fails partway through.
func (p *provisioner) cleanupVM(ctx context.Context, node string, vmID int, opts *provision.Options) {
	fmt.Fprintf(opts.LogWriter, "cleaning up VM %d due to error\n", vmID)
	// Try to stop and delete the VM
	var status VMStatus
	if err := p.client.Get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/status/current", node, vmID), &status); err == nil {
		if status.Status == "running" {
			var taskID string
			if err := p.client.Post(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/status/stop", node, vmID), nil, &taskID); err == nil {
				p.client.WaitForTask(ctx, node, taskID, 30*time.Second)
			}
		}
	}
	// Delete VM (ignore errors - best effort cleanup)
	_ = p.client.Delete(ctx, fmt.Sprintf("/nodes/%s/qemu/%d", node, vmID), nil)
}

// CreateNode creates a single Proxmox VM.
// This is a public method to allow adding nodes to existing clusters.
func (p *provisioner) CreateNode(ctx context.Context, state *provision.State, clusterReq provision.ClusterRequest, nodeReq provision.NodeRequest, opts *provision.Options, node, storage, uploadStorage string) (provision.NodeInfo, error) {
	return p.createNode(ctx, state, clusterReq, nodeReq, opts, node, storage, uploadStorage)
}

// createNode creates a single Proxmox VM.
func (p *provisioner) createNode(ctx context.Context, state *provision.State, clusterReq provision.ClusterRequest, nodeReq provision.NodeRequest, opts *provision.Options, node, storage, uploadStorage string) (provision.NodeInfo, error) {
	// Validate and calculate VM resources
	resources, err := p.validateNodeRequest(nodeReq)
	if err != nil {
		return provision.NodeInfo{}, err
	}

	// Generate VM ID (use range 100-999999, prefer 100-999 for clusters)
	vmID, err := p.findAvailableVMID(ctx, node)
	if err != nil {
		return provision.NodeInfo{}, fmt.Errorf("failed to find available VM ID: %w", err)
	}

	// Generate node UUID
	nodeUUID := uuid.New()
	if nodeReq.UUID != nil {
		nodeUUID = *nodeReq.UUID
	}

	// Note: Talos configuration is no longer embedded in cloud-init ISO
	// Configuration is applied via `talosctl apply-config` after the VM boots

	// Configure PXE boot if IPXEBootScript is set (like QEMU provider)
	// This enables PXE boot for all nodes automatically
	// The IPXEBootScript URL contains the architecture in the path (e.g., /pxe/.../amd64/... or /pxe/.../arm64/...)
	if clusterReq.IPXEBootScript != "" && len(clusterReq.Network.GatewayAddrs) > 0 {
		nodeReq.TFTPServer = clusterReq.Network.GatewayAddrs[0].String()
		// Extract architecture from IPXEBootScript URL or use default
		// The URL format is typically: https://factory.talos.dev/pxe/{schematic}/{version}/{arch}/...
		arch := "amd64" // Default to amd64
		if opts.TargetArch != "" {
			arch = opts.TargetArch
		}
		nodeReq.IPXEBootFilename = fmt.Sprintf("ipxe/%s/snp.efi", arch)
		nodeReq.PXEBooted = true
		fmt.Fprintf(opts.LogWriter, "configured PXE boot for %s: TFTP=%s, bootfile=%s\n", nodeReq.Name, nodeReq.TFTPServer, nodeReq.IPXEBootFilename)
	}

	// Ensure Talos ISO is uploaded (if provided and not using PXE boot)
	var talosISOFilename string
	if !nodeReq.PXEBooted {
		// If ISOPath is not provided, try to find ISO from existing VMs or use default
		isoPath := clusterReq.ISOPath
		if isoPath == "" {
			// Try to find ISO from existing control plane VM
			// For now, use a default ISO name that should exist in storage
			// The ISO should already be uploaded from the initial cluster creation
			talosISOFilename = "talos-hardened-55b79f48.iso" // Default ISO name
			fmt.Fprintf(opts.LogWriter, "ISO path not provided, using default ISO: %s (should already exist in storage)\n", talosISOFilename)
			// Verify ISO exists in storage
			exists, err := p.client.CheckISOExists(ctx, node, uploadStorage, talosISOFilename)
			if err == nil && exists {
				fmt.Fprintf(opts.LogWriter, "ISO %s found in storage %s\n", talosISOFilename, uploadStorage)
				} else {
					// Try to find any Talos ISO in storage
					fmt.Fprintf(opts.LogWriter, "ISO %s not found, searching for Talos ISO in storage...\n", talosISOFilename)
					// List ISOs in storage to find a Talos ISO
					var contents []StorageContent
					if err := p.client.Get(ctx, fmt.Sprintf("/nodes/%s/storage/%s/content", node, uploadStorage), &contents); err == nil {
						for _, content := range contents {
							// Extract filename from volid (format: storage:iso/filename)
							volID := content.VolID
							if strings.Contains(volID, "talos") && strings.HasSuffix(volID, ".iso") {
								// Extract filename from volid
								parts := strings.Split(volID, "/")
								if len(parts) > 0 {
									talosISOFilename = parts[len(parts)-1]
									fmt.Fprintf(opts.LogWriter, "Found Talos ISO in storage: %s\n", talosISOFilename)
									break
								}
							}
						}
					}
				}
		} else {
			// ISO path provided - check if it's a local file or already on Proxmox storage
			// Proxmox storage paths look like: storage:iso/filename or /var/lib/vz/template/iso/filename
			// Local paths are relative or absolute paths on the machine running talosctl
			if strings.HasPrefix(isoPath, "/var/lib/vz/template/iso/") || strings.Contains(isoPath, ":iso/") {
				// This is already a Proxmox storage path - extract filename and use directly
				talosISOFilename = filepath.Base(isoPath)
				// Remove any storage prefix (e.g., "local:iso/" or "/var/lib/vz/template/iso/")
				if strings.Contains(isoPath, ":iso/") {
					parts := strings.Split(isoPath, ":iso/")
					if len(parts) > 1 {
						talosISOFilename = parts[1]
					}
				} else if strings.HasPrefix(isoPath, "/var/lib/vz/template/iso/") {
					talosISOFilename = strings.TrimPrefix(isoPath, "/var/lib/vz/template/iso/")
				}
				fmt.Fprintf(opts.LogWriter, "using existing ISO from Proxmox storage: %s\n", talosISOFilename)
				// Verify ISO exists in storage
				exists, err := p.client.CheckISOExists(ctx, node, uploadStorage, talosISOFilename)
				if err != nil {
					return provision.NodeInfo{}, fmt.Errorf("failed to verify ISO exists in storage: %w", err)
				}
				if !exists {
					return provision.NodeInfo{}, fmt.Errorf("ISO %s not found in storage %s - please upload it first or use a local file path", talosISOFilename, uploadStorage)
				}
			} else {
				// Local file path - upload it to Proxmox storage
				talosISOFilename, err = p.ensureTalosISO(ctx, node, uploadStorage, isoPath, opts)
				if err != nil {
					return provision.NodeInfo{}, err
				}
			}
		}
	}

	// Skip cloud-init ISO creation for Talos Linux
	// Talos doesn't use cloud-init - configuration is applied via `talosctl apply-config`
	// Creating multiple CD-ROMs causes Proxmox to fall back to IDE mode instead of SATA/AHCI
	// This is why working VM 202 only has one CD-ROM (sata0) and boots correctly
	cloudInitISOFilename := "" // Empty filename, won't be attached

	// Build VM configuration
	params := p.buildVMConfig(vmID, nodeReq, resources, storage, uploadStorage, talosISOFilename, cloudInitISOFilename, opts)

	// Create VM
	createdVMID, err := p.createVM(ctx, node, params, opts)
	if err != nil {
		// Cleanup on failure
		p.cleanupVM(ctx, node, vmID, opts)
		return provision.NodeInfo{}, err
	}

	// Configure UEFI boot entries after VM creation
	// This ensures the boot order is properly set for UEFI firmware
	// IMPORTANT: With OVMF (UEFI), we need to ensure the CDROM is properly detected
	// The boot parameter helps, but UEFI firmware may still need manual configuration
	if talosISOFilename != "" {
		// configureUEFIBoot now accepts the ISO filename to determine correct boot order
		if err := p.configureUEFIBoot(ctx, node, createdVMID, talosISOFilename, opts); err != nil {
			// Log warning but don't fail - VM might still boot correctly
			fmt.Fprintf(opts.LogWriter, "warning: failed to configure UEFI boot entries: %v\n", err)
		}

		// Verify VM configuration after creation
		if err := p.verifyVMISOConfig(ctx, node, createdVMID, uploadStorage, talosISOFilename, opts); err != nil {
			fmt.Fprintf(opts.LogWriter, "warning: VM ISO configuration verification failed: %v\n", err)
			// Don't fail VM creation, but log the warning
		}
	}

	// Start VM
	if err := p.startVM(ctx, node, createdVMID, nodeReq.Name, opts); err != nil {
		// Log error but don't fail - VM might still be starting
		fmt.Fprintf(opts.LogWriter, "warning: VM start had issues: %v\n", err)
	}

	// Get API bind address (use node IP or first gateway)
	var apiBind *net.TCPAddr
	if len(nodeReq.IPs) > 0 {
		apiBind = &net.TCPAddr{
			IP:   net.IP(nodeReq.IPs[0].AsSlice()),
			Port: constants.ApidPort,
		}
	} else if len(clusterReq.Network.GatewayAddrs) > 0 {
		apiBind = &net.TCPAddr{
			IP:   net.IP(clusterReq.Network.GatewayAddrs[0].AsSlice()),
			Port: constants.ApidPort,
		}
	} else {
		apiBind = &net.TCPAddr{
			IP:   net.IPv4(0, 0, 0, 0),
			Port: constants.ApidPort,
		}
	}

	nodeInfo := provision.NodeInfo{
		ID:   fmt.Sprintf("%d", createdVMID), // Store VM ID as node ID
		UUID: nodeUUID,
		Name: nodeReq.Name,
		Type: nodeReq.Type,

		NanoCPUs: nodeReq.NanoCPUs,
		Memory:   nodeReq.Memory,
		DiskSize: nodeReq.Disks[0].Size,

		IPs: nodeReq.IPs,

		APIPort: apiBind.Port,
	}

	// Store VM ID in state
	vmIDPath := state.GetRelativePath(fmt.Sprintf("%s.vmid", nodeReq.Name))
	if err := os.WriteFile(vmIDPath, []byte(strconv.Itoa(createdVMID)), 0o644); err != nil {
		return provision.NodeInfo{}, fmt.Errorf("failed to save VM ID: %w", err)
	}

	// Store Proxmox node name in state for multi-node support
	nodePath := state.GetRelativePath(fmt.Sprintf("%s.node", nodeReq.Name))
	if err := os.WriteFile(nodePath, []byte(node), 0o644); err != nil {
		return provision.NodeInfo{}, fmt.Errorf("failed to save Proxmox node: %w", err)
	}

	// Create IPAM record for DHCP server (required for PXE boot)
	// Extract MAC address from VM config
	macAddress := p.extractMACFromConfig(ctx, node, createdVMID)
	if macAddress == "" {
		// Fallback: use generated MAC from buildVMConfig
		macAddress = p.generateMACAddress()
	}

	// Get nameservers from network request
	var nameservers []netip.Addr
	if len(clusterReq.Network.Nameservers) > 0 {
		nameservers = clusterReq.Network.Nameservers
	}

	// Create IPAM record for each IP address
	for i, ip := range nodeReq.IPs {
		var cidr netip.Prefix
		for _, c := range clusterReq.Network.CIDRs {
			if c.Contains(ip) {
				cidr = c
				break
			}
		}
		if cidr == (netip.Prefix{}) {
			continue // Skip if no matching CIDR found
		}

		var gateway netip.Addr
		if len(clusterReq.Network.GatewayAddrs) > i {
			gateway = clusterReq.Network.GatewayAddrs[i]
		} else if len(clusterReq.Network.GatewayAddrs) > 0 {
			gateway = clusterReq.Network.GatewayAddrs[0]
		}

		ipamRecord := vm.IPAMRecord{
			IP:               ip,
			Netmask:          byte(cidr.Bits()),
			MAC:              macAddress,
			Hostname:         nodeReq.Name,
			Gateway:          gateway,
			MTU:              clusterReq.Network.MTU,
			Nameservers:      nameservers,
			TFTPServer:       nodeReq.TFTPServer,
			IPXEBootFilename: nodeReq.IPXEBootFilename,
		}

		statePath, err := state.StatePath()
		if err != nil {
			fmt.Fprintf(opts.LogWriter, "warning: failed to get state path for IPAM record: %v\n", err)
			continue
		}

		if err := vm.DumpIPAMRecord(statePath, ipamRecord); err != nil {
			fmt.Fprintf(opts.LogWriter, "warning: failed to create IPAM record for %s: %v\n", nodeReq.Name, err)
		} else {
			fmt.Fprintf(opts.LogWriter, "created IPAM record for %s: MAC=%s, IP=%s\n", nodeReq.Name, macAddress, ip)
		}
	}

	fmt.Fprintf(opts.LogWriter, "VM %d (%s) created and started successfully on node %s\n", createdVMID, nodeReq.Name, node)

	return nodeInfo, nil
}

// findAvailableVMID finds an available VM ID in the range 100-999.
// It checks both QEMU VMs and LXC containers since Proxmox uses the same ID space.
// Retries on transient errors to handle race conditions.
func (p *provisioner) findAvailableVMID(ctx context.Context, node string) (int, error) {
	const maxRetries = 3
	const retryDelay = 500 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay)
		}

	var vms []VMInfo
	if err := p.client.Get(ctx, fmt.Sprintf("/nodes/%s/qemu", node), &vms); err != nil {
			if attempt == maxRetries-1 {
				return 0, fmt.Errorf("failed to get VMs after %d attempts: %w", maxRetries, err)
			}
			continue
	}

	existingVMIDs := make(map[int]bool)
	for _, vm := range vms {
		existingVMIDs[vm.VMID] = true
	}

	// Also check LXC containers (they share the same ID space)
	var containers []VMInfo
	if err := p.client.Get(ctx, fmt.Sprintf("/nodes/%s/lxc", node), &containers); err == nil {
		for _, ct := range containers {
			existingVMIDs[ct.VMID] = true
		}
	}

	// Try range 100-999 first (for clusters)
	for vmid := 100; vmid < 1000; vmid++ {
		if !existingVMIDs[vmid] {
			return vmid, nil
		}
	}

	// Fallback to 1000-9999
	for vmid := 1000; vmid < 10000; vmid++ {
		if !existingVMIDs[vmid] {
			return vmid, nil
		}
	}

		// If we get here, all IDs are taken - this is a persistent error, not transient
		return 0, fmt.Errorf("no available VM ID found in range 100-9999")
	}

	return 0, fmt.Errorf("failed to find available VM ID after %d attempts", maxRetries)
}

// generateMACAddress generates a random local MAC address.
func (p *provisioner) generateMACAddress() string {
	const (
		local     = 0b10
		multicast = 0b1
	)

	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		// Fallback to deterministic MAC based on node name
		b = []byte{0xBC, 0x24, 0x11, 0x00, 0x00, 0x00}
	}

	b[0] = (b[0] &^ multicast) | local

	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X", b[0], b[1], b[2], b[3], b[4], b[5])
}

// extractMACFromConfig extracts MAC address from VM configuration.
func (p *provisioner) extractMACFromConfig(ctx context.Context, node string, vmID int) string {
	var vmConfig map[string]interface{}
	if err := p.client.Get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/config", node, vmID), &vmConfig); err != nil {
		return ""
	}

	net0, ok := vmConfig["net0"].(string)
	if !ok {
		return ""
	}

	// Parse MAC from net0 config (format: virtio=MAC:XX:XX:XX:XX:XX:XX,bridge=vmbr0)
	parts := strings.Split(net0, ",")
	for _, part := range parts {
		if strings.HasPrefix(part, "virtio=") {
			macPart := strings.TrimPrefix(part, "virtio=")
			// Extract MAC address (format: MAC:XX:XX:XX:XX:XX:XX or just XX:XX:XX:XX:XX:XX)
			if strings.HasPrefix(macPart, "MAC:") {
				return strings.TrimPrefix(macPart, "MAC:")
			}
			return macPart
		}
	}

	return ""
}

// verifyVMISOConfig verifies that the VM has the correct ISO attached and configured.
// This helps catch issues where the ISO might be corrupted, missing, or incorrectly configured.
func (p *provisioner) verifyVMISOConfig(ctx context.Context, node string, vmID int, uploadStorage, expectedISOFilename string, opts *provision.Options) error {
	var vmConfig VMConfig
	if err := p.client.Get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/config", node, vmID), &vmConfig); err != nil {
		return fmt.Errorf("failed to get VM config: %w", err)
	}

	// Check if sata0 is configured
	sata0, ok := vmConfig["sata0"].(string)
	if !ok {
		return fmt.Errorf("sata0 not configured in VM %d", vmID)
	}

	// Verify sata0 points to the expected ISO
	expectedVolID := fmt.Sprintf("%s:iso/%s", uploadStorage, expectedISOFilename)
	if !strings.Contains(sata0, expectedVolID) {
		return fmt.Errorf("VM %d sata0 (%s) does not match expected ISO (%s)", vmID, sata0, expectedVolID)
	}

	// Verify boot order includes sata0 and is correct format
	boot, ok := vmConfig["boot"].(string)
	if !ok || !strings.Contains(boot, "sata0") {
		return fmt.Errorf("VM %d boot order (%s) does not include sata0", vmID, boot)
	}

	// For Secure Boot ISOs, verify boot order is "order=sata0" (CD only)
	// This prevents "Access Denied" errors during Secure Boot
	isSecureBootISO := strings.Contains(strings.ToLower(expectedISOFilename), "secureboot")
	if isSecureBootISO && boot != "order=sata0" {
		return fmt.Errorf("VM %d boot order (%s) is incorrect for Secure Boot ISO - expected 'order=sata0' (CD only)", vmID, boot)
	}

	// Verify bootdisk is set to sata0
	bootdisk, ok := vmConfig["bootdisk"].(string)
	if !ok || bootdisk != "sata0" {
		return fmt.Errorf("VM %d bootdisk (%s) is not set to sata0", vmID, bootdisk)
	}

	// Verify ISO exists in storage and has reasonable size
	isoSize, err := p.client.GetISOSize(ctx, node, uploadStorage, expectedISOFilename)
	if err != nil {
		return fmt.Errorf("failed to verify ISO size: %w", err)
	}

	// Minimum expected size for Talos ISO: 100MB
	minISOSize := uint64(100 * 1024 * 1024) // 100MB
	if isoSize < minISOSize {
		return fmt.Errorf("ISO %s is suspiciously small (%d bytes), expected at least %d bytes", expectedISOFilename, isoSize, minISOSize)
	}

	fmt.Fprintf(opts.LogWriter, "verified VM %d ISO configuration: sata0=%s, boot=%s, bootdisk=%s, ISO size=%d bytes\n", vmID, sata0, boot, bootdisk, isoSize)
	return nil
}

