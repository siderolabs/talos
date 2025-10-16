// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package provision provides abstract definitions for Talos cluster provisioners.
package provision

import (
	"context"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

// Provisioner is an interface each provisioner should implement.
type Provisioner interface {
	Create(context.Context, ClusterRequest, ...Option) (Cluster, error)
	Destroy(context.Context, Cluster, ...Option) error

	Reflect(ctx context.Context, clusterName, stateDirectory string) (Cluster, error)

	GenOptions(NetworkRequest, *config.VersionContract) ([]generate.Option, []bundle.Option)

	GetInClusterKubernetesControlPlaneEndpoint(req NetworkRequest, controlPlanePort int) string
	GetExternalKubernetesControlPlaneEndpoint(req NetworkRequest, controlPlanePort int) string
	GetTalosAPIEndpoints(NetworkRequest) []string

	GetFirstInterface() v1alpha1.IfaceSelector

	Close() error

	UserDiskName(index int) string
}
