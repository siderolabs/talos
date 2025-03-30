// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package block provides block device and volume configuration documents.
package block

//go:generate docgen -output block_doc.go block.go volume_config.go fs_scrub.go

//go:generate deep-copy -type VolumeConfigV1Alpha1 -type FilesystemScrubV1Alpha1 -pointer-receiver -header-file ../../../../../hack/boilerplate.txt -o deep_copy.generated.go .
