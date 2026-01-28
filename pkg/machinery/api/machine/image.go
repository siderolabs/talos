// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package machine contains the machine service API definitions.
package machine

import (
	fmt "fmt"

	"github.com/dustin/go-humanize"
)

// Fmt formats the pull progress status into a human-readable string.
func (s *ImageServicePullLayerProgress) Fmt() string {
	switch s.GetStatus() {
	case ImageServicePullLayerProgress_DOWNLOADING:
		return fmt.Sprintf("downloading layer %s/%s (%.1f%%)",
			humanize.IBytes(uint64(s.GetOffset())),
			humanize.IBytes(uint64(s.GetTotal())),
			float64(s.GetOffset())/float64(s.GetTotal())*100.0,
		)

	case ImageServicePullLayerProgress_DOWNLOAD_COMPLETE:
		return "layer download complete"

	case ImageServicePullLayerProgress_EXTRACTING:
		return fmt.Sprintf("extracting layer (%s)", s.Elapsed.AsDuration())

	case ImageServicePullLayerProgress_EXTRACT_COMPLETE:
		return "layer pull complete"

	case ImageServicePullLayerProgress_ALREADY_EXISTS:
		return "layer already exists"
	}

	return ""
}
