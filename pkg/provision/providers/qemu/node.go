// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/klauspost/compress/zstd"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-cmd/pkg/cmd"
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/kernel"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

//nolint:gocyclo,cyclop
func (p *provisioner) createNode(state *vm.State, clusterReq provision.ClusterRequest, nodeReq provision.NodeRequest, opts *provision.Options) (provision.NodeInfo, error) {
	arch := Arch(opts.TargetArch)
	pidPath := state.GetRelativePath(fmt.Sprintf("%s.pid", nodeReq.Name))

	var pflashImages []string

	if pflashSpec := arch.PFlash(opts.UEFIEnabled, opts.ExtraUEFISearchPaths); pflashSpec != nil {
		var err error

		if pflashImages, err = p.createPFlashImages(state, nodeReq.Name, pflashSpec); err != nil {
			return provision.NodeInfo{}, fmt.Errorf("error creating flash images: %w", err)
		}
	}

	vcpuCount := int64(math.RoundToEven(float64(nodeReq.NanoCPUs) / 1000 / 1000 / 1000))
	if vcpuCount < 2 {
		vcpuCount = 1
	}

	memSize := nodeReq.Memory / 1024 / 1024

	diskPaths, err := p.CreateDisks(state, nodeReq)
	if err != nil {
		return provision.NodeInfo{}, err
	}

	err = p.populateSystemDisk(diskPaths, clusterReq)
	if err != nil {
		return provision.NodeInfo{}, err
	}

	logFile, err := os.OpenFile(state.GetRelativePath(fmt.Sprintf("%s.log", nodeReq.Name)), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return provision.NodeInfo{}, err
	}

	defer logFile.Close() //nolint:errcheck

	cmdline := procfs.NewCmdline("")

	cmdline.SetAll(kernel.DefaultArgs(nodeReq.Quirks))

	// required to get kernel console
	cmdline.Append("console", arch.Console())

	// reboot configuration
	cmdline.Append("reboot", "k")
	cmdline.Append("panic", "1")
	cmdline.Append("talos.shutdown", "halt")

	// Talos config
	cmdline.Append("talos.platform", constants.PlatformMetal)

	// add overrides
	if nodeReq.ExtraKernelArgs != nil {
		if err = cmdline.AppendAll(
			nodeReq.ExtraKernelArgs.Strings(),
			procfs.WithDeleteNegatedArgs(),
		); err != nil {
			return provision.NodeInfo{}, err
		}
	}

	if opts.WithDebugShell {
		cmdline.Append("talos.debugshell", "")
	}

	var (
		nodeConfig   string
		extraISOPath string
	)

	if !nodeReq.SkipInjectingConfig {
		nodeConfig, err = nodeReq.Config.EncodeString()
		if err != nil {
			return provision.NodeInfo{}, err
		}

		switch nodeReq.ConfigInjectionMethod {
		case provision.ConfigInjectionMethodHTTP:
			cmdline.Append("talos.config", "{TALOS_CONFIG_URL}") // to be patched by launcher
		case provision.ConfigInjectionMethodMetalISO:
			cmdline.Append("talos.config", "metal-iso")

			extraISOPath, err = p.createMetalConfigISO(state, nodeReq.Name, nodeConfig)
			if err != nil {
				return provision.NodeInfo{}, fmt.Errorf("error creating metal-iso: %w", err)
			}
		}
	}

	nodeUUID := uuid.New()
	if nodeReq.UUID != nil {
		nodeUUID = *nodeReq.UUID
	}

	apiBind, err := p.findAPIBindAddrs(clusterReq)
	if err != nil {
		return provision.NodeInfo{}, fmt.Errorf("error finding listen address for the API: %w", err)
	}

	defaultBootOrder := "cd"
	if nodeReq.DefaultBootOrder != "" {
		defaultBootOrder = nodeReq.DefaultBootOrder
	}

	// backwards compatibility, set Driver/BlockSize if not set
	for i := range nodeReq.Disks {
		if nodeReq.Disks[i].BlockSize == 0 {
			nodeReq.Disks[i].BlockSize = 512
		}

		if nodeReq.Disks[i].Driver != "" {
			continue
		}

		if i == 0 {
			nodeReq.Disks[i].Driver = "virtio"
		} else {
			nodeReq.Disks[i].Driver = "ide"
		}
	}

	launchConfig := LaunchConfig{
		ArchitectureData: arch,
		DiskPaths:        diskPaths,
		DiskDrivers: xslices.Map(nodeReq.Disks, func(disk *provision.Disk) string {
			return disk.Driver
		}),
		DiskBlockSizes: xslices.Map(nodeReq.Disks, func(disk *provision.Disk) uint {
			return disk.BlockSize
		}),
		VCPUCount:         vcpuCount,
		MemSize:           memSize,
		KernelArgs:        cmdline.String(),
		ExtraISOPath:      extraISOPath,
		PFlashImages:      pflashImages,
		MonitorPath:       state.GetRelativePath(fmt.Sprintf("%s.monitor", nodeReq.Name)),
		BadRTC:            nodeReq.BadRTC,
		DefaultBootOrder:  defaultBootOrder,
		BootloaderEnabled: opts.BootloaderEnabled,
		NodeUUID:          nodeUUID,
		Config:            nodeConfig,
		TFTPServer:        nodeReq.TFTPServer,
		IPXEBootFileName:  nodeReq.IPXEBootFilename,
		APIBindAddress:    apiBind,
		WithDebugShell:    opts.WithDebugShell,
		IOMMUEnabled:      opts.IOMMUEnabled,
		Network:           getLaunchNetworkConfig(state, clusterReq, nodeReq),

		// Generate a random MAC address.
		// On linux this is later overridden to the interface mac.
		VMMac: getRandomMacAddress(),
	}

	if clusterReq.IPXEBootScript != "" {
		launchConfig.TFTPServer = clusterReq.Network.GatewayAddrs[0].String()
		launchConfig.IPXEBootFileName = fmt.Sprintf("ipxe/%s/snp.efi", string(arch))
	}

	nodeInfo := provision.NodeInfo{
		ID:   pidPath,
		UUID: nodeUUID,
		Name: nodeReq.Name,
		Type: nodeReq.Type,

		NanoCPUs: nodeReq.NanoCPUs,
		Memory:   nodeReq.Memory,
		DiskSize: nodeReq.Disks[0].Size,

		IPs: nodeReq.IPs,

		APIPort: apiBind.Port,
	}

	if opts.TPM1_2Enabled || opts.TPM2Enabled {
		tpmConfig, tpm2Err := p.createVirtualTPMState(state, nodeReq.Name, opts.TPM2Enabled)
		if tpm2Err != nil {
			return provision.NodeInfo{}, tpm2Err
		}

		launchConfig.TPMConfig = tpmConfig
		nodeInfo.TPMStateDir = tpmConfig.StateDir
	}

	if !clusterReq.Network.DHCPSkipHostname {
		launchConfig.Network.Hostname = nodeReq.Name
	}

	if !nodeReq.PXEBooted && launchConfig.IPXEBootFileName == "" {
		launchConfig.KernelImagePath = strings.ReplaceAll(clusterReq.KernelPath, constants.ArchVariable, opts.TargetArch)
		launchConfig.InitrdPath = strings.ReplaceAll(clusterReq.InitramfsPath, constants.ArchVariable, opts.TargetArch)
		launchConfig.ISOPath = strings.ReplaceAll(clusterReq.ISOPath, constants.ArchVariable, opts.TargetArch)
		launchConfig.USBPath = strings.ReplaceAll(clusterReq.USBPath, constants.ArchVariable, opts.TargetArch)
		launchConfig.UKIPath = strings.ReplaceAll(clusterReq.UKIPath, constants.ArchVariable, opts.TargetArch)
	}

	launchConfig.StatePath, err = state.StatePath()
	if err != nil {
		return provision.NodeInfo{}, err
	}

	launchConfigFile, err := os.Create(state.GetRelativePath(fmt.Sprintf("%s.config", nodeReq.Name)))
	if err != nil {
		return provision.NodeInfo{}, err
	}

	if err = json.NewEncoder(launchConfigFile).Encode(&launchConfig); err != nil {
		return provision.NodeInfo{}, err
	}

	if _, err = launchConfigFile.Seek(0, io.SeekStart); err != nil {
		return provision.NodeInfo{}, err
	}

	defer launchConfigFile.Close() //nolint:errcheck

	cmd := exec.Command(clusterReq.SelfExecutable, "qemu-launch")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = launchConfigFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // daemonize
	}

	if err = cmd.Start(); err != nil {
		return provision.NodeInfo{}, err
	}

	if err = os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), os.ModePerm); err != nil {
		return provision.NodeInfo{}, fmt.Errorf("error writing PID file: %w", err)
	}

	// no need to wait here, as cmd has all the Stdin/out/err via *os.File

	return nodeInfo, nil
}

