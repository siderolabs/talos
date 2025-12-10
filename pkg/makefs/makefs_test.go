// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makefs_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/makefs"
)

func TestPartitionGUIDFromLabel(t *testing.T) {
	tests := []struct {
		label    string
		expected string
	}{
		{
			label:    constants.EFIPartitionLabel,
			expected: "bca5174b-0118-8a6a-af0b-2c6e1585acae",
		},
		{
			label:    constants.BootPartitionLabel,
			expected: "24c73076-4092-85dd-9d98-686a2dbf6d81",
		},
		{
			label:    constants.MetaPartitionLabel,
			expected: "cf04f137-101e-8ec6-9627-fa3f51cdabb8",
		},
		{
			label:    constants.ImageCachePartitionLabel,
			expected: "3b5449bc-c21f-8009-8b51-18945967f8df",
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			require.Equal(t, tt.expected, makefs.GUIDFromLabel(tt.label).String())
		})
	}
}
