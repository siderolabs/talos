// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package containers provides resources for containers declared via ContainerConfig.
package containers

// These containers are run directly by Talos, without Kubernetes and without registering a Talos service.

import "github.com/cosi-project/runtime/pkg/resource"

//go:generate go tool github.com/siderolabs/deep-copy -type ContainerSpecSpec -type ContainerImageStatusSpec -type ContainerMountStatusSpec -type ContainerInstanceSpecSpec -type ContainerInstanceStatusSpec -type ContainerLifecycleSpec -type ContainerStatusSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

//go:generate go tool github.com/dmarkham/enumer -type=ContainerImagePhase,ContainerInstancePhase,ContainerState,ContainerHealth -linecomment -text

// NamespaceName contains resources for Talos-managed containers.
const NamespaceName resource.Namespace = "containers"
