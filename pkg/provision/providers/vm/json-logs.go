// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/siderolabs/talos/pkg/provision"
)

const (
	jsonLogsPid = "json-logs.pid"
	jsonLogsLog = "json-logs.log"
)

// CreateJSONLogs creates JSON logs server.
func (p *Provisioner) CreateJSONLogs(state *provision.State, clusterReq provision.ClusterRequest, options provision.Options) error {
	pidPath := state.GetRelativePath(jsonLogsPid)

	logFile, err := os.OpenFile(state.GetRelativePath(jsonLogsLog), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return err
	}

	defer logFile.Close() //nolint:errcheck

	key := make([]byte, 32)
	if _, err = io.ReadFull(rand.Reader, key); err != nil {
		return err
	}

	args := []string{
		"json-logs-launch",
		"--addr", options.JSONLogsEndpoint,
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
		return fmt.Errorf("error writing LB PID file: %w", err)
	}

	return nil
}

// DestroyJSONLogs destroys JSON logs server.
func (p *Provisioner) DestroyJSONLogs(state *provision.State) error {
	pidPath := state.GetRelativePath(jsonLogsPid)

	return StopProcessByPidfile(pidPath)
}
