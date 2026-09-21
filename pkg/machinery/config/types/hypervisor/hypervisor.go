// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package hypervisor provides hypervisor configuration documents.
package hypervisor

//go:generate go tool github.com/siderolabs/talos/tools/docgen -output hypervisor_doc.go hypervisor.go content_library_config.go virtual_machine_config.go virtual_machine_disk.go

//go:generate go tool github.com/siderolabs/deep-copy -type ContentLibraryConfigV1Alpha1 -type VirtualMachineConfigV1Alpha1 -pointer-receiver -header-file ../../../../../hack/boilerplate.txt -o deep_copy.generated.go .
