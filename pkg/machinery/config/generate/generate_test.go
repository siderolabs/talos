// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate_test

import (
	"crypto/x509"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
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

	_, err = cfg.Validate(runtimeMode{false})
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
