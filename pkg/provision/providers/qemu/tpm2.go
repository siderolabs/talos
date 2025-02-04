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

func (p *Provisioner) createVirtualTPM2State(state *vm.State, nodeName string) (tpm2Config, error) {
	tpm2StateDir := state.GetRelativePath(fmt.Sprintf("%s-tpm2", nodeName))

	if err := os.MkdirAll(tpm2StateDir, 0o755); err != nil {
		return tpm2Config{}, err
	}

	return tpm2Config{
		NodeName: nodeName,
		StateDir: tpm2StateDir,
	}, nil
}

func (p *Provisioner) destroyVirtualTPM2s(cluster provision.ClusterInfo) error {
	errCh := make(chan error)

	nodes := append([]provision.NodeInfo{}, cluster.Nodes...)

	for _, node := range nodes {
		if node.TPM2StateDir == "" {
			continue
		}

		tpm2PidPath := filepath.Join(node.TPM2StateDir, "swtpm.pid")

		go func() {
			errCh <- p.destroyVirtualTPM2(tpm2PidPath)
		}()
	}

	var multiErr *multierror.Error

	for _, node := range nodes {
		if node.TPM2StateDir == "" {
			continue
		}

		multiErr = multierror.Append(multiErr, <-errCh)
	}

	return multiErr.ErrorOrNil()
}

func (p *Provisioner) destroyVirtualTPM2(pid string) error {
	return vm.StopProcessByPidfile(pid)
}
