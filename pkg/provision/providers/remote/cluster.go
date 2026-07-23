// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package remote

import (
	"errors"

	"github.com/siderolabs/talos/pkg/provision"
)

// Cluster is the client-side view of a Cluster created on a remote
// cluster-server. It implements provision.Cluster directly from the JSON
// payload the server returns on Create/Reflect.
type Cluster struct {
	wire wireCluster
}

// Provisioner returns the name of the provisioner that built the cluster on
// the server.
func (c *Cluster) Provisioner() string {
	return c.wire.ProvisionerName
}

// StatePath returns the path to the state directory on the server.
//
// Returns an error if the server did not include a state path (e.g. when
// Reflect returns an empty Cluster).
func (c *Cluster) StatePath() (string, error) {
	if c.wire.StatePath == "" {
		return "", errors.New("remote cluster has no state path")
	}

	return c.wire.StatePath, nil
}

// Info returns the cluster information.
func (c *Cluster) Info() provision.ClusterInfo {
	return c.wire.Info
}
