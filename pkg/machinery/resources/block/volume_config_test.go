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
