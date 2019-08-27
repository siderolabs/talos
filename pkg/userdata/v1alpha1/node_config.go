/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package v1alpha1

// NodeConfig holds the full representation of the node config
type NodeConfig struct {
	Version string
	Machine *MachineConfig
	Cluster *ClusterConfig
}
