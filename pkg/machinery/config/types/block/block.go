// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package block provides block device and volume configuration documents.
package block

//go:generate go tool github.com/siderolabs/talos/tools/docgen -output block_doc.go block.go encryption.go existing_volume_config.go external_volume_config.go lvm_config.go raw_volume_config.go swap_volume_config.go user_volume_config.go volume_config.go zswap_config.go

//go:generate go tool github.com/siderolabs/deep-copy -type ExistingVolumeConfigV1Alpha1 -type RawVolumeConfigV1Alpha1 -type SwapVolumeConfigV1Alpha1 -type UserVolumeConfigV1Alpha1 -type ExternalVolumeConfigV1Alpha1 -type VolumeConfigV1Alpha1 -type ZswapConfigV1Alpha1 -type LVMVolumeConfigV1Alpha1 -pointer-receiver -header-file ../../../../../hack/boilerplate.txt -o deep_copy.generated.go .
