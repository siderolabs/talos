// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-multierror"

	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

func (p *provisioner) createVirtualTPMState(state *vm.State, nodeName string, tpm2Enabled bool) (tpmConfig, error) {
	tpmStateDir := state.GetRelativePath(fmt.Sprintf("%s-tpm", nodeName))

	if err := os.MkdirAll(tpmStateDir, 0o755); err != nil {
		return tpmConfig{}, err
	}

	return tpmConfig{
		NodeName: nodeName,
		StateDir: tpmStateDir,

		TPM2: tpm2Enabled,
	}, nil
}

func (p *provisioner) destroyVirtualTPMs(cluster provision.ClusterInfo) error {
	errCh := make(chan error)

	nodes := append([]provision.NodeInfo{}, cluster.Nodes...)

	for _, node := range nodes {
		if node.TPMStateDir == "" {
			continue
		}

		tpm2PidPath := filepath.Join(node.TPMStateDir, "swtpm.pid")

		go func() {
			errCh <- p.destroyVirtualTPM(tpm2PidPath)
		}()
	}

	var multiErr *multierror.Error

	for _, node := range nodes {
		if node.TPMStateDir == "" {
			continue
		}

		multiErr = multierror.Append(multiErr, <-errCh)
	}

	return multiErr.ErrorOrNil()
}

func (p *provisioner) destroyVirtualTPM(pid string) error {
	return vm.StopProcessByPidfile(pid)
}
