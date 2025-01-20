// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package platforms_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/platforms"
)

func TestSBC(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		sbc          platforms.SBC
		talosVersion string

		expectedDiskImagePath string
	}{
		{
			name: "rpi_generic",

			sbc:          platforms.SBCs()[0],
			talosVersion: "1.9.0",

			expectedDiskImagePath: "metal-arm64.raw.xz",
		},
		{
			name: "bananapi_m64",

			sbc:          platforms.SBCs()[0],
			talosVersion: "1.4.0",

			expectedDiskImagePath: "metal-rpi_generic-arm64.raw.xz",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expectedDiskImagePath, test.sbc.DiskImagePath(test.talosVersion))
		})
	}
}
