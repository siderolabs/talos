// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package hypervisor provides resources for the Talos hypervisor.
package hypervisor

import "github.com/cosi-project/runtime/pkg/resource"

//go:generate go tool github.com/siderolabs/deep-copy -type ContentLibraryStatusSpec -type VirtualMachineSpecSpec -type VirtualMachineDomainSpecSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

// NamespaceName contains resources of the Talos hypervisor.
const NamespaceName resource.Namespace = "hypervisor"
