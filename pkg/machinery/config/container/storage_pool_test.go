// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
)

func TestStoragePoolReferences(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		filename string
		err      string
	}{
		{filename: "storagepool-user-directory.yaml"},
		{filename: "storagepool-user-partition.yaml"},
		{filename: "storagepool-user-partition-none.yaml"},
		{filename: "storagepool-user-partition-unsupported.yaml", err: "unsupported filesystem type"},
		{filename: "storagepool-existing.yaml"},
		{filename: "storagepool-external.yaml"},
		{filename: "storagepool-missing.yaml", err: "must reference a UserVolumeConfig"},
		{filename: "storagepool-raw.yaml", err: "must reference a UserVolumeConfig"},
		{filename: "storagepool-ambiguous-user-existing.yaml", err: "conflicting documents"},
		{filename: "storagepool-wrong-prefix.yaml", err: `volume.name "e-vm-data" must reference`},
		{filename: "storagepool-readonly-existing.yaml", err: "is read-only"},
		{filename: "storagepool-readonly-external.yaml", err: "is read-only"},
		{filename: "storagepool-shared-backing.yaml", err: "reference the same backing volume"},
		{filename: "storagepool-independent.yaml"},
	} {
		t.Run(test.filename, func(t *testing.T) {
			t.Parallel()

			provider, err := configloader.NewFromFile(filepath.Join("testdata", test.filename))
			if err == nil {
				_, err = provider.ValidateAsClient(validationMode{})
			}

			if test.err != "" {
				require.ErrorContains(t, err, test.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
