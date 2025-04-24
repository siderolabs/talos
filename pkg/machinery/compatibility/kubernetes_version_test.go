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

func TestKubernetesCompatibility12(t *testing.T) {
	for _, tt := range []kubernetesVersionTest{
		{
			kubernetesVersion: "1.23.1",
			target:            "1.2.0",
		},
		{
			kubernetesVersion: "1.24.3",
			target:            "1.2.0-beta.0",
		},
		{
			kubernetesVersion: "1.25.0-rc.0",
			target:            "1.2.7",
		},
		{
			kubernetesVersion: "1.26.0-alpha.0",
			target:            "1.2.0",
			expectedError:     "version of Kubernetes 1.26.0-alpha.0 is too new to be used with Talos 1.2.0",
		},
		{
			kubernetesVersion: "1.22.4",
			target:            "1.2.0",
			expectedError:     "version of Kubernetes 1.22.4 is too old to be used with Talos 1.2.0",
		},
	} {
		runKubernetesVersionTest(t, tt)
	}
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

func TestKubernetesCompatibility14(t *testing.T) {
	for _, tt := range []kubernetesVersionTest{
		{
			kubernetesVersion: "1.25.1",
			target:            "1.4.0",
		},
		{
			kubernetesVersion: "1.26.3",
			target:            "1.4.0-beta.0",
		},
		{
			kubernetesVersion: "1.27.0-rc.0",
			target:            "1.4.7",
		},
		{
			kubernetesVersion: "1.28.0-alpha.0",
			target:            "1.4.0",
			expectedError:     "version of Kubernetes 1.28.0-alpha.0 is too new to be used with Talos 1.4.0",
		},
		{
			kubernetesVersion: "1.24.1",
			target:            "1.4.0",
			expectedError:     "version of Kubernetes 1.24.1 is too old to be used with Talos 1.4.0",
		},
	} {
		runKubernetesVersionTest(t, tt)
	}
}

func TestKubernetesCompatibility15(t *testing.T) {
	for _, tt := range []kubernetesVersionTest{
		{
			kubernetesVersion: "1.26.1",
			target:            "1.5.0",
		},
		{
			kubernetesVersion: "1.27.3",
			target:            "1.5.0-beta.0",
		},
		{
			kubernetesVersion: "1.28.0-rc.0",
			target:            "1.5.7",
		},
		{
			kubernetesVersion: "1.29.0-alpha.0",
			target:            "1.5.0",
			expectedError:     "version of Kubernetes 1.29.0-alpha.0 is too new to be used with Talos 1.5.0",
		},
		{
			kubernetesVersion: "1.25.1",
			target:            "1.5.0",
			expectedError:     "version of Kubernetes 1.25.1 is too old to be used with Talos 1.5.0",
		},
	} {
		runKubernetesVersionTest(t, tt)
	}
}

func TestKubernetesCompatibility16(t *testing.T) {
	for _, tt := range []kubernetesVersionTest{
		{
			kubernetesVersion: "1.27.1",
			target:            "1.6.0",
		},
		{
			kubernetesVersion: "1.24.1",
			target:            "1.6.0",
		},
		{
			kubernetesVersion: "1.28.3",
			target:            "1.6.0-beta.0",
		},
		{
			kubernetesVersion: "1.29.0-rc.0",
			target:            "1.6.7",
		},
		{
			kubernetesVersion: "1.30.0-alpha.0",
			target:            "1.6.0",
			expectedError:     "version of Kubernetes 1.30.0-alpha.0 is too new to be used with Talos 1.6.0",
		},
		{
			kubernetesVersion: "1.23.1",
			target:            "1.6.0",
			expectedError:     "version of Kubernetes 1.23.1 is too old to be used with Talos 1.6.0",
		},
	} {
		runKubernetesVersionTest(t, tt)
	}
}

func TestKubernetesCompatibility17(t *testing.T) {
	for _, tt := range []kubernetesVersionTest{
		{
			kubernetesVersion: "1.27.1",
			target:            "1.7.0",
		},
		{
			kubernetesVersion: "1.25.1",
			target:            "1.7.0",
		},
		{
			kubernetesVersion: "1.28.3",
			target:            "1.7.0-beta.0",
		},
		{
			kubernetesVersion: "1.30.0-rc.0",
			target:            "1.7.7",
		},
		{
			kubernetesVersion: "1.31.0-alpha.0",
			target:            "1.7.0",
			expectedError:     "version of Kubernetes 1.31.0-alpha.0 is too new to be used with Talos 1.7.0",
		},
		{
			kubernetesVersion: "1.24.1",
			target:            "1.7.0",
			expectedError:     "version of Kubernetes 1.24.1 is too old to be used with Talos 1.7.0",
		},
	} {
		runKubernetesVersionTest(t, tt)
	}
}

func TestKubernetesCompatibility18(t *testing.T) {
	for _, tt := range []kubernetesVersionTest{
		{
			kubernetesVersion: "1.27.1",
			target:            "1.8.0",
		},
		{
			kubernetesVersion: "1.26.1",
			target:            "1.8.0",
		},
		{
			kubernetesVersion: "1.30.3",
			target:            "1.8.0-beta.0",
		},
		{
			kubernetesVersion: "1.31.0-rc.0",
			target:            "1.8.7",
		},
		{
			kubernetesVersion: "1.32.0-alpha.0",
			target:            "1.8.0",
			expectedError:     "version of Kubernetes 1.32.0-alpha.0 is too new to be used with Talos 1.8.0",
		},
		{
			kubernetesVersion: "1.25.1",
			target:            "1.8.0",
			expectedError:     "version of Kubernetes 1.25.1 is too old to be used with Talos 1.8.0",
		},
	} {
		runKubernetesVersionTest(t, tt)
	}
}

func TestKubernetesCompatibility19(t *testing.T) {
	for _, tt := range []kubernetesVersionTest{
		{
			kubernetesVersion: "1.28.1",
			target:            "1.9.0",
		},
		{
			kubernetesVersion: "1.27.1",
			target:            "1.9.0",
		},
		{
			kubernetesVersion: "1.31.3",
			target:            "1.9.0-beta.0",
		},
		{
			kubernetesVersion: "1.32.0-rc.0",
			target:            "1.9.7",
		},
		{
			kubernetesVersion: "1.33.0-alpha.0",
			target:            "1.9.0",
			expectedError:     "version of Kubernetes 1.33.0-alpha.0 is too new to be used with Talos 1.9.0",
		},
		{
			kubernetesVersion: "1.26.1",
			target:            "1.9.0",
			expectedError:     "version of Kubernetes 1.26.1 is too old to be used with Talos 1.9.0",
		},
	} {
		runKubernetesVersionTest(t, tt)
	}
}

func TestKubernetesCompatibility110(t *testing.T) {
	for _, tt := range []kubernetesVersionTest{
		{
			kubernetesVersion: "1.29.1",
			target:            "1.10.0",
		},
		{
			kubernetesVersion: "1.28.1",
			target:            "1.10.0",
		},
		{
			kubernetesVersion: "1.32.3",
			target:            "1.10.0-beta.0",
		},
		{
			kubernetesVersion: "1.33.0-rc.0",
			target:            "1.10.7",
		},
		{
			kubernetesVersion: "1.34.0-alpha.0",
			target:            "1.10.0",
			expectedError:     "version of Kubernetes 1.34.0-alpha.0 is too new to be used with Talos 1.10.0",
		},
		{
			kubernetesVersion: "1.27.1",
			target:            "1.10.0",
			expectedError:     "version of Kubernetes 1.27.1 is too old to be used with Talos 1.10.0",
		},
	} {
		runKubernetesVersionTest(t, tt)
	}
}

func TestKubernetesCompatibility111(t *testing.T) {
	for _, tt := range []kubernetesVersionTest{
		{
			kubernetesVersion: "1.30.1",
			target:            "1.11.0",
		},
		{
			kubernetesVersion: "1.29.1",
			target:            "1.11.0",
		},
		{
			kubernetesVersion: "1.33.3",
			target:            "1.11.0-beta.0",
		},
		{
			kubernetesVersion: "1.34.0-rc.0",
			target:            "1.11.7",
		},
		{
			kubernetesVersion: "1.35.0-alpha.0",
			target:            "1.11.0",
			expectedError:     "version of Kubernetes 1.35.0-alpha.0 is too new to be used with Talos 1.11.0",
		},
		{
			kubernetesVersion: "1.28.1",
			target:            "1.11.0",
			expectedError:     "version of Kubernetes 1.28.1 is too old to be used with Talos 1.11.0",
		},
	} {
		runKubernetesVersionTest(t, tt)
	}
}

func TestKubernetesCompatibilityUnsupported(t *testing.T) {
	for _, tt := range []kubernetesVersionTest{
		{
			kubernetesVersion: "1.25.0",
			target:            "1.12.0-alpha.0",
			expectedError:     "compatibility with version 1.12.0-alpha.0 is not supported",
		},
		{
			kubernetesVersion: "1.25.0",
			target:            "1.1.0",
			expectedError:     "compatibility with version 1.1.0 is not supported",
		},
	} {
		runKubernetesVersionTest(t, tt)
	}
}
