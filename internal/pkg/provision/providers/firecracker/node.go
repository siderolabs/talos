// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package firecracker

import (
	"context"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	models "github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/provision"
)

func (p *provisioner) createDisk(state *state, nodeReq provision.NodeRequest) (diskPath string, err error) {
	diskPath = filepath.Join(state.tempDir, fmt.Sprintf("%s.disk", nodeReq.Name))

	var diskF *os.File

	diskF, err = os.Create(diskPath)
	if err != nil {
		return
	}

	defer diskF.Close() //nolint: errcheck

	err = diskF.Truncate(nodeReq.DiskSize)

	return
}

func (p *provisioner) createNodes(ctx context.Context, state *state, clusterReq provision.ClusterRequest, nodeReqs []provision.NodeRequest) ([]provision.NodeInfo, error) {
	errCh := make(chan error)
	nodeCh := make(chan provision.NodeInfo, len(nodeReqs))

	for _, nodeReq := range nodeReqs {
		go func(nodeReq provision.NodeRequest) {
			nodeInfo, err := p.createNode(ctx, state, clusterReq, nodeReq)
			errCh <- err

			if err == nil {
				nodeCh <- nodeInfo
			}
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

func (p *provisioner) createNode(ctx context.Context, state *state, clusterReq provision.ClusterRequest, nodeReq provision.NodeRequest) (provision.NodeInfo, error) {
	socketPath := filepath.Join(state.tempDir, fmt.Sprintf("%s.sock", nodeReq.Name))

	vcpuCount := int64(math.RoundToEven(float64(nodeReq.NanoCPUs) / 1000 / 1000 / 1000))
	if vcpuCount < 2 {
		vcpuCount = 1
	}

	memSize := nodeReq.Memory / 1024 / 1024

	diskPath, err := p.createDisk(state, nodeReq)
	if err != nil {
		return provision.NodeInfo{}, err
	}

	cmdline := kernel.NewDefaultCmdline()

	// required to get kernel console
	cmdline.Append("console", "ttyS0")

	// reboot configuration
	cmdline.Append("reboot", "k")
	cmdline.Append("panic", "1")

	// disable stuff we don't need
	cmdline.Append("pci", "off")
	cmdline.Append("acpi", "off")
	cmdline.Append("i8042.noaux", "")

	// Talos config
	cmdline.Append("talos.platform", "metal")
	cmdline.Append("talos.config", fmt.Sprintf("%s%s.yaml", state.baseConfigURL, nodeReq.Name))

	// networking
	cmdline.Append("ip", fmt.Sprintf(
		"%s::%s:%s:%s:eth0:off",
		nodeReq.IP,
		clusterReq.Network.GatewayAddr,
		net.IP(clusterReq.Network.CIDR.Mask),
		nodeReq.Name))

	ones, bits := clusterReq.Network.CIDR.IP.DefaultMask().Size()

	cfg := firecracker.Config{
		DisableValidation: true, // TODO: enable when firecracker Go SDK is fixed
		SocketPath:        socketPath,
		KernelImagePath:   clusterReq.KernelPath,
		KernelArgs:        cmdline.String(),
		InitrdPath:        clusterReq.InitramfsPath,
		ForwardSignals:    []os.Signal{}, // don't forward any signals
		MachineCfg: models.MachineConfiguration{
			HtEnabled:  firecracker.Bool(false),
			VcpuCount:  firecracker.Int64(vcpuCount),
			MemSizeMib: firecracker.Int64(memSize),
		},
		NetworkInterfaces: firecracker.NetworkInterfaces{
			firecracker.NetworkInterface{
				CNIConfiguration: &firecracker.CNIConfiguration{
					BinPath:     clusterReq.Network.CNI.BinPath,
					ConfDir:     clusterReq.Network.CNI.ConfDir,
					CacheDir:    clusterReq.Network.CNI.CacheDir,
					NetworkName: clusterReq.Network.Name,
					Args: [][2]string{
						{"IP", fmt.Sprintf("%s/%d", nodeReq.IP, bits-ones)},
						{"GATEWAY", clusterReq.Network.GatewayAddr.String()},
					},
					IfName: "veth0",
				},
			},
		},
		Drives: []models.Drive{
			{
				DriveID:      firecracker.String("disk"),
				IsRootDevice: firecracker.Bool(false),
				IsReadOnly:   firecracker.Bool(false),
				PathOnHost:   firecracker.String(diskPath),
			},
		},
	}

	defer os.Remove(cfg.SocketPath) //nolint: errcheck

	logFile, err := os.OpenFile(filepath.Join(state.tempDir, fmt.Sprintf("%s.log", nodeReq.Name)), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return provision.NodeInfo{}, err
	}

	// defer logFile.Close() //nolint: errcheck

	// TODO: this loop is to boot VM for the first time, let installer do its work
	//       handle reboot and then leave it running
	//       this is going to change with control process to the non-hacky way
	for i := 0; i < 2; i++ {
		cmd := firecracker.VMCommandBuilder{}.
			WithBin("firecracker").
			WithSocketPath(socketPath).
			WithStdout(logFile).
			WithStderr(logFile).
			Build(ctx)

		m, err := firecracker.NewMachine(ctx, cfg, firecracker.WithProcessRunner(cmd))
		if err != nil {
			return provision.NodeInfo{}, fmt.Errorf("failed to create new machine: %w", err)
		}

		if err := m.Start(ctx); err != nil {
			return provision.NodeInfo{}, fmt.Errorf("failed to initialize machine: %w", err)
		}

		if i == 0 {
			// wait for VMM to execute
			if err := m.Wait(ctx); err != nil {
				return provision.NodeInfo{}, err
			}
		}
	}

	nodeInfo := provision.NodeInfo{
		ID:   socketPath,
		Name: nodeReq.Name,
		Type: nodeReq.Config.Machine().Type(),

		PrivateIP: nodeReq.IP,
	}

	return nodeInfo, nil
}
