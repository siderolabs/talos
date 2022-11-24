// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package compatibility_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/compatibility"
)

type kubernetesVersionTest struct {
	kubernetesVersion string
	target            string
	expectedError     string
}

func runKubernetesVersionTest(t *testing.T, tt kubernetesVersionTest) {
	t.Run(tt.kubernetesVersion+" -> "+tt.target, func(t *testing.T) {
		k8sVersion, err := compatibility.ParseKubernetesVersion(tt.kubernetesVersion)
		require.NoError(t, err)

		target, err := compatibility.ParseTalosVersion(&machine.VersionInfo{
			Tag: tt.target,
		})
		require.NoError(t, err)

		err = k8sVersion.SupportedWith(target)
		if tt.expectedError != "" {
			require.EqualError(t, err, tt.expectedError)
		} else {
			require.NoError(t, err)
		}
	})
}

func TestKubernetesCompatibility13(t *testing.T) {
	for _, tt := range []kubernetesVersionTest{
		{
			kubernetesVersion: "1.24.1",
			target:            "1.3.0",
		},
		{
			kubernetesVersion: "1.25.3",
			target:            "1.3.0-beta.0",
		},
		{
			kubernetesVersion: "1.26.0-rc.0",
			target:            "1.3.7",
		},
		{
			kubernetesVersion: "1.27.0-alpha.0",
			target:            "1.3.0",
			expectedError:     "version of Kubernetes 1.27.0-alpha.0 is too new to be used with Talos 1.3.0",
		},
		{
			kubernetesVersion: "1.23.4",
			target:            "1.3.0",
			expectedError:     "version of Kubernetes 1.23.4 is too old to be used with Talos 1.3.0",
		},
	} {
		runKubernetesVersionTest(t, tt)
	}
}

func TestKubernetesCompatibilityUnsupported(t *testing.T) {
	for _, tt := range []kubernetesVersionTest{
		{
			kubernetesVersion: "1.25.0",
			target:            "1.4.0-alpha.0",
			expectedError:     "compatibility with version 1.4.0-alpha.0 is not supported",
		},
		{
			kubernetesVersion: "1.25.0",
			target:            "1.2.0",
			expectedError:     "compatibility with version 1.2.0 is not supported",
		},
	} {
		runKubernetesVersionTest(t, tt)
	}
}
