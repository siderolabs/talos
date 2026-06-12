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
