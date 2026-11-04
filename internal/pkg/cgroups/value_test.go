// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroups_test

import (
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/cgroups"
)

func TestParseValue(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		in string

		expected        cgroups.Value
		expectedFloat64 float64
	}{
		{
			in:              "42",
			expected:        cgroups.Value{Val: 42, IsSet: true},
			expectedFloat64: 42,
		},
		{
			in:              "max",
			expected:        cgroups.Value{IsMax: true, IsSet: true},
			expectedFloat64: math.Inf(1),
		},
		{
			in:              "42.5",
			expected:        cgroups.Value{Val: 425, Frac: 1, IsSet: true},
			expectedFloat64: 42.5,
		},
		{
			in:       "0.00",
			expected: cgroups.Value{Val: 0, Frac: 2, IsSet: true},
		},
	} {
		t.Run(test.in, func(t *testing.T) {
			t.Parallel()

			v, err := cgroups.ParseValue(test.in)
			require.NoError(t, err)

			assert.Equal(t, test.expected, v)
			assert.Equal(t, test.expectedFloat64, v.Float64())

			assert.Equal(t, test.in, v.String())

			v, err = cgroups.ParseValueBytes([]byte(test.in))
			require.NoError(t, err)

			assert.Equal(t, test.expected, v)
		})
	}
}

func TestParseValueEdgeCases(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		in string

		expected  cgroups.Value
		expectErr bool
	}{
		{in: "-5", expected: cgroups.Value{Val: -5, IsSet: true}},
		{in: "+5", expected: cgroups.Value{Val: 5, IsSet: true}},
		{in: "-0.25", expected: cgroups.Value{Val: -25, Frac: 2, IsSet: true}},
		{in: "1.", expected: cgroups.Value{Val: 1, IsSet: true}},
		{in: ".5", expected: cgroups.Value{Val: 5, Frac: 1, IsSet: true}},
		{in: "9223372036854775807", expected: cgroups.Value{Val: math.MaxInt64, IsSet: true}},
		{in: "-9223372036854775808", expected: cgroups.Value{Val: math.MinInt64, IsSet: true}},
		{in: "9223372036854775808", expectErr: true},
		{in: "", expectErr: true},
		{in: "-", expectErr: true},
		{in: ".", expectErr: true},
		{in: "1.2.3", expectErr: true},
		{in: "abc", expectErr: true},
		{in: "12 ", expectErr: true},
	} {
		t.Run(test.in, func(t *testing.T) {
			t.Parallel()

			v, err := cgroups.ParseValue(test.in)
			vb, errb := cgroups.ParseValueBytes([]byte(test.in))

			if test.expectErr {
				require.Error(t, err)
				require.Error(t, errb)
				assert.Equal(t, err.Error(), errb.Error())

				return
			}

			require.NoError(t, err)
			require.NoError(t, errb)

			assert.Equal(t, test.expected, v)
			assert.Equal(t, test.expected, vb)
		})
	}
}

func TestParseValueBytesAllocs(t *testing.T) {
	for _, in := range []string{"42", "max", "17.06", "-0.25", "12172340"} {
		b := []byte(in)

		allocs := testing.AllocsPerRun(100, func() {
			if _, err := cgroups.ParseValueBytes(b); err != nil {
				t.Fatal(err)
			}
		})

		assert.Zero(t, allocs, "input %q", in)
	}
}

func TestParseNewlineSeparatedValues(t *testing.T) { //nolint:dupl
	t.Parallel()

	for _, test := range []struct {
		name     string
		input    string
		expected cgroups.Values
	}{
		{
			name:  "one",
			input: "42\n",
			expected: cgroups.Values{
				{Val: 42, IsSet: true},
			},
		},
		{
			name:  "two",
			input: "42\n43\n",
			expected: cgroups.Values{
				{Val: 42, IsSet: true},
				{Val: 43, IsSet: true},
			},
		},
		{
			name:  "max",
			input: "max\n",
			expected: cgroups.Values{
				{IsMax: true, IsSet: true},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			r := strings.NewReader(test.input)

			values, err := cgroups.ParseNewlineSeparatedValues(r)
			require.NoError(t, err)
			require.Equal(t, test.expected, values)
		})
	}
}

func TestParseSpaceSeparatedValues(t *testing.T) { //nolint:dupl
	t.Parallel()

	for _, test := range []struct {
		name     string
		input    string
		expected cgroups.Values
	}{
		{
			name:  "one",
			input: "42\n",
			expected: cgroups.Values{
				{Val: 42, IsSet: true},
			},
		},
		{
			name:  "two",
			input: "42 43\n",
			expected: cgroups.Values{
				{Val: 42, IsSet: true},
				{Val: 43, IsSet: true},
			},
		},
		{
			name:  "max",
			input: "max\n",
			expected: cgroups.Values{
				{IsMax: true, IsSet: true},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			r := strings.NewReader(test.input)

			values, err := cgroups.ParseSpaceSeparatedValues(r)
			require.NoError(t, err)
			require.Equal(t, test.expected, values)
		})
	}
}

func TestParseFlatMapValues(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		input    string
		expected cgroups.FlatMap
	}{
		{
			name:  "values",
			input: "anon 3000\nfile 6000\n",
			expected: cgroups.FlatMap{
				"anon": {Val: 3000, IsSet: true},
				"file": {Val: 6000, IsSet: true},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			r := strings.NewReader(test.input)

			values, err := cgroups.ParseFlatMapValues(r)
			require.NoError(t, err)
			require.Equal(t, test.expected, values)
		})
	}
}

func TestParseNestedKeyedValues(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		input    string
		expected cgroups.NestedKeyed
	}{
		{
			name:  "values",
			input: "anon rss=3125 vms=123\nreal rss=1234 vms=5678\n",
			expected: cgroups.NestedKeyed{
				"anon": {
					"rss": {Val: 3125, IsSet: true},
					"vms": {Val: 123, IsSet: true},
				},
				"real": {
					"rss": {Val: 1234, IsSet: true},
					"vms": {Val: 5678, IsSet: true},
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			r := strings.NewReader(test.input)

			values, err := cgroups.ParseNestedKeyedValues(r)
			require.NoError(t, err)
			require.Equal(t, test.expected, values)
		})
	}
}
