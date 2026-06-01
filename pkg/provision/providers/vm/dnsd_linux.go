// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/provision"
)

func (p *Provisioner) startDNSd(state *State, clusterReq provision.ClusterRequest) error {
	pidPath := state.GetRelativePath(dnsPid)

	logFile, err := os.OpenFile(state.GetRelativePath(dnsLog), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return err
	}

	defer logFile.Close() //nolint:errcheck

	gatewayAddrs := xslices.Map(clusterReq.Network.GatewayAddrs, netip.Addr.String)

	args := []string{
		"dnsd-launch",
		"--addr", strings.Join(gatewayAddrs, ","),
		"--resolv-conf", "/etc/resolv.conf",
	}

	cmd := exec.Command(clusterReq.SelfExecutable, args...) //nolint:noctx // runs in background
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // daemonize
	}

	if err = cmd.Start(); err != nil {
		return err
	}

	if err = os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), os.ModePerm); err != nil {
		return fmt.Errorf("error writing dns PID file: %w", err)
	}

	return nil
}

func (p *Provisioner) stopDNSd(state *State) error {
	pidPath := state.GetRelativePath(dnsPid)

	return StopProcessByPidfile(pidPath)
}
