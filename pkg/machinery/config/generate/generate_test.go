// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate_test

import (
	"crypto/x509"
	"fmt"
	"testing"

	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config"
	mc "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/config/types/cri"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

type GenerateSuite struct {
	suite.Suite

	input      *generate.Input
	genOptions []generate.Option

	versionContract *config.VersionContract
}

func TestGenerateSuite(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		label      string
		genOptions []generate.Option
	}{
		{
			label: "current",
		},
		{
			label:      "1.7",
			genOptions: []generate.Option{generate.WithVersionContract(config.TalosVersion1_7)},
		},
		{
			label:      "1.6",
			genOptions: []generate.Option{generate.WithVersionContract(config.TalosVersion1_6)},
		},
		{
			label:      "1.5",
			genOptions: []generate.Option{generate.WithVersionContract(config.TalosVersion1_5)},
		},
		{
			label:      "1.4",
			genOptions: []generate.Option{generate.WithVersionContract(config.TalosVersion1_4)},
		},
		{
			label:      "1.3",
			genOptions: []generate.Option{generate.WithVersionContract(config.TalosVersion1_3)},
		},
		{
			label:      "1.2",
			genOptions: []generate.Option{generate.WithVersionContract(config.TalosVersion1_2)},
		},
		{
			label:      "1.1",
			genOptions: []generate.Option{generate.WithVersionContract(config.TalosVersion1_1)},
		},
		{
			label:      "1.0",
			genOptions: []generate.Option{generate.WithVersionContract(config.TalosVersion1_0)},
		},
	} {
		t.Run(tt.label, func(t *testing.T) {
			t.Parallel()

			suite.Run(t, &GenerateSuite{
				genOptions: tt.genOptions,
			})
		})
	}
}

func (suite *GenerateSuite) SetupSuite() {
	var err error

	suite.input, err = generate.NewInput("test", "https://10.0.1.5", constants.DefaultKubernetesVersion, suite.genOptions...)
	suite.Require().NoError(err)

	var opts generate.Options

	for _, opt := range suite.genOptions {
		suite.Require().NoError(opt(&opts))
	}

	suite.versionContract = suite.input.Options.VersionContract
}

func (suite *GenerateSuite) TestGenerateInitSuccess() {
	cfg, err := suite.input.Config(machine.TypeInit)
	suite.Require().NoError(err)

	suite.NotEmpty(cfg.Machine().Security().IssuingCA())
}

func (suite *GenerateSuite) TestGenerateControlPlaneSuccess() {
	cfg, err := suite.input.Config(machine.TypeControlPlane)
	suite.Require().NoError(err)

	_, err = cfg.ValidateAsClient(runtimeMode{false})
	suite.Require().NoError(err)

	suite.NotEmpty(cfg.Machine().Security().IssuingCA())
}

func (suite *GenerateSuite) TestGenerateWorkerSuccess() {
	cfg, err := suite.input.Config(machine.TypeWorker)
	suite.Require().NoError(err)

	suite.NotEmpty(cfg.Machine().Security().IssuingCA())
}

func (suite *GenerateSuite) TestGenerateTalosconfigSuccess() {
	cfg, err := suite.input.Talosconfig()
	suite.Require().NoError(err)

	creds, err := client.CertificateFromConfigContext(cfg.Contexts[cfg.Context])
	suite.Require().NoError(err)
	suite.Require().Len(creds.Certificate, 1)

	cert, err := x509.ParseCertificate(creds.Certificate[0])
	suite.Require().NoError(err)

	suite.Equal([]string{string(role.Admin)}, cert.Subject.Organization)
}

