// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import "github.com/siderolabs/gen/xslices"

// conflictingVolumeKinds returns the volume document kinds which share a name namespace with
// selfKind.
//
// These are the kinds mounted at `/var/mnt/<name>`: two documents of different kinds under one name
// would claim the same mount point, and the name would no longer identify a single volume. Raw and
// swap volumes are deliberately absent, as they are never mounted there.
func conflictingVolumeKinds(selfKind string) []string {
	return xslices.Filter([]string{
		UserVolumeConfigKind,
		ExistingVolumeConfigKind,
		ExternalVolumeConfigKind,
	}, func(kind string) bool {
		return kind != selfKind
	})
}
