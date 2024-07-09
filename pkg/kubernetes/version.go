// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"github.com/blang/semver/v4"
	"github.com/distribution/reference"
)

// VersionGTE returns true if the version of the image is greater than or equal to the provided version.
//
// It supports any kind of image reference, but requires the tag to be present.
func VersionGTE(image string, version semver.Version) bool {
	imageRef, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		// couldn't parse the reference, so we can't compare
		return false
	}

	taggedRef, ok := imageRef.(reference.Tagged)
	if !ok {
		// tag is missing
		return false
	}

	vers, err := semver.ParseTolerant(taggedRef.Tag())
	if err != nil {
		// invalid version
		return false
	}

	vers.Pre = nil // reset the pre-release version to compare only the version

	return vers.GTE(version)
}
