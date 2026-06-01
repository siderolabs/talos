// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
)

func TestSizeUnmarshal(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		in       string
		want     uint64
		negative bool
	}{
		{in: "", want: 0},
		{in: "100%", want: 100},
		{in: "33.4%", want: 33},
		{in: "33.4124%", want: 33},
		{in: "1048576", want: 1048576},
		{in: "2.5GiB", want: 2684354560},
		{in: "2.5GB", want: 2500000000},
		{in: "2.5G", want: 2500000000},
		{in: "1MiB", want: 1048576},
		{in: "-100%", want: 100, negative: true},
		{in: "-33.4%", want: 33, negative: true},
		{in: "-33.4124%", want: 33, negative: true},
		{in: "-1048576", want: 1048576, negative: true},
		{in: "-2.5GiB", want: 2684354560, negative: true},
		{in: "-2.5GB", want: 2500000000, negative: true},
		{in: "-2.5G", want: 2500000000, negative: true},
		{in: "-1MiB", want: 1048576, negative: true},
	} {
		t.Run(test.in, func(t *testing.T) {
			t.Parallel()

			var s block.Size

			require.NoError(t, s.UnmarshalText([]byte(test.in)))

			if strings.Contains(test.in, "%") {
				assert.Zero(t, s.Value())

				val, ok := s.RelativeValue()
				assert.True(t, ok)
				assert.Equal(t, test.want, val)
				assert.Equal(t, test.negative, s.IsNegative())
			} else {
				assert.Equal(t, test.want, s.Value())

				val, ok := s.RelativeValue()
				assert.False(t, ok)
				assert.Zero(t, val)
				assert.Equal(t, test.negative, s.IsNegative())
			}

			out, err := s.MarshalText()
			require.NoError(t, err)

			assert.Equal(t, test.in, string(out))
		})
	}
}
