// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package quirks_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
)

func TestSupportsResetOption(t *testing.T) {
	for _, test := range []struct {
		version string

		expected bool
	}{
		{
			version:  "1.5.0",
			expected: true,
		},
		{
			expected: true,
		},
		{
			version:  "1.3.7",
			expected: false,
		},
	} {
		t.Run(test.version, func(t *testing.T) {
			assert.Equal(t, test.expected, quirks.New(test.version).SupportsResetGRUBOption())
		})
	}
}

func TestSupportsCompressedEncodedMETA(t *testing.T) {
	for _, test := range []struct {
		version string

		expected bool
	}{
		{
			version:  "1.6.3",
			expected: true,
		},
		{
			version:  "1.7.0",
			expected: true,
		},
		{
			expected: true,
		},
		{
			version:  "1.6.2",
			expected: false,
		},
	} {
		t.Run(test.version, func(t *testing.T) {
			assert.Equal(t, test.expected, quirks.New(test.version).SupportsCompressedEncodedMETA())
		})
	}
}

func TestSupportsOverlay(t *testing.T) {
	for _, test := range []struct {
		version string

		expected bool
	}{
		{
			version:  "1.6.3",
			expected: false,
		},
		{
			version:  "1.7.0",
			expected: true,
		},
		{
			expected: true,
		},
		{
			version:  "1.6.2",
			expected: false,
		},
		{
			version:  "1.7.0-alpha.0",
			expected: true,
		},
		{
			version:  "v1.7.0-alpha.0-75-gff08e2821",
			expected: true,
		},
	} {
		t.Run(test.version, func(t *testing.T) {
			assert.Equal(t, test.expected, quirks.New(test.version).SupportsOverlay())
		})
	}
}

func TestSupportsZstd(t *testing.T) {
	for _, test := range []struct {
		version string

		expected bool
	}{
		{
			version:  "1.7.3",
			expected: false,
		},
		{
			expected: true,
		},
		{
			version:  "1.6.2",
			expected: false,
		},
		{
			version:  "1.8.0-alpha.0",
			expected: true,
		},
		{
			version:  "v1.8.3",
			expected: true,
		},
	} {
		t.Run(test.version, func(t *testing.T) {
			assert.Equal(t, test.expected, quirks.New(test.version).UseZSTDCompression())
		})
	}
}

func TestXFSMkfsConfigFile(t *testing.T) {
	for _, test := range []struct {
		version string

		expected string
	}{
		{
			version:  "1.5.0",
			expected: "/usr/share/xfsprogs/mkfs/lts_6.1.conf",
		},
		{
			version:  "1.6.2",
			expected: "/usr/share/xfsprogs/mkfs/lts_6.1.conf",
		},
		{
			version:  "1.7.0",
			expected: "/usr/share/xfsprogs/mkfs/lts_6.1.conf",
		},

		{
			version:  "1.8.1",
			expected: "/usr/share/xfsprogs/mkfs/lts_6.6.conf",
		},
		{
			version:  "1.9.3",
			expected: "/usr/share/xfsprogs/mkfs/lts_6.6.conf",
		},
		{
			version:  "1.10.0",
			expected: "/usr/share/xfsprogs/mkfs/lts_6.12.conf",
		},
		{
			expected: "/usr/share/xfsprogs/mkfs/lts_6.12.conf",
		},
		{
			expected: "/usr/share/xfsprogs/mkfs/lts_6.12.conf",
		},
	} {
		t.Run(test.version, func(t *testing.T) {
			assert.Equal(t, test.expected, quirks.New(test.version).XFSMkfsConfig())
		})
	}
}

func TestPartitionSizes(t *testing.T) {
	const (
		MiB = 1024 * 1024
		GiB = 1024 * MiB
	)

	for _, test := range []struct {
		version string

		// expected partition sizes
		grubEFISize      uint64
		grubBIOSSize     uint64
		grubBootSize     uint64
		ukiEFISize       uint64
		metaSize         uint64
		stateSize        uint64
		ephemeralMinSize uint64
	}{
		{
			version:          "1.9.0",
			grubEFISize:      100 * MiB,
			grubBIOSSize:     1 * MiB,
			grubBootSize:     1000 * MiB,
			ukiEFISize:       1000*MiB + 100*MiB + 1*MiB,
			metaSize:         1 * MiB,
			stateSize:        100 * MiB,
			ephemeralMinSize: 2 * GiB,
		},
		{
			version:          "1.10.0",
			grubEFISize:      100 * MiB,
			grubBIOSSize:     1 * MiB,
			grubBootSize:     1000 * MiB,
			ukiEFISize:       1000*MiB + 100*MiB + 1*MiB,
			metaSize:         1 * MiB,
			stateSize:        100 * MiB,
			ephemeralMinSize: 2 * GiB,
		},
		{
			version:          "1.11.0",
			grubEFISize:      100 * MiB,
			grubBIOSSize:     1 * MiB,
			grubBootSize:     2000 * MiB,
			ukiEFISize:       2000*MiB + 100*MiB + 1*MiB,
			metaSize:         1 * MiB,
			stateSize:        100 * MiB,
			ephemeralMinSize: 2 * GiB,
		},
		{
			version:          "",
			grubEFISize:      100 * MiB,
			grubBIOSSize:     1 * MiB,
			grubBootSize:     2000 * MiB,
			ukiEFISize:       2000*MiB + 100*MiB + 1*MiB,
			metaSize:         1 * MiB,
			stateSize:        100 * MiB,
			ephemeralMinSize: 2 * GiB,
		},
	} {
		t.Run(test.version, func(t *testing.T) {
			ps := quirks.New(test.version).PartitionSizes()

			assert.Equal(t, test.grubEFISize, ps.GrubEFISize())
			assert.Equal(t, test.grubBIOSSize, ps.GrubBIOSSize())
			assert.Equal(t, test.grubBootSize, ps.GrubBootSize())
			assert.Equal(t, test.ukiEFISize, ps.UKIEFISize())
			assert.Equal(t, test.metaSize, ps.METASize())
			assert.Equal(t, test.stateSize, ps.StateSize())
			assert.Equal(t, test.ephemeralMinSize, ps.EphemeralMinSize())
		})
	}
}
