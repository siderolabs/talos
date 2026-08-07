// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package containers provides resources for containers declared via ContainerConfig.
//
// These containers are run directly by Talos, without Kubernetes and without registering a Talos
// service. The full resource chain, per RFD 41, is:
//
//	ContainerConfig (machine config) -> ContainerSpec -> ContainerInstanceSpec -> ContainerInstanceStatus
//
// with ContainerImageStatus and ContainerMountStatus gating the step from spec to instance, and
// ContainerStatus as the aggregated user-facing surface. This package currently only carries
// ContainerSpec; the remaining resources arrive in follow-up controllers.
package containers

import "github.com/cosi-project/runtime/pkg/resource"

//go:generate go tool github.com/siderolabs/deep-copy -type ContainerSpecSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

// NamespaceName contains resources for Talos-managed containers.
const NamespaceName resource.Namespace = "containers"
