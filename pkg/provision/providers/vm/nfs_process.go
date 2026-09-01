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

	"github.com/siderolabs/talos/pkg/provision"
)

const (
	nfsPIDFile = "nfs.pid"
	nfsLogFile = "nfs.log"
)

// CreateNFS starts the development NFS server.
func (p *Provisioner) CreateNFS(state *provision.State, clusterReq provision.ClusterRequest) error {
	if len(clusterReq.Network.GatewayAddrs) == 0 {
		return errors.New("NFS server requires a bridge gateway address")
	}

	gateway := clusterReq.Network.GatewayAddrs[0]

	logFile, err := os.OpenFile(state.GetRelativePath(nfsLogFile), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return err
	}
	defer logFile.Close() //nolint:errcheck

	cmd := exec.Command( //nolint:noctx // runs in background
		clusterReq.SelfExecutable,
		"nfs-launch",
		"--bind-address", gateway.String(),
		"--port", "2049",
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	setDetachedProcess(cmd)

	if err = cmd.Start(); err != nil {
		return err
	}

	if err = os.WriteFile(state.GetRelativePath(nfsPIDFile), []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
		return errors.Join(
			fmt.Errorf("error writing NFS PID file: %w", err),
			stopStartedProcess(cmd),
		)
	}

	return nil
}

func stopStartedProcess(cmd *exec.Cmd) error {
	killErr := cmd.Process.Kill()
	waitErr := cmd.Wait()

	if exitErr, ok := errors.AsType[*exec.ExitError](waitErr); ok && exitErr != nil {
		waitErr = nil
	}

	return errors.Join(killErr, waitErr)
}

// DestroyNFS stops the development NFS server.
func (p *Provisioner) DestroyNFS(state *provision.State) error {
	return StopProcessByPidfile(state.GetRelativePath(nfsPIDFile))
}
