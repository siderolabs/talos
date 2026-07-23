// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

//go:embed testdata/unattendedinstall.yaml
var expectedUnattendedInstallDocument []byte

func TestUnattendedInstallMarshalStability(t *testing.T) {
	cfg := runtime.NewUnattendedInstallConfigV1Alpha1()
	cfg.Installer.Image = "factory.talos.dev/metal-installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:latest"
	cfg.ProvisioningSpec.DiskSelector.Match = cel.MustExpression(cel.ParseBooleanExpression(`disk.transport == "nvme"`, celenv.VolumeLocator()))
	cfg.ProvisioningSpec.Wipe = new(true)

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedUnattendedInstallDocument, marshaled)
}

func TestUnattendedInstallConfigV1Alpha1ConflictValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name          string
		v1alpha1Cfg   *v1alpha1.Config
		expectedError string
	}{
		{
			name:        "nil v1alpha1 machine config",
			v1alpha1Cfg: &v1alpha1.Config{},
		},
		{
			name: "no legacy install config",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{},
			},
		},
		{
			name: "legacy install config conflicts",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineInstall: &v1alpha1.InstallConfig{}, //nolint:staticcheck // testing deprecated field
				},
			},
			expectedError: "UnattendedInstallConfig config is incompatible with v1alpha1 config (.machine.install)",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := runtime.NewUnattendedInstallConfigV1Alpha1().V1Alpha1ConflictValidate(test.v1alpha1Cfg)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnattendedInstallValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *runtime.UnattendedInstallConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name:          "empty",
			cfg:           runtime.NewUnattendedInstallConfigV1Alpha1,
			expectedError: "provisioning.volumeSelector.match is required",
			expectedWarnings: []string{
				"installer.image is not set, if Talos is not booted from asset built by Image Factory, installation will fail",
			},
		},
		{
			name: "no installer image",
			cfg: func() *runtime.UnattendedInstallConfigV1Alpha1 {
				cfg := runtime.NewUnattendedInstallConfigV1Alpha1()
				cfg.ProvisioningSpec.DiskSelector.Match = cel.MustExpression(cel.ParseBooleanExpression(`disk.transport == "nvme"`, celenv.VolumeLocator()))

				return cfg
			},
			expectedWarnings: []string{
				"installer.image is not set, if Talos is not booted from asset built by Image Factory, installation will fail",
			},
		},
		{
			name: "no volume selector match",
			cfg: func() *runtime.UnattendedInstallConfigV1Alpha1 {
				cfg := runtime.NewUnattendedInstallConfigV1Alpha1()
				cfg.Installer.Image = "factory.talos.dev/metal-installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:v1.0.0"

				return cfg
			},
			expectedError: "provisioning.volumeSelector.match is required",
		},
		{
			name: "invalid match expression",
			cfg: func() *runtime.UnattendedInstallConfigV1Alpha1 {
				cfg := runtime.NewUnattendedInstallConfigV1Alpha1()
				cfg.Installer.Image = "factory.talos.dev/metal-installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:v1.0.0"

				var expr cel.Expression

				require.NoError(t, expr.UnmarshalText([]byte(`1 + 2`)))

				cfg.ProvisioningSpec.DiskSelector.Match = expr

				return cfg
			},
			expectedError: "provisioning.volumeSelector.match: expression output type is int, expected bool",
		},
		{
			name: "valid config",
			cfg: func() *runtime.UnattendedInstallConfigV1Alpha1 {
				cfg := runtime.NewUnattendedInstallConfigV1Alpha1()
				cfg.Installer.Image = "factory.talos.dev/metal-installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:v1.0.0"
				cfg.ProvisioningSpec.DiskSelector.Match = cel.MustExpression(cel.ParseBooleanExpression(`disk.transport == "nvme"`, celenv.VolumeLocator()))

				return cfg
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})

			assert.Equal(t, test.expectedWarnings, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
