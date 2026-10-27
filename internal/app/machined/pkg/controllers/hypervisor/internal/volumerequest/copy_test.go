// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumerequest_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/volumerequest"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

func TestEnsureAdoption(t *testing.T) {
	for _, test := range []struct {
		name      string
		owner     resource.Owner
		existing  bool
		pool      string
		nameValue string
		format    string
		copySpec  *storage.StoragePoolVolumeCopySpec
		want      error
	}{
		{
			name: "new",
		},
		{
			name:     "existing unowned",
			existing: true,
			want:     volumerequest.ErrOwner,
		},
		{
			name:     "other owner",
			existing: true,
			owner:    "other",
			want:     volumerequest.ErrOwner,
		},
		{
			name:     "owned empty",
			existing: true,
			owner:    "owner",
		},
		{
			name:  "pool mismatch",
			owner: "owner",
			pool:  "other",
			want:  volumerequest.ErrDestination,
		},
		{
			name:      "name mismatch",
			owner:     "owner",
			pool:      "pool",
			nameValue: "other",
			format:    "raw",
			copySpec:  &storage.StoragePoolVolumeCopySpec{},
			want:      volumerequest.ErrDestination,
		},
		{
			name:      "format mismatch",
			owner:     "owner",
			pool:      "pool",
			nameValue: "name",
			format:    "qcow2",
			copySpec:  &storage.StoragePoolVolumeCopySpec{},
			want:      volumerequest.ErrDestination,
		},
		{
			name:      "missing copy",
			owner:     "owner",
			pool:      "pool",
			nameValue: "name",
			format:    "raw",
			want:      volumerequest.ErrDestination,
		},
		{
			name:      "bound copy",
			owner:     "owner",
			pool:      "pool",
			nameValue: "name",
			format:    "raw",
			copySpec: &storage.StoragePoolVolumeCopySpec{
				Source: "source",
				Digest: "digest",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := storage.NewStoragePoolVolumeSpec(storage.NamespaceName, "id")
			require.NoError(t, request.Metadata().SetOwner(test.owner))

			if test.existing {
				request.Metadata().SetVersion(resource.VersionUndefined.Next())
			}

			*request.TypedSpec() = storage.StoragePoolVolumeSpecSpec{
				Pool:   test.pool,
				Name:   test.nameValue,
				Format: test.format,
				Copy:   test.copySpec,
			}
			err := volumerequest.EnsureAdoption(request, "owner", volumerequest.Destination{
				Pool:   "pool",
				Name:   "name",
				Format: "raw",
			})
			require.ErrorIs(t, err, test.want)

			if err == nil {
				require.Equal(t, "pool", request.TypedSpec().Pool)
				require.Equal(t, "name", request.TypedSpec().Name)
				require.Equal(t, "raw", request.TypedSpec().Format)
				require.NotNil(t, request.TypedSpec().Copy)

				if test.copySpec != nil {
					require.Equal(t, test.copySpec, request.TypedSpec().Copy)
				}
			}
		})
	}
}

func TestBindSource(t *testing.T) {
	for _, test := range []struct {
		name       string
		owner      resource.Owner
		pool       string
		volume     string
		format     string
		copySpec   *storage.StoragePoolVolumeCopySpec
		imageError error
	}{
		{
			name:     "unset",
			owner:    "owner",
			pool:     "pool",
			volume:   "name",
			format:   "raw",
			copySpec: &storage.StoragePoolVolumeCopySpec{},
		},
		{
			name:   "same",
			owner:  "owner",
			pool:   "pool",
			volume: "name",
			format: "raw",
			copySpec: &storage.StoragePoolVolumeCopySpec{
				Source: "source",
				Digest: "digest",
			},
		},
		{
			name:   "source mismatch",
			owner:  "owner",
			pool:   "pool",
			volume: "name",
			format: "raw",
			copySpec: &storage.StoragePoolVolumeCopySpec{
				Source: "other",
				Digest: "digest",
			},
		},
		{
			name:   "digest mismatch",
			owner:  "owner",
			pool:   "pool",
			volume: "name",
			format: "raw",
			copySpec: &storage.StoragePoolVolumeCopySpec{
				Source: "source",
				Digest: "other",
			},
		},
		{
			name:   "digest without source",
			owner:  "owner",
			pool:   "pool",
			volume: "name",
			format: "raw",
			copySpec: &storage.StoragePoolVolumeCopySpec{
				Digest: "other",
			},
		},
		{
			name:       "unowned",
			pool:       "pool",
			volume:     "name",
			format:     "raw",
			copySpec:   &storage.StoragePoolVolumeCopySpec{},
			imageError: volumerequest.ErrOwner,
		},
		{
			name:       "other owner",
			owner:      "other",
			pool:       "pool",
			volume:     "name",
			format:     "raw",
			copySpec:   &storage.StoragePoolVolumeCopySpec{},
			imageError: volumerequest.ErrOwner,
		},
		{
			name:       "pool mismatch",
			owner:      "owner",
			pool:       "other",
			volume:     "name",
			format:     "raw",
			copySpec:   &storage.StoragePoolVolumeCopySpec{},
			imageError: volumerequest.ErrDestination,
		},
		{
			name:       "name mismatch",
			owner:      "owner",
			pool:       "pool",
			volume:     "other",
			format:     "raw",
			copySpec:   &storage.StoragePoolVolumeCopySpec{},
			imageError: volumerequest.ErrDestination,
		},
		{
			name:     "format mismatch",
			owner:    "owner",
			pool:     "pool",
			volume:   "name",
			format:   "qcow2",
			copySpec: &storage.StoragePoolVolumeCopySpec{},
		},
		{
			name:       "missing copy",
			owner:      "owner",
			pool:       "pool",
			volume:     "name",
			format:     "raw",
			imageError: volumerequest.ErrDestination,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := storage.NewStoragePoolVolumeSpec(storage.NamespaceName, "id")
			require.NoError(t, request.Metadata().SetOwner(test.owner))

			*request.TypedSpec() = storage.StoragePoolVolumeSpecSpec{
				Pool:   test.pool,
				Name:   test.volume,
				Format: test.format,
			}
			if test.copySpec != nil {
				copySpec := *test.copySpec
				request.TypedSpec().Copy = &copySpec
			}

			destination := volumerequest.Destination{
				Pool:   "pool",
				Name:   "name",
				Format: "raw",
			}

			err := volumerequest.BindImage(request, "owner", destination, "source", "digest")
			require.ErrorIs(t, err, test.imageError)

			if err != nil || test.copySpec.Source != "" {
				require.Equal(t, test.copySpec, request.TypedSpec().Copy)
			} else {
				require.Equal(t, &storage.StoragePoolVolumeCopySpec{
					Source: "source",
					Digest: "digest",
				}, request.TypedSpec().Copy)
			}
		})
	}
}
