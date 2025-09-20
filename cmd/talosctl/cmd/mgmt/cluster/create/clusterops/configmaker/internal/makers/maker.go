// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makers

import "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"

// ConfigMaker helps creating cluster and provision configuration.
type ConfigMaker interface {
	GetClusterConfigs() (clusterops.ClusterConfigs, error)
}
