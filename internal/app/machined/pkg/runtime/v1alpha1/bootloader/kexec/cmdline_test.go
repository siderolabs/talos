// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kexec_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/kexec"
)

func TestAppendBootPartitionUUID(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cmdline       string
		partitionUUID string
		expected      string
	}{
		{
			name:          "appended",
			cmdline:       "talos.platform=metal console=ttyS0",
			partitionUUID: "6f5a6e8a-9c79-4c0f-8f35-0c2a1f1a0001",
			expected:      "talos.platform=metal console=ttyS0 talos.boot.partuuid=6f5a6e8a-9c79-4c0f-8f35-0c2a1f1a0001",
		},
		{
			name:          "replaced",
			cmdline:       "talos.platform=metal talos.boot.partuuid=00000000-0000-0000-0000-000000000000 console=ttyS0",
			partitionUUID: "6f5a6e8a-9c79-4c0f-8f35-0c2a1f1a0001",
			expected:      "talos.platform=metal talos.boot.partuuid=6f5a6e8a-9c79-4c0f-8f35-0c2a1f1a0001 console=ttyS0",
		},
		{
			name:     "unknown",
			cmdline:  "talos.platform=metal",
			expected: "talos.platform=metal",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, kexec.AppendBootPartitionUUID(test.cmdline, test.partitionUUID))
		})
	}
}
