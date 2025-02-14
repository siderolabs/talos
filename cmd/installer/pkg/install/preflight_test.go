// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/installer/pkg/install"
)

func TestKubernetesVersionFromImageRef(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		imageRef string

		expectedVersion string
	}{
		{
			imageRef:        "ghcr.io/siderolabs/kubelet:v1.32.2",
			expectedVersion: "1.32.2",
		},
		{
			imageRef:        "ghcr.io/siderolabs/kubelet:v1.32.2@sha256:123456",
			expectedVersion: "1.32.2",
		},
	} {
		t.Run(test.imageRef, func(t *testing.T) {
			t.Parallel()

			version, err := install.KubernetesVersionFromImageRef(test.imageRef)
			require.NoError(t, err)

			assert.Equal(t, test.expectedVersion, version.String())
		})
	}
}
