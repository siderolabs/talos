// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package storage provides storage virtualization configuration documents.
package storage

//go:generate go tool github.com/siderolabs/talos/tools/docgen -output storage_doc.go storage.go lvm_volume_group_config.go lvm_logical_volume_config.go raid_array_config.go

//go:generate go tool github.com/siderolabs/deep-copy -type LVMVolumeGroupConfigV1Alpha1 -type LVMLogicalVolumeConfigV1Alpha1 -type RAIDArrayConfigV1Alpha1 -pointer-receiver -header-file ../../../../../hack/boilerplate.txt -o deep_copy.generated.go .
