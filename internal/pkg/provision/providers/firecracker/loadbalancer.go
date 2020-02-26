// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package firecracker

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/talos-systems/talos/internal/pkg/provision"
)

const (
	lbPid = "lb.pid"
	lbLog = "lb.log"
)

func (p *provisioner) createLoadBalancer(state *state, clusterReq provision.ClusterRequest) error {
	pidPath := filepath.Join(state.statePath, lbPid)

	logFile, err := os.OpenFile(filepath.Join(state.statePath, lbLog), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return err
	}

	defer logFile.Close() //nolint: errcheck

	masterNodes := clusterReq.Nodes.MasterNodes()
	masterIPs := make([]string, len(masterNodes))

	for i := range masterIPs {
		masterIPs[i] = masterNodes[i].IP.String()
	}

	args := []string{
		"loadbalancer-launch",
		"--loadbalancer-addr", clusterReq.Network.GatewayAddr.String(),
		"--loadbalancer-upstreams", strings.Join(masterIPs, ","),
	}

	if clusterReq.Network.LoadBalancer.LimitApidOnlyInitNode {
		args = append(args, "--apid-only-init-node")
	}

	cmd := exec.Command(clusterReq.SelfExecutable, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // daemonize
	}

	if err = cmd.Start(); err != nil {
		return err
	}

	if err = ioutil.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), os.ModePerm); err != nil {
		return fmt.Errorf("error writing LB PID file: %w", err)
	}

	return nil
}

func (p *provisioner) destroyLoadBalancer(state *state) error {
	pidPath := filepath.Join(state.statePath, lbPid)

	return stopProcessByPidfile(pidPath)
}
