// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package firecracker

import (
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	models "github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"github.com/hashicorp/go-multierror"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/pkg/provision"
)

func (p *provisioner) createDisk(state *state, nodeReq provision.NodeRequest) (diskPath string, err error) {
	diskPath = filepath.Join(state.statePath, fmt.Sprintf("%s.disk", nodeReq.Name))

	var diskF *os.File

	diskF, err = os.Create(diskPath)
	if err != nil {
		return
	}

	defer diskF.Close() //nolint: errcheck

	err = diskF.Truncate(nodeReq.DiskSize)

	return
}

func (p *provisioner) createNodes(state *state, clusterReq provision.ClusterRequest, nodeReqs []provision.NodeRequest, opts *provision.Options) ([]provision.NodeInfo, error) {
	errCh := make(chan error)
	nodeCh := make(chan provision.NodeInfo, len(nodeReqs))

	for _, nodeReq := range nodeReqs {
		go func(nodeReq provision.NodeRequest) {
			nodeInfo, err := p.createNode(state, clusterReq, nodeReq, opts)
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

//nolint: gocyclo
func (p *provisioner) createNode(state *state, clusterReq provision.ClusterRequest, nodeReq provision.NodeRequest, opts *provision.Options) (provision.NodeInfo, error) {
	socketPath := filepath.Join(state.statePath, fmt.Sprintf("%s.sock", nodeReq.Name))
	pidPath := filepath.Join(state.statePath, fmt.Sprintf("%s.pid", nodeReq.Name))

	vcpuCount := int64(math.RoundToEven(float64(nodeReq.NanoCPUs) / 1000 / 1000 / 1000))
	if vcpuCount < 2 {
		vcpuCount = 1
	}

	memSize := nodeReq.Memory / 1024 / 1024

	diskPath, err := p.createDisk(state, nodeReq)
	if err != nil {
		return provision.NodeInfo{}, err
	}

	cmdline := procfs.NewDefaultCmdline()

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
	cmdline.Append("talos.config", "{TALOS_CONFIG_URL}") // to be patched by launcher
	cmdline.Append("talos.hostname", nodeReq.Name)

	ones, _ := clusterReq.Network.CIDR.Mask.Size()

	cfg := firecracker.Config{
		SocketPath:      socketPath,
		KernelImagePath: clusterReq.KernelPath,
		KernelArgs:      cmdline.String(),
		InitrdPath:      clusterReq.InitramfsPath,
		ForwardSignals:  []os.Signal{}, // don't forward any signals
		MachineCfg: models.MachineConfiguration{
			HtEnabled:  firecracker.Bool(false),
			VcpuCount:  firecracker.Int64(vcpuCount),
			MemSizeMib: firecracker.Int64(memSize),
		},
		NetworkInterfaces: firecracker.NetworkInterfaces{
			firecracker.NetworkInterface{
				CNIConfiguration: &firecracker.CNIConfiguration{
					BinPath:       clusterReq.Network.CNI.BinPath,
					ConfDir:       clusterReq.Network.CNI.ConfDir,
					CacheDir:      clusterReq.Network.CNI.CacheDir,
					NetworkConfig: state.vmCNIConfig,
					Args: [][2]string{
						{"IP", fmt.Sprintf("%s/%d", nodeReq.IP, ones)},
						{"GATEWAY", clusterReq.Network.GatewayAddr.String()},
					},
					IfName:   "veth0",
					VMIfName: "eth0",
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

	logFile, err := os.OpenFile(filepath.Join(state.statePath, fmt.Sprintf("%s.log", nodeReq.Name)), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return provision.NodeInfo{}, err
	}

	defer logFile.Close() //nolint: errcheck

	nodeConfig, err := nodeReq.Config.String()
	if err != nil {
		return provision.NodeInfo{}, err
	}

	launchConfig := LaunchConfig{
		FirecrackerConfig:   cfg,
		Config:              nodeConfig,
		GatewayAddr:         clusterReq.Network.GatewayAddr.String(),
		BootloaderEmulation: opts.BootloaderEmulation,
	}

	launchConfigFile, err := os.Create(filepath.Join(state.statePath, fmt.Sprintf("%s.config", nodeReq.Name)))
	if err != nil {
		return provision.NodeInfo{}, err
	}

	if err = json.NewEncoder(launchConfigFile).Encode(&launchConfig); err != nil {
		return provision.NodeInfo{}, err
	}

	if _, err = launchConfigFile.Seek(0, io.SeekStart); err != nil {
		return provision.NodeInfo{}, err
	}

	defer launchConfigFile.Close() //nolint: errcheck

	cmd := exec.Command(clusterReq.SelfExecutable, "firecracker-launch")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = launchConfigFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // daemonize
	}

	if err = cmd.Start(); err != nil {
		return provision.NodeInfo{}, err
	}

	if err = ioutil.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), os.ModePerm); err != nil {
		return provision.NodeInfo{}, fmt.Errorf("error writing PID file: %w", err)
	}

	// no need to wait here, as cmd has all the Stdin/out/err via *os.File

	nodeInfo := provision.NodeInfo{
		ID:   pidPath,
		Name: nodeReq.Name,
		Type: nodeReq.Config.Machine().Type(),

		NanoCPUs: nodeReq.NanoCPUs,
		Memory:   nodeReq.Memory,
		DiskSize: nodeReq.DiskSize,

		PrivateIP: nodeReq.IP,
	}

	return nodeInfo, nil
}

func (p *provisioner) destroyNodes(cluster provision.ClusterInfo, options *provision.Options) error {
	errCh := make(chan error)

	for _, node := range cluster.Nodes {
		go func(node provision.NodeInfo) {
			fmt.Fprintln(options.LogWriter, "stopping VM", node.Name)

			errCh <- p.destroyNode(node)
		}(node)
	}

	var multiErr *multierror.Error

	for range cluster.Nodes {
		multiErr = multierror.Append(multiErr, <-errCh)
	}

	return multiErr.ErrorOrNil()
}

func (p *provisioner) destroyNode(node provision.NodeInfo) error {
	return stopProcessByPidfile(node.ID) // node.ID stores PID path for control process
}