func (p *provisioner) createNodes(state *vm.State, clusterReq provision.ClusterRequest, nodeReqs []provision.NodeRequest, opts *provision.Options) ([]provision.NodeInfo, error) {
	errCh := make(chan error)
	nodeCh := make(chan provision.NodeInfo, len(nodeReqs))

	for _, nodeReq := range nodeReqs {
		go func(nodeReq provision.NodeRequest) {
			nodeInfo, err := p.createNode(state, clusterReq, nodeReq, opts)
			if err == nil {
				nodeCh <- nodeInfo
			}

			errCh <- err
		}(nodeReq)
	}

	var multiErr *multierror.Error

	for range nodeReqs {
		multiErr = multierror.Append(multiErr, <-errCh)
	}

	close(nodeCh)

	nodesInfo := make([]provision.NodeInfo, 0, len(nodeReqs))

	for nodeInfo := range nodeCh {
		nodesInfo = append(nodesInfo, nodeInfo)
	}

	return nodesInfo, multiErr.ErrorOrNil()
}

func (p *provisioner) populateSystemDisk(disks []string, clusterReq provision.ClusterRequest) error {
	if len(disks) > 0 && clusterReq.DiskImagePath != "" {
		if err := p.handleOptionalZSTDDiskImage(disks[0], clusterReq.DiskImagePath); err != nil {
			return err
		}
	}

	return nil
}

