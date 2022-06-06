// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package maps_test

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/pkg/machinery/generic/maps"
)

func TestFilterInPlace(t *testing.T) {
	t.Parallel()

	type args struct {
		m map[string]string
	}

	tests := map[string]struct {
		args args
		want map[string]string
	}{
		"nil": {
			args: args{
				m: nil,
			},
			want: nil,
		},
		"empty": {
			args: args{
				m: map[string]string{},
			},
			want: map[string]string{},
		},
		"single": {
			args: args{
				m: map[string]string{"foo": "b"},
			},
			want: map[string]string{"foo": "b"},
		},
		"multiple": {
			args: args{
				m: map[string]string{"foo": "b", "bar": "c", "baz": "d"},
			},
			want: map[string]string{"foo": "b"},
		},
		"multiple to empty": {
			args: args{
				m: map[string]string{"far": "b", "bar": "c", "baz": "d"},
			},
			want: map[string]string{},
		},
	}

	for name, tt := range tests {
		tt := tt

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := maps.FilterInPlace(tt.args.m, func(k, v string) bool { return k == "foo" })
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFilter(t *testing.T) {
	t.Parallel()

	type args struct {
		m map[string]string
	}

	tests := map[string]struct {
		args args
		want map[string]string
	}{
		"nil": {
			args: args{
				m: nil,
			},
			want: nil,
		},
		"empty": {
			args: args{
				m: map[string]string{},
			},
			want: nil,
		},
		"single": {
			args: args{
				m: map[string]string{"foo": "b"},
			},
			want: map[string]string{"foo": "b"},
		},
		"multiple": {
			args: args{
				m: map[string]string{"foo": "b", "bar": "c", "baz": "d"},
			},
			want: map[string]string{"foo": "b"},
		},
		"multiple to empty": {
			args: args{
				m: map[string]string{"far": "b", "bar": "c", "baz": "d"},
			},
			want: nil,
		},
	}

	for name, tt := range tests {
		tt := tt

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := maps.Filter(tt.args.m, func(k, v string) bool { return k == "foo" })
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestKeys(t *testing.T) {
	t.Parallel()

	type args struct {
		m map[string]string
	}

	tests := map[string]struct {
		args args
		want []string
	}{
		"nil": {
			args: args{
				m: nil,
			},
			want: nil,
		},
		"empty": {
			args: args{
				m: map[string]string{},
			},
			want: nil,
		},
		"single": {
			args: args{
				m: map[string]string{"foo": "b"},
			},
			want: []string{"foo"},
		},
		"multiple": {
			args: args{
				m: map[string]string{"foo": "b", "bar": "c", "baz": "d"},
			},
			want: []string{"bar", "baz", "foo"},
		},
	}

	for name, tt := range tests {
		tt := tt

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := maps.Keys(tt.args.m)
			sort.Strings(got)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestKeysFunc(t *testing.T) {
	t.Parallel()

	type args struct {
		m map[string]string
	}

	tests := map[string]struct {
		args args
		want []string
	}{
		"nil": {
			args: args{
				m: nil,
			},
			want: nil,
		},
		"empty": {
			args: args{
				m: map[string]string{},
			},
			want: nil,
		},
		"single": {
			args: args{
				m: map[string]string{"foo": "b"},
			},
			want: []string{"foo func"},
		},
		"multiple": {
			args: args{
				m: map[string]string{"foo": "b", "bar": "c", "baz": "d"},
			},
			want: []string{"bar func", "baz func", "foo func"},
		},
	}

	for name, tt := range tests {
		tt := tt

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := maps.KeysFunc(tt.args.m, func(k string) string { return k + " func" })
			sort.Strings(got)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToSlice(t *testing.T) {
	t.Parallel()

	type args struct {
		m map[string]string
	}

	tests := map[string]struct {
		args args
		want []string
	}{
		"nil": {
			args: args{
				m: nil,
			},
			want: nil,
		},
		"empty": {
			args: args{
				m: map[string]string{},
			},
			want: nil,
		},
		"single": {
			args: args{
				m: map[string]string{"foo": "b"},
			},
			want: []string{"foo b"},
		},
		"multiple": {
			args: args{
				m: map[string]string{"foo": "b", "bar": "c", "baz": "d"},
			},
			want: []string{"bar c", "baz d", "foo b"},
		},
	}

	for name, tt := range tests {
		tt := tt

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := maps.ToSlice(tt.args.m, func(k, v string) string { return k + " " + v })
			sort.Strings(got)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValuesFunc(t *testing.T) {
	t.Parallel()

	type args struct {
		m map[string]string
	}

	tests := map[string]struct {
		args args
		want []string
	}{
		"nil": {
			args: args{
				m: nil,
			},
			want: nil,
		},
		"empty": {
			args: args{
				m: map[string]string{},
			},
			want: nil,
		},
		"single": {
			args: args{
				m: map[string]string{"foo": "b"},
			},
			want: []string{"b"},
		},
		"multiple": {
			args: args{
				m: map[string]string{"foo": "b", "bar": "c", "baz": "d"},
			},
			want: []string{"b", "c", "d"},
		},
	}

	for name, tt := range tests {
		tt := tt

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := maps.ValuesFunc(tt.args.m, func(v string) string { return v })
			sort.Strings(got)
			assert.Equal(t, tt.want, got)
		})
	}
}
