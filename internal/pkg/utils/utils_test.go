// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package utils_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/siderolabs/gen/pair"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/gen/xtesting/check"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/utils"
)

func TestHandleSet(t *testing.T) {
	table := []struct {
		state       string
		vals        []int
		expected    []string
		removed     []string
		expectedErr check.Check
	}{
		{
			"initial state",
			[]int{1, 2, 3},
			[]string{"1", "2", "3"},
			nil,
			check.NoError(),
		},
		{
			"add and remove",
			[]int{1, 2, 4},
			[]string{"1", "2", "4"},
			[]string{"3"},
			check.NoError(),
		},
		{
			"add and remove with error",
			[]int{1, 2, 5, 42, 43},
			[]string{"1", "2", "5"},
			[]string{"4"},
			check.EqualError("42 is not allowed"),
		},
		{
			"remove all",
			[]int{},
			nil,
			[]string{"1", "2", "5"},
			check.NoError(),
		},
		{
			"start again",
			[]int{1, 2, 3, 45, 46},
			[]string{"1", "2", "3", "45", "46"},
			nil,
			check.NoError(),
		},
		{
			"remove with error",
			[]int{2, 3},
			[]string{"2", "3", "45", "46"},
			[]string{"1"},
			check.EqualError("45 is not allowed to delete"),
		},
		{
			"remove all again",
			[]int{},
			nil,
			[]string{"2", "3", "45", "46"},
			check.NoError(),
		},
	}

	oneWithError := false

	add := func(i int) (string, error) {
		if i == 42 {
			return "", errors.New("42 is not allowed")
		}

		return strconv.Itoa(i), nil
	}

	var removed []string

	remove := func(h pair.Pair[int, string]) error {
		if h.F1 == 45 && !oneWithError {
			oneWithError = true

			return errors.New("45 is not allowed to delete")
		}

		removed = append(removed, h.F2)

		return nil
	}

	var hs []pair.Pair[int, string]

	for _, tt := range table {
		t.Run(tt.state, func(t *testing.T) {
			removed = nil

			var err error
			hs, err = utils.UpdatePairSet(hs, tt.vals, add, remove)

			tt.expectedErr(t, err)

			require.Equal(t, tt.expected, xslices.Map(hs, func(h pair.Pair[int, string]) string { return h.F2 }))
			require.Equal(t, tt.removed, removed)
		})
	}
}
