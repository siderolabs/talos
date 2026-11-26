// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/types/container"
)

func TestContainerRunAsValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name         string
		runAs        container.ContainerRunAs
		expectedErrs []string
	}{
		{
			name:  "unset uid and gid",
			runAs: container.ContainerRunAs{},
		},
		{
			name: "valid uid and gid",
			runAs: container.ContainerRunAs{
				RunAsUID: new(uint32(65534)),
				RunAsGID: new(uint32(65534)),
			},
		},
		{
			name: "valid uid and empty gid",
			runAs: container.ContainerRunAs{
				RunAsUID: new(uint32(65534)),
			},
		},
		{
			name: "zero uid and gid",
			runAs: container.ContainerRunAs{
				RunAsUID: new(uint32(0)),
				RunAsGID: new(uint32(0)),
			},
		},
		{
			name: "highest valid uid and gid",
			runAs: container.ContainerRunAs{
				RunAsUID: new(uint32(4294967294)),
				RunAsGID: new(uint32(4294967294)),
			},
		},
		{
			name: "reserved uid",
			runAs: container.ContainerRunAs{
				RunAsUID: new(uint32(4294967295)),
			},
			expectedErrs: []string{"runAs.uid must be at most 4294967294, got 4294967295"},
		},
		{
			name: "reserved gid",
			runAs: container.ContainerRunAs{
				RunAsGID: new(uint32(4294967295)),
			},
			expectedErrs: []string{"runAs.gid must be at most 4294967294, got 4294967295"},
		},
		{
			name: "reserved uid and gid",
			runAs: container.ContainerRunAs{
				RunAsUID: new(uint32(4294967295)),
				RunAsGID: new(uint32(4294967295)),
			},
			expectedErrs: []string{
				"runAs.uid must be at most 4294967294, got 4294967295",
				"runAs.gid must be at most 4294967294, got 4294967295",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.runAs.Validate()

			if len(test.expectedErrs) == 0 {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)

			for _, expected := range test.expectedErrs {
				assert.Contains(t, err.Error(), expected)
			}
		})
	}
}
