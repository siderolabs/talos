// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestPartitionSpec_ResolveMaxSize(t *testing.T) {
	tests := []struct {
		name      string
		ps        *block.PartitionSpec
		available uint64
		want      uint64
		err       string
	}{
		{
			name: "absolute max size",
			ps: &block.PartitionSpec{
				MaxSize: 50,
			},
			available: 100,
			want:      50,
		},
		{
			name: "relative max size",
			ps: &block.PartitionSpec{
				RelativeMaxSize: 50,
			},
			available: 100,
			want:      50,
		},
		{
			name: "negative absolute max size",
			ps: &block.PartitionSpec{
				MaxSize:         20,
				NegativeMaxSize: true,
			},
			available: 100,
			want:      80,
		},
		{
			name: "negative relative max size",
			ps: &block.PartitionSpec{
				RelativeMaxSize: 25,
				NegativeMaxSize: true,
			},
			available: 200,
			want:      150,
		},
		{
			name:      "zero sizes",
			ps:        &block.PartitionSpec{},
			available: 100,
			want:      0,
		},
		{
			name: "negative absolute max size > available",
			ps: &block.PartitionSpec{
				MaxSize:         2500,
				NegativeMaxSize: true,
			},
			available: 200,
			err:       "partition size cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := tt.ps.ResolveMaxSize(tt.available)
			assert.Equal(t, tt.want, actual)

			if tt.err != "" {
				assert.EqualError(t, err, tt.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTmpfsSize(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		parameters []block.ParameterSpec

		expectedSize uint64
		expectedOK   bool
	}{
		{
			name: "no parameters",
		},
		{
			name: "bytes",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "1048576"),
			},
			expectedSize: 1048576,
			expectedOK:   true,
		},
		{
			name: "among other parameters",
			parameters: []block.ParameterSpec{
				block.NewStringParameter("mode", "0755"),
				block.NewBooleanParameter("noswap"),
				block.NewStringParameter(block.TmpfsSizeParameter, "4096"),
			},
			expectedSize: 4096,
			expectedOK:   true,
		},
		{
			name: "zero",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "0"),
			},
		},
		{
			name: "zero with suffix",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "0k"),
			},
		},
		{
			name: "zero percentage",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "0%"),
			},
		},
		{
			name: "suffix",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "512m"),
			},
			expectedSize: 512 << 20,
			expectedOK:   true,
		},
		{
			name: "upper case suffix",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "2G"),
			},
			expectedSize: 2 << 30,
			expectedOK:   true,
		},
		{
			name: "largest suffix",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "15e"),
			},
			expectedSize: 15 << 60,
			expectedOK:   true,
		},
		{
			name: "suffix overflow",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "16e"),
			},
		},
		{
			name: "double suffix",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "1kk"),
			},
		},
		{
			name: "suffix only",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "k"),
			},
		},
		{
			name: "unknown suffix",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "1x"),
			},
		},
		{
			name: "hexadecimal",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "0x100000"),
			},
			expectedSize: 1 << 20,
			expectedOK:   true,
		},
		{
			name: "hexadecimal digit e is not a suffix",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "0x1E00"),
			},
			expectedSize: 0x1E00 + 512,
			expectedOK:   true,
		},
		{
			name: "hexadecimal with suffix",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "0x10k"),
			},
			expectedSize: 16 << 10,
			expectedOK:   true,
		},
		{
			name: "octal",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "010000"),
			},
			expectedSize: 4096,
			expectedOK:   true,
		},
		{
			name: "invalid octal digit",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "08"),
			},
		},
		{
			name: "percentage",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "50%"),
			},
		},
		{
			name: "negative",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "-1"),
			},
		},
		{
			name: "overflow",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "18446744073709551616"),
			},
		},
		{
			name: "other parameters only",
			parameters: []block.ParameterSpec{
				block.NewStringParameter("nr_inodes", "1024"),
			},
		},
		{
			name: "nil string is skipped",
			parameters: []block.ParameterSpec{
				{Type: block.FSParameterTypeStringValue, Name: block.TmpfsSizeParameter},
				block.NewStringParameter(block.TmpfsSizeParameter, "8192"),
			},
			expectedSize: 8192,
			expectedOK:   true,
		},
		{
			name: "boolean size is skipped",
			parameters: []block.ParameterSpec{
				block.NewBooleanParameter(block.TmpfsSizeParameter),
			},
		},
		{
			name: "last size wins",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "4096"),
				block.NewStringParameter(block.TmpfsSizeParameter, "8192"),
			},
			expectedSize: 8192,
			expectedOK:   true,
		},
		{
			name: "earlier size unparsable",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "50%"),
				block.NewStringParameter(block.TmpfsSizeParameter, "8192"),
			},
			expectedSize: 8192,
			expectedOK:   true,
		},
		{
			name: "last size unparsable",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "8192"),
				block.NewStringParameter(block.TmpfsSizeParameter, "50%"),
			},
		},
		{
			name: "nil string after a size is skipped",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "8192"),
				{Type: block.FSParameterTypeStringValue, Name: block.TmpfsSizeParameter},
			},
			expectedSize: 8192,
			expectedOK:   true,
		},
		{
			name: "rounded up to a page",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "1000"),
			},
			expectedSize: 4096,
			expectedOK:   true,
		},
		{
			name: "one byte over a page",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "4097"),
			},
			expectedSize: 8192,
			expectedOK:   true,
		},
		{
			name: "rounding overflow",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "18446744073709551615"),
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			size, ok := block.TmpfsSize(test.parameters, 4096)

			assert.Equal(t, test.expectedOK, ok)
			assert.Equal(t, test.expectedSize, size)
		})
	}
}

