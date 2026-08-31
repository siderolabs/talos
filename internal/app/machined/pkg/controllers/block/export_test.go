// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"github.com/siderolabs/go-blockdevice/v2/blkid"
	"github.com/siderolabs/go-blockdevice/v2/partitioning/gpt"
)

// GPTNestedProbeResults is exported for testing purposes.
func GPTNestedProbeResults(gptdev gpt.Device, kernelPartitions map[uint]string) ([]blkid.NestedProbeResult, error) {
	return gptNestedProbeResults(gptdev, kernelPartitions)
}
