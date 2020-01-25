// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package provision provides abstract definitions for Talos cluster provisioners.
package provision

import (
	"context"

	"github.com/talos-systems/talos/pkg/config/types/v1alpha1/generate"
)

// Provisioner is an interface each provisioner should implement.
type Provisioner interface {
	Create(context.Context, ClusterRequest, ...Option) (Cluster, error)
	Destroy(context.Context, Cluster, ...Option) error

	Reflect(ctx context.Context, clusterName, stateDirectory string) (Cluster, error)

	GenOptions(NetworkRequest) []generate.GenOption

	Close() error
}
