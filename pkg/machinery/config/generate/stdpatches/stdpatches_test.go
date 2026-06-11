// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package stdpatches_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/stdpatches"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestPatches(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		patch             func(*config.VersionContract) ([]byte, error)
		versionContracts  []*config.VersionContract
		kubernetesVersion string

		assertion func(t *testing.T, cfg config.Config)
	}{
		{
			name: "WithStaticHostname",

			patch: func(vc *config.VersionContract) ([]byte, error) {
				return stdpatches.WithStaticHostname(vc, "hostname-1")
			},

			versionContracts: []*config.VersionContract{
				config.TalosVersion1_11,
				config.TalosVersion1_12,
			},
			kubernetesVersion: "1.34.0",

			assertion: func(t *testing.T, cfg config.Config) {
				assert.Equal(t, "hostname-1", cfg.NetworkHostnameConfig().Hostname())
			},
		},
		{
			name: "WithTrustedRoots",

			patch: func(vc *config.VersionContract) ([]byte, error) {
				return stdpatches.WithTrustedRoots(vc, "trusted-roots-1")
			},

			versionContracts: []*config.VersionContract{
				config.TalosVersion1_12,
			},
			kubernetesVersion: "1.34.0",

			assertion: func(t *testing.T, cfg config.Config) {
				assert.Len(t, cfg.TrustedRoots().ExtraTrustedRootCertificates(), 1)
			},
		},
		{
			name: "WithKubeletImage",

			patch: func(vc *config.VersionContract) ([]byte, error) {
				return stdpatches.WithKubeletImage(vc, constants.KubeletImage+":v1.35.0")
			},
			versionContracts: []*config.VersionContract{
				config.TalosVersion1_13,
				config.TalosVersion1_14,
			},
			kubernetesVersion: "1.34.0",

			assertion: func(t *testing.T, cfg config.Config) {
				assert.Equal(t, constants.KubeletImage+":v1.35.0", cfg.Machine().Kubelet().Image())
			},
		},
		{
			name: "WithKubeApiServerImage",

			patch: func(vc *config.VersionContract) ([]byte, error) {
				return stdpatches.WithKubeAPIServerImage(vc, constants.KubernetesAPIServerImage+":v1.35.0")
			},
			versionContracts: []*config.VersionContract{
				config.TalosVersion1_13,
				config.TalosVersion1_14,
			},
			kubernetesVersion: "1.34.0",

			assertion: func(t *testing.T, cfg config.Config) {
				assert.Equal(t, constants.KubernetesAPIServerImage+":v1.35.0", cfg.Cluster().APIServer().Image())
			},
		},
		{
			name: "WithKubeControllerManagerImage",

			patch: func(vc *config.VersionContract) ([]byte, error) {
				return stdpatches.WithKubeControllerManagerImage(vc, constants.KubernetesControllerManagerImage+":v1.35.0")
			},
			versionContracts: []*config.VersionContract{
				config.TalosVersion1_13,
				config.TalosVersion1_14,
			},
			kubernetesVersion: "1.34.0",

			assertion: func(t *testing.T, cfg config.Config) {
				assert.Equal(t, constants.KubernetesControllerManagerImage+":v1.35.0", cfg.K8sControllerManagerConfig().Image())
			},
		},
		{
			name: "WithKubeSchedulerImage",

			patch: func(vc *config.VersionContract) ([]byte, error) {
				return stdpatches.WithKubeSchedulerImage(vc, constants.KubernetesSchedulerImage+":v1.35.0")
			},
			versionContracts: []*config.VersionContract{
				config.TalosVersion1_13,
				config.TalosVersion1_14,
			},
			kubernetesVersion: "1.34.0",

			assertion: func(t *testing.T, cfg config.Config) {
				assert.Equal(t, constants.KubernetesSchedulerImage+":v1.35.0", cfg.K8sSchedulerConfig().Image())
			},
		},
		{
			name: "WithKubeProxyImage",

			patch: func(vc *config.VersionContract) ([]byte, error) {
				return stdpatches.WithKubeProxyImage(vc, constants.KubeProxyImage+":v1.35.0")
			},
			versionContracts: []*config.VersionContract{
				config.TalosVersion1_13,
				config.TalosVersion1_14,
			},
			kubernetesVersion: "1.34.0",

			assertion: func(t *testing.T, cfg config.Config) {
				assert.Equal(t, constants.KubeProxyImage+":v1.35.0", cfg.K8sProxyConfig().Image())
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			for _, vc := range test.versionContracts {
				t.Run(vc.String(), func(t *testing.T) {
					t.Parallel()

					in, err := generate.NewInput(
						strings.ToLower(test.name),
						"https://127.0.0.1/",
						test.kubernetesVersion,
						generate.WithVersionContract(vc),
					)
					require.NoError(t, err)

					cfg, err := in.Config(machine.TypeControlPlane)
					require.NoError(t, err)

					bytesPatch, err := test.patch(vc)
					require.NoError(t, err)

					patch, err := configpatcher.LoadPatch(bytesPatch)
					require.NoError(t, err)

					patched, err := configpatcher.Apply(configpatcher.WithConfig(cfg), []configpatcher.Patch{patch})
					require.NoError(t, err)

					patchedCfg, err := patched.Config()
					require.NoError(t, err)

					_, err = patchedCfg.ValidateAsClient(mockValidationMode{}, validation.WithLocal())
					require.NoError(t, err)

					test.assertion(t, patchedCfg)
				})
			}
		})
	}
}

type mockValidationMode struct{}

func (mockValidationMode) String() string {
	return "mock"
}

func (mockValidationMode) RequiresInstall() bool {
	return false
}

func (mockValidationMode) InContainer() bool {
	return false
}
