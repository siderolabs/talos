// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sysblock_test

import (
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/sysblock"
)

const mpathUUID = "mpath-36005076810800567c8000000000009fb"

// buildSysFs materializes a fake sysfs tree: files are created together with their parent
// directories, then symlinks are created pointing at the given (relative) targets.
func buildSysFs(t *testing.T, files, links map[string]string) string {
	t.Helper()

	root := t.TempDir()

	for path, contents := range files {
		full := filepath.Join(root, path)

		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(contents), 0o644))
	}

	for path, target := range links {
		full := filepath.Join(root, path)

		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.Symlink(target, full))
	}

	return root
}

//nolint:maintidx
func TestAugmentDeviceMapper(t *testing.T) {
	t.Parallel()

	// the values of the uevent of a device-mapper device, which the kernel always describes as a
	// whole disk
	diskValues := map[string]string{
		"MAJOR":   "253",
		"MINOR":   "1",
		"DEVNAME": "dm-1",
		"DEVTYPE": "disk",
	}

	for _, test := range []struct {
		// contents of the fake sysfs tree, relative to its root
		files map[string]string
		// symlinks of the fake sysfs tree, relative to its root
		links map[string]string
		// the uevent values to augment; the disk values above when unset
		values map[string]string
		// the values expected to be added by the augmentation; none when unset
		expectedAdded map[string]string

		name string
		// the device to augment, relative to the root of the tree
		device string
	}{
		{
			name: "multipath partition map",
			files: map[string]string{
				"dm-0/dm/name": "mpatha\n",
				"dm-0/dm/uuid": mpathUUID + "\n",
				"dm-1/dm/name": "mpatha-part1\n",
				"dm-1/dm/uuid": "part1-" + mpathUUID + "\n",
			},
			links: map[string]string{
				"dm-1/slaves/dm-0":  "../../dm-0",
				"dm-0/holders/dm-1": "../../dm-1",
			},
			device: "dm-1",

			expectedAdded: map[string]string{
				"DEVTYPE":                "partition",
				"PARTN":                  "1",
				sysblock.ParentDeviceKey: "dm-0",
			},
		},
		{
			name: "partition number with two digits",
			files: map[string]string{
				"dm-0/dm/uuid": mpathUUID + "\n",
				"dm-1/dm/uuid": "part12-" + mpathUUID + "\n",
			},
			links: map[string]string{
				"dm-1/slaves/dm-0": "../../dm-0",
			},
			device: "dm-1",

			expectedAdded: map[string]string{
				"DEVTYPE":                "partition",
				"PARTN":                  "12",
				sysblock.ParentDeviceKey: "dm-0",
			},
		},
		{
			name: "whole multipath device",
			files: map[string]string{
				"dm-0/dm/name": "mpatha\n",
				"dm-0/dm/uuid": mpathUUID + "\n",
			},
			links: map[string]string{
				"dm-0/holders/dm-1": "../../dm-1",
			},
			device: "dm-0",
		},
		{
			name: "LVM logical volume",
			files: map[string]string{
				"dm-1/dm/uuid": "LVM-AbCdEf0001000200030004000500060007-lvname\n",
			},
			links: map[string]string{
				"dm-1/slaves/dm-0": "../../dm-0",
			},
			device: "dm-1",
		},
		{
			name: "dm-crypt device",
			files: map[string]string{
				"dm-1/dm/uuid": "CRYPT-LUKS2-abcdef1234567890abcdef1234567890-cryptname\n",
			},
			links: map[string]string{
				"dm-1/slaves/dm-0": "../../dm-0",
			},
			device: "dm-1",
		},
		{
			name: "kpartx over a device which is not device-mapper",
			files: map[string]string{
				// the loop device carries real kernel partitions, so its kpartx maps are a
				// duplicate view and are left as disks
				"loop0/loop0p1/partition": "1\n",
				"dm-1/dm/uuid":            "part1-devnode_7:0_QY0zjmTfMLI9XApVhtCsQAcxbcvMHK6b\n",
			},
			links: map[string]string{
				"dm-1/slaves/loop0": "../../loop0",
			},
			device: "dm-1",
		},
		{
			name: "parent carries a different UUID",
			files: map[string]string{
				"dm-0/dm/uuid": "mpath-36005076810800567c8000000000009f8\n",
				"dm-1/dm/uuid": "part1-" + mpathUUID + "\n",
			},
			links: map[string]string{
				"dm-1/slaves/dm-0": "../../dm-0",
			},
			device: "dm-1",
		},
		{
			name: "no device mapped over",
			files: map[string]string{
				"dm-1/dm/uuid": "part1-" + mpathUUID + "\n",
			},
			device: "dm-1",
		},
		{
			name: "more than one device mapped over",
			files: map[string]string{
				"dm-0/dm/uuid": mpathUUID + "\n",
				"dm-2/dm/uuid": mpathUUID + "\n",
				"dm-1/dm/uuid": "part1-" + mpathUUID + "\n",
			},
			links: map[string]string{
				"dm-1/slaves/dm-0": "../../dm-0",
				"dm-1/slaves/dm-2": "../../dm-2",
			},
			device: "dm-1",
		},
		{
			name: "device which is not device-mapper",
			files: map[string]string{
				"sda/size": "3907029168\n",
			},
			values: map[string]string{
				"MAJOR":   "8",
				"MINOR":   "0",
				"DEVNAME": "sda",
				"DEVTYPE": "disk",
			},
			device: "sda",
		},
		{
			name: "kernel partition is left alone",
			files: map[string]string{
				"sda/sda1/partition": "1\n",
			},
			values: map[string]string{
				"MAJOR":   "8",
				"MINOR":   "1",
				"DEVNAME": "sda1",
				"DEVTYPE": "partition",
				"PARTN":   "1",
			},
			device: "sda/sda1",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root := buildSysFs(t, test.files, test.links)

			values := maps.Clone(test.values)
			if values == nil {
				values = maps.Clone(diskValues)
			}

			expected := maps.Clone(values)
			maps.Copy(expected, test.expectedAdded)

			sysblock.AugmentDeviceMapper(filepath.Join(root, test.device), values)

			assert.Equal(t, expected, values)
		})
	}
}
