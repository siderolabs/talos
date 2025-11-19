// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/siderolabs/talos/pkg/provision"
)

func (p *Provisioner) findVirtiofsd() (string, error) {
	virtiofsdPaths := []string{
		"virtiofsd",
		"/usr/libexec/virtiofsd",
	}

	for _, p := range virtiofsdPaths {
		if full, err := exec.LookPath(p); err == nil {
			return full, nil
		}
	}

	return "", fmt.Errorf("virtiofsd not found in paths: %v", virtiofsdPaths)
}

func (p *Provisioner) startVirtiofsd(state *State, clusterReq provision.ClusterRequest, virtiofdPath string) error {
	pidPath := state.GetRelativePath(virtiofsdPid)

	logFile, err := os.OpenFile(state.GetRelativePath(virtiofsdLog), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return err
	}

	defer logFile.Close() //nolint:errcheck

	args := []string{
		"virtiofsd-launch",
		"--bin", virtiofdPath,
		// TODO: add virtiofd configs
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
		return fmt.Errorf("error writing virtiofsd PID file: %w", err)
	}

	return nil
}

func (p *Provisioner) stopVirtiofsd(state *State) error {
	pidPath := state.GetRelativePath(virtiofsdPid)

	return StopProcessByPidfile(pidPath)
}
