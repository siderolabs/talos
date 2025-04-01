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
