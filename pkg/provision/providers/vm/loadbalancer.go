// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/provision"
)

const (
	lbPid = "lb.pid"
	lbLog = "lb.log"
)

// CreateLoadBalancer creates load balancer.
func (p *Provisioner) CreateLoadBalancer(state *State, clusterReq provision.ClusterRequest) error {
	pidPath := state.GetRelativePath(lbPid)

	logFile, err := os.OpenFile(state.GetRelativePath(lbLog), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return err
	}

	defer logFile.Close() //nolint:errcheck

	controlPlaneIPs := xslices.Map(clusterReq.Nodes.ControlPlaneNodes(), func(req provision.NodeRequest) string { return req.IPs[0].String() })
	ports := xslices.Map(clusterReq.Network.LoadBalancerPorts, strconv.Itoa)

	args := []string{
		"loadbalancer-launch",
		"--loadbalancer-addr", getLbBindIP(clusterReq.Network.GatewayAddrs[0]),
		"--loadbalancer-upstreams", strings.Join(controlPlaneIPs, ","),
	}

	if len(ports) > 0 {
		args = append(args, "--loadbalancer-ports", strings.Join(ports, ","))
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

	if err = os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), os.ModePerm); err != nil {
		return fmt.Errorf("error writing LB PID file: %w", err)
	}

	return nil
}

// DestroyLoadBalancer destroys load balancer.
func (p *Provisioner) DestroyLoadBalancer(state *State) error {
	pidPath := state.GetRelativePath(lbPid)

	return StopProcessByPidfile(pidPath)
}