func TestValidateTmpfsParameters(t *testing.T) {
	t.Parallel()

	size := block.NewStringParameter(block.TmpfsSizeParameter, "1048576")

	for _, test := range []struct {
		name       string
		parameters []block.ParameterSpec

		expectedError string
	}{
		{
			name:       "size",
			parameters: []block.ParameterSpec{size},
		},
		{
			name:       "size with other parameters",
			parameters: []block.ParameterSpec{size, block.NewStringParameter("nr_inodes", "1024"), block.NewBooleanParameter("noswap")},
		},
		{
			name:       "percentage",
			parameters: []block.ParameterSpec{block.NewStringParameter(block.TmpfsSizeParameter, "10%")},
		},
		{
			name:          "no parameters",
			expectedError: `tmpfs requires the "size" parameter`,
		},
		{
			name:          "no size",
			parameters:    []block.ParameterSpec{block.NewStringParameter("nr_inodes", "1024")},
			expectedError: `tmpfs requires the "size" parameter`,
		},
		{
			name:          "nil size",
			parameters:    []block.ParameterSpec{{Type: block.FSParameterTypeStringValue, Name: block.TmpfsSizeParameter}},
			expectedError: `tmpfs parameter "size" has no value`,
		},
		{
			name:          "nil string",
			parameters:    []block.ParameterSpec{size, {Type: block.FSParameterTypeStringValue, Name: "nr_inodes"}},
			expectedError: `tmpfs parameter "nr_inodes" has no value`,
		},
		{
			name:          "nil binary",
			parameters:    []block.ParameterSpec{size, block.NewBinaryParameter("key", nil)},
			expectedError: `tmpfs parameter "key" has no value`,
		},
		{
			name:          "boolean size",
			parameters:    []block.ParameterSpec{block.NewBooleanParameter(block.TmpfsSizeParameter)},
			expectedError: `tmpfs requires the "size" parameter`,
		},
		{
			name:          "nr_blocks",
			parameters:    []block.ParameterSpec{size, block.NewStringParameter("nr_blocks", "0")},
			expectedError: `tmpfs parameter "nr_blocks" is not supported, use "size" instead`,
		},
		{
			name:          "mode",
			parameters:    []block.ParameterSpec{size, block.NewStringParameter("mode", "0755")},
			expectedError: `tmpfs parameter "mode" is not supported`,
		},
		{
			name:          "uid",
			parameters:    []block.ParameterSpec{size, block.NewStringParameter("uid", "1000")},
			expectedError: `tmpfs parameter "uid" is not supported`,
		},
		{
			name:          "gid",
			parameters:    []block.ParameterSpec{size, block.NewStringParameter("gid", "1000")},
			expectedError: `tmpfs parameter "gid" is not supported`,
		},
		{
			name:       "hexadecimal size",
			parameters: []block.ParameterSpec{block.NewStringParameter(block.TmpfsSizeParameter, "0x100000")},
		},
		{
			name:       "percentage above the memory",
			parameters: []block.ParameterSpec{block.NewStringParameter(block.TmpfsSizeParameter, "150%")},
		},
		{
			name:          "zero size",
			parameters:    []block.ParameterSpec{block.NewStringParameter(block.TmpfsSizeParameter, "0")},
			expectedError: `invalid tmpfs size "0": size 0 makes the tmpfs unbounded`,
		},
		{
			name:          "zero octal size",
			parameters:    []block.ParameterSpec{block.NewStringParameter(block.TmpfsSizeParameter, "00")},
			expectedError: "size 0 makes the tmpfs unbounded",
		},
		{
			name:          "zero hexadecimal size",
			parameters:    []block.ParameterSpec{block.NewStringParameter(block.TmpfsSizeParameter, "0x0")},
			expectedError: "size 0 makes the tmpfs unbounded",
		},
		{
			name:          "zero size with suffix",
			parameters:    []block.ParameterSpec{block.NewStringParameter(block.TmpfsSizeParameter, "0G")},
			expectedError: "size 0 makes the tmpfs unbounded",
		},
		{
			name:          "zero percentage",
			parameters:    []block.ParameterSpec{block.NewStringParameter(block.TmpfsSizeParameter, "0%")},
			expectedError: "size 0 makes the tmpfs unbounded",
		},
		{
			name:          "suffix wrapping to zero",
			parameters:    []block.ParameterSpec{block.NewStringParameter(block.TmpfsSizeParameter, "16e")},
			expectedError: "size out of range",
		},
		{
			name:          "number wrapping to zero",
			parameters:    []block.ParameterSpec{block.NewStringParameter(block.TmpfsSizeParameter, "18446744073709551616")},
			expectedError: "size out of range",
		},
		{
			name:          "not a number",
			parameters:    []block.ParameterSpec{block.NewStringParameter(block.TmpfsSizeParameter, "invalid")},
			expectedError: "expected a number",
		},
		{
			name:          "suffix and percentage",
			parameters:    []block.ParameterSpec{block.NewStringParameter(block.TmpfsSizeParameter, "1k%")},
			expectedError: `unexpected "%"`,
		},
		{
			name:          "duplicate size",
			parameters:    []block.ParameterSpec{size, block.NewStringParameter(block.TmpfsSizeParameter, "2097152")},
			expectedError: `tmpfs parameter "size" is set more than once`,
		},
		{
			name:          "zero among valid sizes",
			parameters:    []block.ParameterSpec{size, block.NewStringParameter(block.TmpfsSizeParameter, "0")},
			expectedError: "size 0 makes the tmpfs unbounded",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := block.ValidateTmpfsParameters(test.parameters)

			if test.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, test.expectedError)
			}
		})
	}
}
