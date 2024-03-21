// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/siderolabs/talos/pkg/provision"
)

const (
	siderolinkAgentPid = "siderolink-agent.pid"
	siderolinkAgentLog = "siderolink-agent.log"
)

// CreateSiderolinkAgent creates siderlink agent.
func (p *Provisioner) CreateSiderolinkAgent(state *State, clusterReq provision.ClusterRequest) error {
	pidPath := state.GetRelativePath(siderolinkAgentPid)

	logFile, err := os.OpenFile(state.GetRelativePath(siderolinkAgentLog), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return err
	}

	defer logFile.Close() //nolint:errcheck

	args := []string{
		"siderolink-launch",
		"--sidero-link-join-token", "foo",
		"--sidero-link-wireguard-endpoint", clusterReq.SiderolinkRequest.WireguardEndpoint,
		"--event-sink-endpoint", clusterReq.SiderolinkRequest.SinkEndpoint,
		"--sidero-link-api-endpoint", clusterReq.SiderolinkRequest.APIEndpoint,
		"--log-receiver-endpoint", clusterReq.SiderolinkRequest.LogEndpoint,
	}

	for _, bind := range clusterReq.SiderolinkRequest.SiderolinkBind {
		args = append(args, "--predefined-pair", bind.UUID.String()+"="+bind.Addr.String())
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
		return fmt.Errorf("error writing SA PID file: %w", err)
	}

	return nil
}

// DestroySiderolinkAgent destroys siderolink agent.
func (p *Provisioner) DestroySiderolinkAgent(state *State) error {
	pidPath := state.GetRelativePath(siderolinkAgentPid)

	if _, err := os.Stat(pidPath); errors.Is(err, os.ErrNotExist) {
		// If the pid file does not exist, the process was not started.
		return nil
	}

	return StopProcessByPidfile(pidPath)
}
