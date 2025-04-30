// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/siderolabs/gen/ensure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/gendata"
)

// TestConfigEncodingStability ensures that the encoding of a configuration is stable as we moved forward with the config format.
func TestConfigEncodingStability(t *testing.T) {
	t.Parallel()

	// flip this to generate missing configs
	const generateMode = false

	secretsBundle, err := secrets.LoadBundle("testdata/stability/secrets.yaml")
	require.NoError(t, err)

	versionContracts := []*config.VersionContract{
		config.TalosVersion1_3,
		config.TalosVersion1_4,
		config.TalosVersion1_5,
		config.TalosVersion1_6,
		config.TalosVersion1_7,
		config.TalosVersion1_8,
		config.TalosVersion1_9,
		config.TalosVersion1_10,
		config.TalosVersion1_11,
	}

	currentVersion := ensure.Value(semver.ParseTolerant(gendata.VersionTag))
	currentVersion.Patch = 0
	maxContractVersion := ensure.Value(semver.ParseTolerant(versionContracts[len(versionContracts)-1].String()))
	require.True(t, currentVersion.LTE(maxContractVersion), "latest version contract is not tested")

	for _, versionContract := range versionContracts {
		t.Run(versionContract.String(), func(t *testing.T) {
			t.Parallel()

			t.Run("base", func(t *testing.T) {
				t.Parallel()

				in, err := generate.NewInput("base", "https://base:6443", "1.28.0",
					generate.WithSecretsBundle(secretsBundle),
					generate.WithVersionContract(versionContract),
				)
				require.NoError(t, err)

				testConfigStability(t, in, versionContract, "base", generateMode)
			})

			t.Run("with overrides", func(t *testing.T) {
				t.Parallel()

				in, err := generate.NewInput("base", "https://base:6443", "1.28.0",
					generate.WithSecretsBundle(secretsBundle),
					generate.WithVersionContract(versionContract),
					generate.WithAdditionalSubjectAltNames([]string{"foo", "bar"}),
					generate.WithAllowSchedulingOnControlPlanes(true),
					generate.WithDNSDomain("example.com"),
					generate.WithInstallDisk("/dev/vda"),
					generate.WithInstallExtraKernelArgs([]string{"foo=bar", "bar=baz"}),
					generate.WithLocalAPIServerPort(5443),
					generate.WithSysctls(map[string]string{"foo": "bar"}),
					generate.WithClusterCNIConfig(&v1alpha1.CNIConfig{
						CNIName: "custom",
						CNIUrls: []string{"https://example.com/cni.yaml"},
					}),
					generate.WithRegistryMirror("ghcr.io", "https://ghcr.io.my-mirror.com"),
				)
				require.NoError(t, err)

				patches, err := configpatcher.LoadPatches([]string{"@testdata/stability/patch.yaml"})
				require.NoError(t, err)

				testConfigStability(t, in, versionContract, "overrides", generateMode, patches...)
			})
		})
	}
}

func testConfigStability(t *testing.T, in *generate.Input, versionContract *config.VersionContract, flavor string, generateMode bool, patches ...configpatcher.Patch) {
	t.Helper()

	for _, machineType := range []machine.Type{
		machine.TypeControlPlane,
		machine.TypeWorker,
	} {
		cfg, err := in.Config(machineType)
		require.NoError(t, err)

		cfgBytes, err := cfg.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
		require.NoError(t, err)

		patched, err := configpatcher.Apply(configpatcher.WithBytes(cfgBytes), patches)
		require.NoError(t, err)

		cfgBytes, err = patched.Bytes()
		require.NoError(t, err)

		expectedPath := fmt.Sprintf("testdata/stability/%s/%s-%s.yaml", versionContract, flavor, machineType)

		expectedBytes, err := os.ReadFile(expectedPath)
		if os.IsNotExist(err) && generateMode {
			require.NoError(t, os.WriteFile(expectedPath, cfgBytes, 0o644))

			t.Logf("generated %s", expectedPath)

			continue
		}

		require.NoError(t, err)

		assert.Equal(t, string(expectedBytes), string(cfgBytes), "config encoding mismatch for %s", expectedPath)
	}
}
