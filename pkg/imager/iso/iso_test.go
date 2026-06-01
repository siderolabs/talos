// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package iso_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/imager/iso"
)

func TestVolumeID(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		in string

		out string
	}{
		{
			in: "Talos-v1.7.6",

			out: "TALOS_V1_7_6",
		},
		{
			in: "Talos-v1.7.6-beta.0",

			out: "TALOS_V1_7_6_BETA_0",
		},
	} {
		t.Run(test.in, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.out, iso.VolumeID(test.in))
		})
	}
}
