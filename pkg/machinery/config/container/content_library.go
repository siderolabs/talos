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

	backingVolumes := backingVolumeCandidates(container)

	backedBy := map[string]string{}

	for _, contentLibraryConfig := range sorted {
		volumeName := contentLibraryConfig.BackingVolumeName()

		if volumeName == "" {
			// Missing backing volume is reported by the document's own validation; reporting every
			// library which is missing one as sharing a volume with the others would only add noise.
			continue
		}

		readOnly, declared := backingVolumes[volumeName]

		switch {
		case !declared:
			errs = multierror.Append(errs, fmt.Errorf(
				"content library %q cannot be backed by volume %q: no UserVolumeConfig, ExistingVolumeConfig or ExternalVolumeConfig declares that volume",
				contentLibraryConfig.Name(), volumeName,
			))
		case readOnly:
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

// backingVolumeCandidates maps the name of every volume which can back a content library or
// storage pool to its read-only mount policy.
//
// These are the kinds mounted at `/var/mnt/<name>`, which consumers resolve backing volume
// names against. The name is taken as the document declares it: the internal ID is derived from the
// kind and the name, so a volume declared `u-images` is a volume of its own and not the ID of one
// declared `images`. A user volume is never read-only, as the kind carries no such option.
func backingVolumeCandidates(container *Container) map[string]bool {
	readOnlyByName := map[string]bool{}

	for _, userVolumeConfig := range container.UserVolumeConfigs() {
		readOnlyByName[userVolumeConfig.Name()] = false
	}

	for _, existingVolumeConfig := range container.ExistingVolumeConfigs() {
		readOnlyByName[existingVolumeConfig.Name()] = existingVolumeConfig.Mount().ReadOnly()
	}

	for _, externalVolumeConfig := range container.ExternalVolumeConfigs() {
		readOnlyByName[externalVolumeConfig.Name()] = externalVolumeConfig.Mount().ReadOnly()
	}

	return readOnlyByName
}
