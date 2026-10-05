// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kernel_test

import (
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/kernel"
)

func TestNormalizeMemoryLimit(t *testing.T) {
	t.Parallel()

	for _, pageSize := range []int{4096, 16384, 65536} {
		t.Run(strconv.Itoa(pageSize), func(t *testing.T) {
			t.Parallel()

			page := uint64(pageSize)
			sentinel := uint64(math.MaxInt64) / page * page
			largestFinite := sentinel - page

			for _, test := range []struct {
				name     string
				limit    uint64
				expected uint64
				errMsg   string
			}{
				{name: "aligned", limit: 4 << 30, expected: 4 << 30},
				{name: "one page", limit: page, expected: page},
				{name: "rounds down", limit: (4 << 30) + page - 1, expected: 4 << 30},
				{name: "zero", limit: 0, errMsg: "smaller than the page size"},
				{name: "sub page", limit: page - 1, errMsg: "smaller than the page size"},
				{name: "exceeds int64", limit: math.MaxInt64 + 1, errMsg: "exceeds the maximum"},
				{name: "max uint64", limit: math.MaxUint64, errMsg: "exceeds the maximum"},
				{name: "int64 max is the kernel sentinel", limit: math.MaxInt64, errMsg: "treated as unlimited"},
				{name: "exact sentinel", limit: sentinel, errMsg: "treated as unlimited"},
				{name: "sentinel after rounding", limit: sentinel + 1, errMsg: "treated as unlimited"},
				{name: "largest finite", limit: largestFinite, expected: largestFinite},
				{name: "largest finite after rounding", limit: sentinel - 1, expected: largestFinite},
			} {
				t.Run(test.name, func(t *testing.T) {
					t.Parallel()

					actual, err := kernel.NormalizeMemoryLimit(test.limit, pageSize)

					if test.errMsg != "" {
						require.ErrorContains(t, err, test.errMsg)
						assert.Zero(t, actual)

						return
					}

					require.NoError(t, err)
					assert.Equal(t, test.expected, actual)
					assert.Zero(t, actual%page)
				})
			}
		})
	}

	for _, pageSize := range []int{0, -4096} {
		t.Run("invalid page size "+strconv.Itoa(pageSize), func(t *testing.T) {
			t.Parallel()

			actual, err := kernel.NormalizeMemoryLimit(4<<30, pageSize)
			require.ErrorContains(t, err, "invalid page size")
			assert.Zero(t, actual)
		})
	}
}