func TestGenerateRegistryMirrorsOrder(t *testing.T) {
	t.Parallel()

	input, err := generate.NewInput(
		"test", "https://10.0.1.5", constants.DefaultKubernetesVersion,
		generate.WithRegistryMirror("b.com", "http://127.0.0.1:5004"),
		generate.WithRegistryMirror("a.com", "http://127.0.0.1:5005"),
	)

	require.NoError(t, err)

	cfg, err := input.Config(machine.TypeControlPlane)
	require.NoError(t, err)

	registryConfigs := xslices.Filter(cfg.Documents(), func(doc mc.Document) bool {
		return doc.Kind() == cri.RegistryMirrorConfig
	})
	require.Len(t, registryConfigs, 2)

	named, ok := registryConfigs[0].(mc.NamedDocument)
	require.True(t, ok)
	assert.Equal(t, "a.com", named.Name())

	named, ok = registryConfigs[1].(mc.NamedDocument)
	require.True(t, ok)
	assert.Equal(t, "b.com", named.Name())
}

func TestGenerateEphemeralVolumeConfig(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name            string
		versionContract *config.VersionContract
		expectConfig    bool
	}{
		{
			name:         "current",
			expectConfig: true,
		},
		{
			name:            "1.14",
			versionContract: config.TalosVersion1_14,
			expectConfig:    true,
		},
		{
			name:            "1.13",
			versionContract: config.TalosVersion1_13,
		},
	} {
		for _, machineType := range []machine.Type{machine.TypeInit, machine.TypeControlPlane, machine.TypeWorker} {
			t.Run(fmt.Sprintf("%s/%s", test.name, machineType), func(t *testing.T) {
				t.Parallel()

				input, err := generate.NewInput(
					"test",
					"https://10.0.1.5:6443",
					constants.DefaultKubernetesVersion,
					generate.WithVersionContract(test.versionContract),
				)
				require.NoError(t, err)

				cfg, err := input.Config(machineType)
				require.NoError(t, err)

				volumeConfig, ok := cfg.Volumes().ByName(constants.EphemeralPartitionLabel)
				require.Equal(t, test.expectConfig, ok)

				if !ok {
					assert.False(t, volumeConfig.Mount().Secure())

					return
				}

				ephemeralConfig, ok := volumeConfig.(*blockcfg.VolumeConfigV1Alpha1)
				require.True(t, ok)
				require.NotNil(t, ephemeralConfig.MountSpec.MountSecure)
				assert.True(t, *ephemeralConfig.MountSpec.MountSecure)
			})
		}
	}
}

// TestGenerateDiscoveryServiceConfig verifies that discovery config generation is gated on the version contract:
// 1.14+ emits a multi-doc DiscoveryServiceConfig, older versions emit the legacy .cluster.discovery block,
// and disabling discovery emits neither.
func TestGenerateDiscoveryServiceConfig(t *testing.T) {
	t.Parallel()

	for _, machineType := range []machine.Type{machine.TypeControlPlane, machine.TypeWorker} {
		for _, test := range []struct {
			name       string
			genOptions []generate.Option

			// expectMultidoc: a DiscoveryServiceConfig document named "default" is emitted
			expectMultidoc bool
			// expectLegacy: the deprecated .cluster.discovery block is populated
			expectLegacy bool
			// expectEndpoint: endpoint surfaced via the DiscoveryServiceConfigs() accessor (empty means none)
			expectEndpoint string
		}{
			{
				name:           "1.14 discovery enabled by default",
				genOptions:     []generate.Option{generate.WithVersionContract(config.TalosVersion1_14)},
				expectMultidoc: true,
				expectEndpoint: constants.DefaultDiscoveryServiceEndpoint,
			},
			{
				name:           "1.13 discovery enabled uses legacy block",
				genOptions:     []generate.Option{generate.WithVersionContract(config.TalosVersion1_13)},
				expectLegacy:   true,
				expectEndpoint: constants.DefaultDiscoveryServiceEndpoint,
			},
			{
				name:           "1.14 discovery disabled emits nothing",
				genOptions:     []generate.Option{generate.WithVersionContract(config.TalosVersion1_14), generate.WithClusterDiscovery(false)},
				expectLegacy:   false,
				expectEndpoint: "",
			},
			{
				name:         "1.13 discovery disabled uses legacy block",
				genOptions:   []generate.Option{generate.WithVersionContract(config.TalosVersion1_13), generate.WithClusterDiscovery(false)},
				expectLegacy: true,
			},
		} {
			t.Run(fmt.Sprintf("%s/%s", machineType, test.name), func(t *testing.T) {
				t.Parallel()

				input, err := generate.NewInput("test", "https://10.0.1.5:6443", constants.DefaultKubernetesVersion, test.genOptions...)
				require.NoError(t, err)

				cfg, err := input.Config(machineType)
				require.NoError(t, err)

				multidocCount := len(xslices.Filter(cfg.Documents(), func(doc mc.Document) bool {
					return doc.Kind() == "DiscoveryServiceConfig"
				}))

				if test.expectMultidoc {
					assert.Equal(t, 1, multidocCount)
				} else {
					assert.Equal(t, 0, multidocCount)
				}

				assert.Equal(t, test.expectLegacy, cfg.RawV1Alpha1().ClusterConfig.ClusterDiscoveryConfig != nil) //nolint:staticcheck // verifying legacy config presence

				discoveryConfigs := cfg.DiscoveryServiceConfigs()

				if test.expectEndpoint == "" {
					assert.Empty(t, discoveryConfigs)

					return
				}

				require.Len(t, discoveryConfigs, 1)
				assert.Equal(t, test.expectEndpoint, discoveryConfigs[0].Endpoint().String())
			})
		}
	}
}

