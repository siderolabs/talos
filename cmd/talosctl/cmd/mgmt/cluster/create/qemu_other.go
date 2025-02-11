// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !linux

package create

import (
	"context"
	"errors"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clustermaker"
)

func createQemuCluster(_ context.Context, _ clustermaker.Options, _ qemuOps) error {
	return errors.New("Qemu is only supported on linux")
}
