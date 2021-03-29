// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

type runtimeMode struct {
	requiresInstall bool
}

func (m runtimeMode) String() string {
	return fmt.Sprintf("runtimeMode(%v)", m.requiresInstall)
}

func (m runtimeMode) RequiresInstall() bool {
	return m.requiresInstall
}

func TestValidate(t *testing.T) {
	endpointURL, err := url.Parse("https://localhost:6443/")
	require.NoError(t, err)

	for _, test := range []struct {
		name            string
		config          *v1alpha1.Config
		requiresInstall bool
		expectedError   string
	}{
		{
			name: "NoMachine",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
			},
			expectedError: `1 error occurred:
	* machine instructions are required

`,
		},
		{
			name: "NoMachineInstall",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
		},
		{
			name: "NoMachineInstallRequired",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
			requiresInstall: true,
			expectedError: `1 error occurred:
	* install instructions are required in "runtimeMode(true)" mode

`,
		},
		{
			name: "MachineInstallDisk",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineInstall: &v1alpha1.InstallConfig{
						InstallDisk: "/dev/vda",
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
			requiresInstall: true,
		},
	} {
		test := test

		t.Run(test.name, func(t *testing.T) {
			err := test.config.Validate(runtimeMode{test.requiresInstall}, config.WithLocal())

			if test.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.expectedError)
			}
		})
	}
}
