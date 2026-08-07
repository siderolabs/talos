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
			name: "valid non-negative uid and gid",
			runAs: container.ContainerRunAs{
				RunAsUID: new(int32(65534)),
				RunAsGID: new(int32(65534)),
			},
		},
		{
			name: "valid non-negative uid and empty gid",
			runAs: container.ContainerRunAs{
				RunAsUID: new(int32(65534)),
			},
		},
		{
			name: "zero uid and gid",
			runAs: container.ContainerRunAs{
				RunAsUID: new(int32(0)),
				RunAsGID: new(int32(0)),
			},
		},
		{
			name: "negative uid",
			runAs: container.ContainerRunAs{
				RunAsUID: new(int32(-1)),
			},
			expectedErrs: []string{"runAs.uid must be non-negative, got -1"},
		},
		{
			name: "negative gid",
			runAs: container.ContainerRunAs{
				RunAsGID: new(int32(-1)),
			},
			expectedErrs: []string{"runAs.gid must be non-negative, got -1"},
		},
		{
			name: "negative uid and gid",
			runAs: container.ContainerRunAs{
				RunAsUID: new(int32(-1)),
				RunAsGID: new(int32(-2)),
			},
			expectedErrs: []string{
				"runAs.uid must be non-negative, got -1",
				"runAs.gid must be non-negative, got -2",
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
