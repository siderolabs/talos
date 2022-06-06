// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package slices_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/pkg/machinery/generic/slices"
)

func TestFilterInPlace(t *testing.T) {
	t.Parallel()

	type args struct {
		slice []int
	}

	tests := map[string]struct {
		args args
		want []int
	}{
		"nil": {
			args: args{
				slice: nil,
			},
			want: nil,
		},
		"empty": {
			args: args{
				slice: []int{},
			},
			want: []int{},
		},
		"single": {
			args: args{
				slice: []int{2},
			},
			want: []int{2},
		},
		"multiple": {
			args: args{
				slice: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			},
			want: []int{2, 4, 6, 8, 10},
		},
		"multiple to empty": {
			args: args{
				slice: []int{1, 3, 5, 7, 9, 11, 13, 15, 17, 19},
			},
			want: []int{},
		},
		"multiple to single": {
			args: args{
				slice: []int{1, 3, 5, 7, 9, 11, 12, 13, 15, 17, 19},
			},
			want: []int{12},
		},
		"preserve all": {
			args: args{
				slice: []int{2, 4, 6, 8, 10},
			},
			want: []int{2, 4, 6, 8, 10},
		},
	}

	for name, tt := range tests {
		tt := tt

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := slices.FilterInPlace(tt.args.slice, func(i int) bool {
				return i%2 == 0
			})
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFilter(t *testing.T) {
	t.Parallel()

	type args struct {
		slice []int
	}

	tests := map[string]struct {
		args args
		want []int
	}{
		"nil": {
			args: args{
				slice: nil,
			},
			want: nil,
		},
		"empty": {
			args: args{
				slice: []int{},
			},
			want: nil,
		},
		"single": {
			args: args{
				slice: []int{2},
			},
			want: []int{2},
		},
		"multiple": {
			args: args{
				slice: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			},
			want: []int{2, 4, 6, 8, 10},
		},
		"multiple to empty": {
			args: args{
				slice: []int{1, 3, 5, 7, 9, 11, 13, 15, 17, 19},
			},
			want: nil,
		},
		"multiple to single": {
			args: args{
				slice: []int{1, 3, 5, 7, 9, 11, 12, 13, 15, 17, 19},
			},
			want: []int{12},
		},
		"preserve all": {
			args: args{
				slice: []int{2, 4, 6, 8, 10},
			},
			want: []int{2, 4, 6, 8, 10},
		},
	}

	for name, tt := range tests {
		tt := tt

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := slices.Filter(tt.args.slice, func(i int) bool {
				return i%2 == 0
			})
			assert.Equal(t, tt.want, got)
		})
	}
}