// TestGenerateDiscoveryIdentityConfig verifies that cluster identity generation is gated on the version contract:
// 1.14+ emits a multi-doc DiscoveryIdentityConfig and leaves the legacy .cluster.id/.cluster.secret empty,
// while older versions populate the legacy fields and emit no dedicated document. In both cases the identity is
// surfaced via the DiscoveryIdentityConfig() accessor.
func TestGenerateDiscoveryIdentityConfig(t *testing.T) {
	t.Parallel()

	for _, machineType := range []machine.Type{machine.TypeControlPlane, machine.TypeWorker} {
		for _, test := range []struct {
			name       string
			genOptions []generate.Option

			// expectMultidoc: a DiscoveryIdentityConfig document is emitted
			expectMultidoc bool
		}{
			{
				name:           "1.14 uses multi-doc identity",
				genOptions:     []generate.Option{generate.WithVersionContract(config.TalosVersion1_14)},
				expectMultidoc: true,
			},
			{
				name:           "1.13 uses legacy identity",
				genOptions:     []generate.Option{generate.WithVersionContract(config.TalosVersion1_13)},
				expectMultidoc: false,
			},
		} {
			t.Run(fmt.Sprintf("%s/%s", machineType, test.name), func(t *testing.T) {
				t.Parallel()

				input, err := generate.NewInput("test", "https://10.0.1.5:6443", constants.DefaultKubernetesVersion, test.genOptions...)
				require.NoError(t, err)

				cfg, err := input.Config(machineType)
				require.NoError(t, err)

				multidocCount := len(xslices.Filter(cfg.Documents(), func(doc mc.Document) bool {
					return doc.Kind() == "DiscoveryIdentityConfig"
				}))

				legacyID := cfg.RawV1Alpha1().ClusterConfig.ClusterID         //nolint:staticcheck // verifying legacy config presence
				legacySecret := cfg.RawV1Alpha1().ClusterConfig.ClusterSecret //nolint:staticcheck // verifying legacy config presence

				if test.expectMultidoc {
					assert.Equal(t, 1, multidocCount)
					assert.Empty(t, legacyID)
					assert.Empty(t, legacySecret)
				} else {
					assert.Equal(t, 0, multidocCount)
					assert.NotEmpty(t, legacyID)
					assert.NotEmpty(t, legacySecret)
				}

				// regardless of the form, the identity must be surfaced through the aggregated accessor
				identity := cfg.DiscoveryIdentityConfig()
				require.NotNil(t, identity)
				assert.NotEmpty(t, identity.ClusterID())
				assert.NotEmpty(t, identity.ClusterSecret())
			})
		}
	}
}

type runtimeMode struct {
	requiresInstall bool
}

func (m runtimeMode) String() string {
	return fmt.Sprintf("runtimeMode(%v)", m.requiresInstall)
}

func (m runtimeMode) RequiresInstall() bool {
	return m.requiresInstall
}

func (runtimeMode) InContainer() bool {
	return false
}
