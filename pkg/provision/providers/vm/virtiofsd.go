// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"context"
	"errors"
	"os"
	"os/exec"

	"github.com/siderolabs/talos/pkg/provision"
)

const (
	virtiofsdPid = "virtiofsd.pid"
	virtiofsdLog = "virtiofsd.log"
)

// FindVirtiofsd tries to find the virtiofsd binary in common locations.
func (p *Provisioner) FindVirtiofsd() (string, error) {
	return p.findVirtiofsd()
}

// Virtiofsd starts the Virtiofsd server.
func Virtiofsd(ctx context.Context, virtiofsdBin, share, socket string) error {
	if virtiofsdBin == "" {
		return errors.New("virtiofsd binary path is empty")
	}

	args := []string{
		"--shared-dir", share,
		"--socket-path", socket,
		"--announce-submounts",
		"--inode-file-handles", "mandatory",
	}

	cmd := exec.CommandContext(ctx, virtiofsdBin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// CreateVirtiofsd creates the Virtiofsd server.
func (p *Provisioner) CreateVirtiofsd(state *State, clusterReq provision.ClusterRequest, virtiofdPath string) error {
	return p.startVirtiofsd(state, clusterReq, virtiofdPath)
}

// DestroyVirtiofsd destoys Virtiofsd server.
func (p *Provisioner) DestroyVirtiofsd(state *State) error {
	return p.stopVirtiofsd(state)
}
