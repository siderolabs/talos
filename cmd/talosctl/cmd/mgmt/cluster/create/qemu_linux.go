// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"context"
	"fmt"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clustermaker"
	"github.com/siderolabs/talos/pkg/provision/providers/qemu"
)

func createQemuCluster(ctx context.Context, cOps clustermaker.Options, qOps qemuOps) error {
	provisioner, err := qemu.NewProvisioner(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if err := provisioner.Close(); err != nil {
			fmt.Printf("failed to close qemu provisioner: %v", err)
		}
	}()

	cm, err := getQemuClusterMaker(qOps, cOps, provisioner)
	if err != nil {
		return err
	}

	return _createQemuCluster(ctx, qOps, cOps, provisioner, cm)
}
