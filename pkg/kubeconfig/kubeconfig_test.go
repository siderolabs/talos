// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeconfig_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/talos-systems/talos/pkg/kubeconfig"
)

func TestSinglePath(t *testing.T) {
	expectedDefaultPath, err := kubeconfig.DefaultPath()
	assert.NoError(t, err)

	for _, tt := range []struct {
		name      string
		envVar    string
		shouldErr bool
		expected  string
	}{
		{
			name:      "NoKUBECONFIGSet",
			shouldErr: false,
			expected:  expectedDefaultPath,
		},
		{
			name:      "UseKUBECONFIGSet",
			envVar:    "/my/custom/path/to/kubeconfig",
			shouldErr: false,
			expected:  "/my/custom/path/to/kubeconfig",
		},
		{
			name:      "MultiKUBECONFIGSet",
			envVar:    "/my/custom/path/to/kubeconfig:/another/path/to/kubeconfig:/foo/bar/kubeconfig",
			shouldErr: true,
		},
	} {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			if tt.envVar != "" {
				t.Setenv("KUBECONFIG", tt.envVar)
			}
			result, err := kubeconfig.SinglePath()

			if tt.shouldErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.expected, result)
		})
	}
}
