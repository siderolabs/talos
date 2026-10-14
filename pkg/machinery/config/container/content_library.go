// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/hashicorp/go-multierror"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// validateContentLibraryBackingVolumes checks the volumes backing the content libraries.
//
// The backing volume is named as the document declaring it names it, and is resolved against the
// rest of the configuration, so a name nothing declares leaves the library permanently unusable.
// A library's contents sit at the root of its backing volume, with no directory of the library's
// own, so two libraries sharing a volume would share every file in it, and either of them coming up
// would clear the uploads the other has in flight. Uploads write into the library, so a volume
// declared read-only cannot back one either.
func validateContentLibraryBackingVolumes(container *Container) error {
	configs := container.ContentLibraryConfigs()

	if len(configs) == 0 {
		return nil
	}

	// Sorted by name, so a given configuration always names the same library as the one already
	// backed by the volume.
	sorted := slices.SortedFunc(slices.Values(configs), func(a, b config.ContentLibraryConfig) int {
		return cmp.Compare(a.Name(), b.Name())
	})

	var errs *multierror.Error

	backedBy := map[string]string{}

	for _, contentLibraryConfig := range sorted {
		volumeName := contentLibraryConfig.BackingVolumeName()

		if volumeName == "" {
			// Missing backing volume is reported by the document's own validation; reporting every
			// library which is missing one as sharing a volume with the others would only add noise.
			continue
		}

		backing, declared := config.ResolveBackingVolume(container, volumeName)

		switch {
		case !declared:
			errs = multierror.Append(errs, fmt.Errorf(
				"content library %q cannot be backed by volume %q: no UserVolumeConfig, ExistingVolumeConfig or ExternalVolumeConfig declares that volume",
				contentLibraryConfig.Name(), volumeName,
			))
		case backing.ReadOnly:
			errs = multierror.Append(errs, fmt.Errorf(
				"content library %q cannot be backed by volume %q: the volume is configured read-only, and uploads write into the library",
				contentLibraryConfig.Name(), volumeName,
			))
		}

		if owner, taken := backedBy[volumeName]; taken {
			errs = multierror.Append(errs, fmt.Errorf(
				"content library %q cannot be backed by volume %q: content library %q is already backed by it",
				contentLibraryConfig.Name(), volumeName, owner,
			))

			continue
		}

		backedBy[volumeName] = contentLibraryConfig.Name()
	}

	return errs.ErrorOrNil()
}
