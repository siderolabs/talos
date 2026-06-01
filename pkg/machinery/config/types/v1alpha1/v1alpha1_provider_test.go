// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

func TestInstallDiskSelector(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		selector v1alpha1.InstallDiskSelector

		expected string
	}{
		{
			name: "size",

			selector: v1alpha1.InstallDiskSelector{
				Size: &v1alpha1.InstallDiskSizeMatcher{
					MatchData: v1alpha1.InstallDiskSizeMatchData{
						Op:   "<=",
						Size: 256 * 1024,
					},
				},
			},

			expected: `disk.size <= 262144u && disk.transport != "" && !disk.readonly && !disk.cdrom`,
		},
		{
			name: "size and type",

			selector: v1alpha1.InstallDiskSelector{
				Size: &v1alpha1.InstallDiskSizeMatcher{
					MatchData: v1alpha1.InstallDiskSizeMatchData{
						Size: 1024 * 1024,
					},
				},
				Type: v1alpha1.InstallDiskType("nvme"),
			},

			expected: `disk.size == 1048576u && disk.transport != "" && disk.transport == "nvme" && !disk.readonly &&
!disk.cdrom`,
		},
		{
			name: "size and type and modalias",

			selector: v1alpha1.InstallDiskSelector{
				Size: &v1alpha1.InstallDiskSizeMatcher{
					MatchData: v1alpha1.InstallDiskSizeMatchData{
						Size: 1024 * 1024,
					},
				},
				Type:     v1alpha1.InstallDiskType("hdd"),
				Modalias: "pci:1234:5678*",
			},

			expected: `disk.size == 1048576u && glob("pci:1234:5678*", disk.modalias) && disk.transport != "" &&
disk.rotational && !disk.readonly && !disk.cdrom`,
		},
		{
			name: "ssd",

			selector: v1alpha1.InstallDiskSelector{
				Type: v1alpha1.InstallDiskType("ssd"),
			},

			expected: `disk.transport != "" && !disk.rotational && !disk.readonly && !disk.cdrom`,
		},
		{
			name: "bus path",

			selector: v1alpha1.InstallDiskSelector{
				BusPath: "/pci-0000:00:1f.2/*",
			},

			expected: `disk.transport != "" && glob("/pci-0000:00:1f.2/*", disk.bus_path) && !disk.readonly &&
!disk.cdrom`,
		},
		{
			name: "uuid",

			selector: v1alpha1.InstallDiskSelector{
				UUID: "0000-0001-*",
			},

			expected: `glob("0000-0001-*", disk.uuid) && disk.transport != "" && !disk.readonly && !disk.cdrom`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			installCfg := &v1alpha1.InstallConfig{
				InstallDiskSelector: &test.selector,
			}

			expr, err := installCfg.DiskMatchExpression()
			require.NoError(t, err)

			assert.Equal(t, test.expected, expr.String())
		})
	}
}
