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

					_, err = patchedCfg.Validate(mockValidationMode{}, validation.WithLocal())
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
