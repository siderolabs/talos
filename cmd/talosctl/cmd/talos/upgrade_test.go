// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos //nolint:testpackage // to test unexported commands and helpers

import (
	"errors"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// parseUpgradeFlags parses args into the upgrade command flags and restores the defaults once the test is done.
func parseUpgradeFlags(t *testing.T, args ...string) *pflag.FlagSet {
	t.Helper()

	t.Cleanup(func() {
		for _, name := range []string{
			upgradeImageFlagName,
			imageFactoryHostFlagName,
			schematicIDFlagName,
			talosVersionFlagName,
			secureBootFlagName,
			platformFlagName,
		} {
			flag := upgradeCmd.Flags().Lookup(name)
			require.NoError(t, flag.Value.Set(flag.DefValue))

			flag.Changed = false
		}
	})

	require.NoError(t, upgradeCmd.ParseFlags(args))

	return upgradeCmd.Flags()
}

func TestGetNodeInstallImageMapUniform(t *testing.T) {
	nodes := []string{"10.5.0.2", "10.5.0.3"}

	for _, tc := range []struct {
		name          string
		args          []string
		expectedImage string
	}{
		{
			name:          "image",
			args:          []string{"--image", "ghcr.io/siderolabs/installer:v1.13.0"},
			expectedImage: "ghcr.io/siderolabs/installer:v1.13.0",
		},
		{
			name: "all component flags",
			args: []string{
				"--image-factory-host", "factory.talos.dev",
				"--schematic-id", "abc",
				"--platform", "metal",
				"--secure-boot",
				"--talos-version", "v1.13.0",
			},
			expectedImage: "factory.talos.dev/metal-installer-secureboot/abc:v1.13.0",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flags := parseUpgradeFlags(t, tc.args...)

			// no client factory: the uniform path must not talk to the nodes
			imageMap, err := getNodeInstallImageMap(t.Context(), nodes, nil, flags)
			require.NoError(t, err)
			assert.Equal(t, map[string]string{
				nodes[0]: tc.expectedImage,
				nodes[1]: tc.expectedImage,
			}, imageMap)
		})
	}
}

func TestImageFlagConflicts(t *testing.T) {
	for _, flag := range imageComponentFlagNames {
		t.Run(flag, func(t *testing.T) {
			parseUpgradeFlags(t, "--image", "ghcr.io/siderolabs/installer:v1.13.0", "--"+flag+"=1")

			err := upgradeCmd.ValidateFlagGroups()
			require.ErrorContains(t, err, upgradeImageFlagName)
			require.ErrorContains(t, err, flag)
		})
	}

	t.Run("image only", func(t *testing.T) {
		parseUpgradeFlags(t, "--image", "ghcr.io/siderolabs/installer:v1.13.0")

		require.NoError(t, upgradeCmd.ValidateFlagGroups())
	})
}

func TestReadNodeImageParams(t *testing.T) {
	for _, tc := range []struct {
		name     string
		args     []string
		expected nodeImageParams
	}{
		{
			name: "from node state",
			expected: nodeImageParams{
				factoryHost: "factory.example.com",
				schematic:   "node-schematic",
				platform:    "aws",
				version:     upgradeCmdFlags.talosVersion,
				secureBoot:  true,
			},
		},
		{
			name: "flags override node state",
			args: []string{"--schematic-id", "flag-schematic", "--platform", "metal", "--secure-boot=false"},
			expected: nodeImageParams{
				factoryHost: "factory.example.com",
				schematic:   "flag-schematic",
				platform:    "metal",
				version:     upgradeCmdFlags.talosVersion,
				secureBoot:  false,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flags := parseUpgradeFlags(t, tc.args...)
			st := newNodeImageState(t)

			p, err := readNodeImageParams(t.Context(), st, imageParamsFromFlags(), newChangedImageFlags(flags))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, p)
		})
	}
}

func TestReadNodeImageParamsSkipsOverriddenReads(t *testing.T) {
	flags := parseUpgradeFlags(t,
		"--image-factory-host", "factory.talos.dev",
		"--schematic-id", "abc",
		"--platform", "metal",
		"--secure-boot",
	)

	// empty state so any read would fail with NotFound
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	p, err := readNodeImageParams(t.Context(), st, imageParamsFromFlags(), newChangedImageFlags(flags))
	require.NoError(t, err)
	assert.Equal(t, nodeImageParams{
		factoryHost: "factory.talos.dev",
		schematic:   "abc",
		platform:    "metal",
		version:     upgradeCmdFlags.talosVersion,
		secureBoot:  true,
	}, p)
}

func TestReadNodeImageParamsNotFromFactory(t *testing.T) {
	flags := parseUpgradeFlags(t)

	// empty state: the node has no ImageFactorySchematic
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	_, err := readNodeImageParams(t.Context(), st, imageParamsFromFlags(), newChangedImageFlags(flags))
	require.ErrorContains(t, err, "not installed from Image Factory")
	require.ErrorContains(t, err, "--image")
}

func TestReadNodeImageParamsEmptyFactoryURL(t *testing.T) {
	flags := parseUpgradeFlags(t)
	st := newNodeImageState(t)

	schematic := runtime.NewImageFactorySchematic(runtime.NamespaceName, runtime.ImageFactorySchematicID)
	schematic.TypedSpec().SchematicID = "node-schematic"
	require.NoError(t, st.Destroy(t.Context(), schematic.Metadata()))
	require.NoError(t, st.Create(t.Context(), schematic))

	p, err := readNodeImageParams(t.Context(), st, imageParamsFromFlags(), newChangedImageFlags(flags))
	require.NoError(t, err)
	assert.Equal(t, defaultImageFactoryHost, p.factoryHost)
	assert.True(t, p.defaultedFactoryHost)
}

func newNodeImageState(t *testing.T) state.State {
	t.Helper()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	schematic := runtime.NewImageFactorySchematic(runtime.NamespaceName, runtime.ImageFactorySchematicID)
	schematic.TypedSpec().SchematicID = "node-schematic"
	schematic.TypedSpec().APIURL = "https://factory.example.com"
	require.NoError(t, st.Create(t.Context(), schematic))

	platform := runtime.NewPlatformMetadataSpec(runtime.NamespaceName, runtime.PlatformMetadataID)
	platform.TypedSpec().Platform = "aws"
	require.NoError(t, st.Create(t.Context(), platform))

	security := runtime.NewSecurityStateSpec(runtime.NamespaceName)
	security.TypedSpec().SecureBoot = true
	require.NoError(t, st.Create(t.Context(), security))

	return st
}

func TestRequireImageFlag(t *testing.T) {
	versionErr := errors.New("10.5.0.2: server version 1.13.0 is outside the desired range")

	t.Run("image set", func(t *testing.T) {
		parseUpgradeFlags(t, "--image", "ghcr.io/siderolabs/installer:v1.13.0")

		require.NoError(t, requireImageFlag(versionErr))
	})

	t.Run("image not set", func(t *testing.T) {
		parseUpgradeFlags(t)

		err := requireImageFlag(versionErr)
		require.ErrorContains(t, err, "--image flag is required")
		require.ErrorContains(t, err, "10.5.0.2: server version 1.13.0")

		require.ErrorContains(t, requireImageFlag(nil), "--image flag is required")
	})
}
