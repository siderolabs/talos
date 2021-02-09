// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"
	"fmt"

	"github.com/talos-systems/talos/pkg/cluster"
)

// UpgradeProvider are the cluster interfaces required by upgrade process.
type UpgradeProvider interface {
	cluster.ClientProvider
	cluster.K8sProvider
}

// UpgradeTalosManaged the Kubernetes control plane.
func UpgradeTalosManaged(ctx context.Context, cluster UpgradeProvider, options UpgradeOptions) error {
	return fmt.Errorf("not implemented yet")
}
