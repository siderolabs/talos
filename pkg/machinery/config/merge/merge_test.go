// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package merge_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/merge"
)

type Config struct {
	A             string
	B             int
	C             *bool
	D             *int
	Slice         []Struct
	ReplacedSlice []string `merge:"replace"`
	Map           map[string]Struct
	CustomSlice   CustomSlice
}

type Struct struct {
	DA bool
	DB *int
}

type CustomSlice []string

func (s *CustomSlice) Merge(other any) error {
	otherSlice, ok := other.(CustomSlice)
	if !ok {
		return fmt.Errorf("other is not CustomSlice: %v", other)
	}

	*s = append(*s, otherSlice...)
	slices.Sort(*s)

	return nil
}

type Unstructured map[string]any

func TestMerge(t *testing.T) {
	for _, tt := range []struct {
		name        string
		left, right any
		expected    any
	}{
		{
			name: "zero",
		},
		{
			name: "partial merge",
			left: &Config{
				A: "a",
				B: 3,
				C: new(true),
				Slice: []Struct{
					{
						DA: true,
						DB: new(1),
					},
				},
				Map: map[string]Struct{
					"a": {
						DA: true,
					},
					"b": {
						DB: new(2),
					},
				},
			},
			right: &Config{
				A: "aa",
				B: 4,
				Slice: []Struct{
					{
						DA: false,
						DB: new(2),
					},
				},
				Map: map[string]Struct{
					"a": {
						DB: new(3),
					},
					"b": {
						DA: true,
						DB: new(5),
					},
					"c": {
						DB: new(4),
					},
				},
			},
			expected: &Config{
				A: "aa",
				B: 4,
				C: new(true),
				Slice: []Struct{
					{
						DA: true,
						DB: new(1),
					},
					{
						DA: false,
						DB: new(2),
					},
				},
				Map: map[string]Struct{
					"a": {
						DB: new(3),
					},
					"b": {
						DA: true,
						DB: new(5),
					},
					"c": {
						DB: new(4),
					},
				},
			},
		},
		{
			name: "merge with zero",
			left: &Config{
				A: "a",
				B: 3,
				C: new(true),
				Slice: []Struct{
					{
						DA: false,
						DB: new(2),
					},
				},
				Map: map[string]Struct{
					"a": {
						DA: true,
					},
					"b": {
						DB: new(2),
					},
				},
			},
			right: &Config{},
			expected: &Config{
				A: "a",
				B: 3,
				C: new(true),
				Slice: []Struct{
					{
						DA: false,
						DB: new(2),
					},
				},
				Map: map[string]Struct{
					"a": {
						DA: true,
					},
					"b": {
						DB: new(2),
					},
				},
			},
		},
		{
			name: "merge from zero",
			left: &Config{},
			right: &Config{
				A: "a",
				B: 3,
				C: new(true),
				Slice: []Struct{
					{
						DA: false,
						DB: new(2),
					},
				},
				Map: map[string]Struct{
					"a": {
						DA: true,
					},
					"b": {
						DB: new(2),
					},
				},
			},
			expected: &Config{
				A: "a",
				B: 3,
				C: new(true),
				Slice: []Struct{
					{
						DA: false,
						DB: new(2),
					},
				},
				Map: map[string]Struct{
					"a": {
						DA: true,
					},
					"b": {
						DB: new(2),
					},
				},
			},
		},
		{
			name: "replace slice",
			left: &Config{
				ReplacedSlice: []string{"a", "b"},
			},
			right: &Config{
				ReplacedSlice: []string{"c", "d"},
			},
			expected: &Config{
				ReplacedSlice: []string{"c", "d"},
			},
		},
		{
			name: "zero slice",
			left: &Config{},
			right: &Config{
				Slice: []Struct{},
			},
			expected: &Config{
				Slice: []Struct{},
			},
		},
		{
			name: "custom slice",
			left: &Config{
				CustomSlice: []string{"a", "c"},
			},
			right: &Config{
				CustomSlice: []string{"b", "d"},
			},
			expected: &Config{
				CustomSlice: []string{"a", "b", "c", "d"},
			},
		},
		{
			name: "merge with pointer override",
			left: &Config{
				D: new(1),
			},
			right: &Config{
				D: new(0),
			},
			expected: &Config{
				D: new(0),
			},
		},
		{
			name: "unstructured",
			left: &Unstructured{
				"a": "aa",
				"map": map[string]any{
					"slice": []any{
						"s1",
					},
					"some": "value",
				},
			},
			right: &Unstructured{
				"b": "bb",
				"map": map[string]any{
					"slice": []any{
						"s2",
					},
					"other": "thing",
				},
			},
			expected: &Unstructured{
				"a": "aa",
				"b": "bb",
				"map": map[string]any{
					"slice": []any{
						"s1",
						"s2",
					},
					"some":  "value",
					"other": "thing",
				},
			},
		},
		{
			name: "unstructed with nil value",
			left: Unstructured{
				"a": nil,
				"b": []any{
					"c",
					"d",
				},
			},
			right: Unstructured{
				"a": Unstructured{
					"b": []any{
						"c",
						"d",
					},
				},
			},
			expected: Unstructured{
				"a": Unstructured{
					"b": []any{
						"c",
						"d",
					},
				},
				"b": []any{
					"c",
					"d",
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := merge.Merge(tt.left, tt.right)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, tt.left)
		})
	}
}