func (p *provisioner) handleOptionalZSTDDiskImage(provisionerDisk, diskImagePath string) error {
	image, err := os.Open(diskImagePath)
	if err != nil {
		return err
	}

	defer image.Close() //nolint:errcheck

	disk, err := os.OpenFile(provisionerDisk, os.O_RDWR, 0o755)
	if err != nil {
		return err
	}

	defer disk.Close() //nolint:errcheck

	if strings.HasSuffix(diskImagePath, ".zst") {
		zstdReader, err := zstd.NewReader(image)
		if err != nil {
			return err
		}

		defer zstdReader.Close() //nolint:errcheck

		_, err = io.Copy(disk, zstdReader)

		return err
	}

	_, err = io.Copy(disk, image)

	return err
}

func (p *provisioner) createMetalConfigISO(state *vm.State, nodeName, config string) (string, error) {
	isoPath := state.GetRelativePath(nodeName + "-metal-config.iso")

	tmpDir, err := os.MkdirTemp("", "talos-metal-config-iso")
	if err != nil {
		return "", err
	}

	defer os.RemoveAll(tmpDir) //nolint:errcheck

	if err = os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(config), 0o644); err != nil {
		return "", err
	}

	_, err = cmd.Run("mkisofs", "-joliet", "-rock", "-volid", "metal-iso", "-output", isoPath, tmpDir)
	if err != nil {
		return "", err
	}

	return isoPath, nil
}

func getLaunchNetworkConfigBase(state *vm.State, clusterReq provision.ClusterRequest, nodeReq provision.NodeRequest) networkConfigBase {
	return networkConfigBase{
		BridgeName:   state.BridgeName,
		CIDRs:        clusterReq.Network.CIDRs,
		IPs:          nodeReq.IPs,
		GatewayAddrs: clusterReq.Network.GatewayAddrs,
		MTU:          clusterReq.Network.MTU,
		Nameservers:  clusterReq.Network.Nameservers,
	}
}

// getRandomMacAddress generates a random local MAC address
// https://stackoverflow.com/a/21027407/10938317
func getRandomMacAddress() string {
	const (
		local     = 0b10
		multicast = 0b1
	)

	buf := make([]byte, 6)
	rand.Read(buf) //nolint:errcheck
	// clear multicast bit (&^), ensure local bit (|)
	buf[0] = buf[0]&^multicast | local

	return net.HardwareAddr(buf).String()
}
