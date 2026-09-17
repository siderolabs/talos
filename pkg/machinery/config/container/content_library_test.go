// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
)

// newContentLibraryDoc builds a ContentLibraryConfig document backed by the named volume.
func newContentLibraryDoc(name, volumeName string) *hypervisorcfg.ContentLibraryConfigV1Alpha1 {
	doc := hypervisorcfg.NewContentLibraryConfigV1Alpha1()
	doc.MetaName = name
	doc.BackingConfig.VolumeName = volumeName

	return doc
}

// newUserVolumeDoc builds a UserVolumeConfig document, the kind which is never read-only.
func newUserVolumeDoc(name string) *blockcfg.UserVolumeConfigV1Alpha1 {
	doc := blockcfg.NewUserVolumeConfigV1Alpha1()
	doc.MetaName = name

	return doc
}

// newExternalVolumeDoc builds an ExternalVolumeConfig document, read-only when asked for.
func newExternalVolumeDoc(name string, readOnly bool) *blockcfg.ExternalVolumeConfigV1Alpha1 {
	doc := blockcfg.NewExternalVolumeConfigV1Alpha1()
	doc.MetaName = name
	doc.MountSpec.MountReadOnly = new(readOnly)

	return doc
}

// newExistingVolumeDoc builds an ExistingVolumeConfig document, read-only when asked for.
func newExistingVolumeDoc(name string, readOnly bool) *blockcfg.ExistingVolumeConfigV1Alpha1 {
	doc := blockcfg.NewExistingVolumeConfigV1Alpha1()
	doc.MetaName = name
	doc.MountSpec.MountReadOnly = new(readOnly)

	return doc
}

// validateContentLibraryDocs runs container-level validation over the given documents.
//
// The volume documents are left bare, so their own validation has plenty to say about them; only
// what the content library check reports is of interest here.
func validateContentLibraryDocs(t *testing.T, docs ...config.Document) error {
	t.Helper()

	ctr, err := container.New(docs...)
	require.NoError(t, err)

	_, err = ctr.ValidateAsClient(validationMode{})

	return err
}

func TestContentLibraryBackingVolumes(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name         string
		docs         []config.Document
		expectedErrs []string
	}{
		{
			name: "one library",
			docs: []config.Document{
				newUserVolumeDoc("vm-images"),
				newContentLibraryDoc("images", "vm-images"),
			},
		},
		{
			name: "a volume each",
			docs: []config.Document{
				newUserVolumeDoc("vm-images"),
				newUserVolumeDoc("more-vm-images"),
				newContentLibraryDoc("images", "vm-images"),
				newContentLibraryDoc("more-images", "more-vm-images"),
			},
		},
		{
			// The ID is derived from the document kind and the name, so this volume is its own, and
			// is addressed as `u-u-images`.
			name: "volume named like an ID",
			docs: []config.Document{
				newUserVolumeDoc("u-images"),
				newContentLibraryDoc("images", "u-images"),
			},
		},
		{
			name: "backing volume not configured",
			docs: []config.Document{
				newContentLibraryDoc("images", "nowhere"),
			},
			expectedErrs: []string{
				`content library "images" cannot be backed by volume "nowhere": no UserVolumeConfig, ExistingVolumeConfig or ExternalVolumeConfig declares that volume`,
			},
		},
		{
			name: "writable external volume",
			docs: []config.Document{
				newExternalVolumeDoc("nfs-images", false),
				newContentLibraryDoc("images", "nfs-images"),
			},
		},
		{
			name: "two libraries on one volume",
			docs: []config.Document{
				newUserVolumeDoc("vm-images"),
				newContentLibraryDoc("images", "vm-images"),
				newContentLibraryDoc("more-images", "vm-images"),
			},
			expectedErrs: []string{
				`content library "more-images" cannot be backed by volume "vm-images": content library "images" is already backed by it`,
			},
		},
		{
			name: "three libraries on one volume",
			docs: []config.Document{
				newUserVolumeDoc("vm-images"),
				newContentLibraryDoc("c-images", "vm-images"),
				newContentLibraryDoc("a-images", "vm-images"),
				newContentLibraryDoc("b-images", "vm-images"),
			},
			expectedErrs: []string{
				`content library "b-images" cannot be backed by volume "vm-images": content library "a-images" is already backed by it`,
				`content library "c-images" cannot be backed by volume "vm-images": content library "a-images" is already backed by it`,
			},
		},
		{
			name: "read-only external volume",
			docs: []config.Document{
				newExternalVolumeDoc("nfs-images", true),
				newContentLibraryDoc("images", "nfs-images"),
			},
			expectedErrs: []string{
				`content library "images" cannot be backed by volume "nfs-images": the volume is configured read-only, and uploads write into the library`,
			},
		},
		{
			name: "read-only existing volume",
			docs: []config.Document{
				newExistingVolumeDoc("found-images", true),
				newContentLibraryDoc("images", "found-images"),
			},
			expectedErrs: []string{
				`content library "images" cannot be backed by volume "found-images": the volume is configured read-only, and uploads write into the library`,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := validateContentLibraryDocs(t, test.docs...)

			if len(test.expectedErrs) == 0 {
				if err != nil {
					// The bare volume documents fail their own validation, which is not what this
					// test is about.
					assert.NotContains(t, err.Error(), "cannot be backed by")
					assert.NotContains(t, err.Error(), "content library")
				}

				return
			}

			require.Error(t, err)

			for _, expectedErr := range test.expectedErrs {
				assert.ErrorContains(t, err, expectedErr)
			}
		})
	}
}
