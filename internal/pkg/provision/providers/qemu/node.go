// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	multierror "github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/internal/pkg/provision"
	"github.com/talos-systems/talos/internal/pkg/provision/providers/vm"

	"github.com/talos-systems/go-procfs/procfs"
)

func (p *provisioner) createNode(state *vm.State, clusterReq provision.ClusterRequest, nodeReq provision.NodeRequest) (provision.NodeInfo, error) {
	pidPath := state.GetRelativePath(fmt.Sprintf("%s.pid", nodeReq.Name))

	vcpuCount := int64(math.RoundToEven(float64(nodeReq.NanoCPUs) / 1000 / 1000 / 1000))
	if vcpuCount < 2 {
		vcpuCount = 1
	}

	memSize := nodeReq.Memory / 1024 / 1024

	diskPath, err := p.CreateDisk(state, nodeReq)
	if err != nil {
		return provision.NodeInfo{}, err
	}

	logFile, err := os.OpenFile(state.GetRelativePath(fmt.Sprintf("%s.log", nodeReq.Name)), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return provision.NodeInfo{}, err
	}

	defer logFile.Close() //nolint: errcheck

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

	launchConfig := LaunchConfig{
		QemuExecutable:  "qemu-system-x86_64",
		DiskPath:        diskPath,
		VCPUCount:       vcpuCount,
		MemSize:         memSize,
		KernelImagePath: clusterReq.CompressedKernelPath,
		KernelArgs:      cmdline.String(),
		InitrdPath:      clusterReq.InitramfsPath,
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

	defer launchConfigFile.Close() //nolint: errcheck

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

func (p *provisioner) createNodes(state *vm.State, clusterReq provision.ClusterRequest, nodeReqs []provision.NodeRequest) ([]provision.NodeInfo, error) {
	errCh := make(chan error)
	nodeCh := make(chan provision.NodeInfo, len(nodeReqs))

	for _, nodeReq := range nodeReqs {
		go func(nodeReq provision.NodeRequest) {
			nodeInfo, err := p.createNode(state, clusterReq, nodeReq)
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
