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
func (p *Provisioner) CreateLoadBalancer(state *provision.State, clusterReq provision.ClusterRequest) error {
	controlPlaneIPs := xslices.Map(clusterReq.Nodes.ControlPlaneNodes(),
		func(req provision.NodeRequest) string { return req.IPs[0].String() })

	state.LoadBalancerConfig = &provision.LoadBalancerConfig{
		BindAddress: GetLbBindIP(clusterReq.Network.GatewayAddrs[0]),
		Upstreams:   controlPlaneIPs,
		Ports:       clusterReq.Network.LoadBalancerPorts,
	}
	state.SelfExecutable = clusterReq.SelfExecutable

	return p.StartLoadBalancer(state)
}

// DestroyLoadBalancer destroys load balancer.
func (p *Provisioner) DestroyLoadBalancer(state *provision.State) error {
	pidPath := state.GetRelativePath(lbPid)

	return StopProcessByPidfile(pidPath)
}

// StartLoadBalancer starts the load balancer if not already running, using saved state config.
func (p *Provisioner) StartLoadBalancer(state *provision.State) error {
	pidPath := state.GetRelativePath(lbPid)

	if IsProcessRunning(pidPath) {
		return nil
	}

	if state.LoadBalancerConfig == nil {
		return fmt.Errorf("no load balancer config in state; cluster was created with older talosctl, please destroy and recreate")
	}

	if state.SelfExecutable == "" {
		return fmt.Errorf("no self executable path in state; cluster was created with older talosctl, please destroy and recreate")
	}

	logFile, err := os.OpenFile(state.GetRelativePath(lbLog), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return err
	}

	defer logFile.Close() //nolint:errcheck

	ports := xslices.Map(state.LoadBalancerConfig.Ports, strconv.Itoa)

	args := []string{
		"loadbalancer-launch",
		"--loadbalancer-addr", state.LoadBalancerConfig.BindAddress,
		"--loadbalancer-upstreams", strings.Join(state.LoadBalancerConfig.Upstreams, ","),
	}

	if len(ports) > 0 {
		args = append(args, "--loadbalancer-ports", strings.Join(ports, ","))
	}

	cmd := exec.Command(state.SelfExecutable, args...) //nolint:noctx // runs in background
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
