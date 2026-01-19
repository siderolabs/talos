// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package partition

import (
	"fmt"
	"strings"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-blockdevice/v2/blkid"
	"github.com/siderolabs/go-blockdevice/v2/block"
)

// WipeWithSignatures wipes the given block device by its signatures (if available)
// and falls back to fast wipe otherwise.
//
// The function assumes that the caller locked properly the block device (or the parent
// device in case of partitions) before calling it.
//
// If non-nil log function is passed, it will be used to log the wipe process.
func WipeWithSignatures(bd *block.Device, deviceName string, log func(string, ...any)) error {
	info, err := blkid.Probe(bd.File(), blkid.WithSkipLocking(true))
	if err == nil && info != nil && len(info.SignatureRanges) > 0 { // probe successful, wipe by signatures
		if err = bd.FastWipe(xslices.Map(info.SignatureRanges, func(r blkid.SignatureRange) block.Range {
			return block.Range(r)
		})...); err != nil {
			return fmt.Errorf("failed to wipe block device %q: %v", deviceName, err)
		}

		if log != nil {
			log("block device %q wiped by ranges: %s",
				deviceName,
				strings.Join(
					xslices.Map(info.SignatureRanges,
						func(r blkid.SignatureRange) string {
							return fmt.Sprintf("%d-%d", r.Offset, r.Offset+r.Size)
						},
					),
					", ",
				),
			)
		}
	}

	// [TODO]: wipe the first/last 1MiB after wiping by signatures to cover somewhat unknown edge cases
	// What has been observed so far is that wiping VFAT signature still makes `mkfs.xfs` believe there is
	// a VFAT filesystem on the partition, refusing to create XFS over it without `-f` flag.
	//
	// probe failed or no signatures found, fast wipe
	if err = bd.FastWipe(); err != nil {
		return fmt.Errorf("failed to wipe block device %q: %v", deviceName, err)
	}

	if log != nil {
		log("block device %q wiped with fast method", deviceName)
	}

	return nil
}
