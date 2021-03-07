// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/pkg/machinery/config"
	genv1alpha1 "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/generate"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

type GenerateSuite struct {
	suite.Suite

	input      *genv1alpha1.Input
	genOptions []genv1alpha1.GenOption
}

func TestGenerateSuite(t *testing.T) {
	for _, tt := range []struct {
		label      string
		genOptions []genv1alpha1.GenOption
	}{
		{
			label: "current",
		},
		{
			label:      "0.9",
			genOptions: []genv1alpha1.GenOption{genv1alpha1.WithVersionContract(config.TalosVersion0_9)},
		},
		{
			label:      "0.8",
			genOptions: []genv1alpha1.GenOption{genv1alpha1.WithVersionContract(config.TalosVersion0_8)},
		},
	} {
		tt := tt

		t.Run(tt.label, func(t *testing.T) {
			suite.Run(t, &GenerateSuite{
				genOptions: tt.genOptions,
			})
		})
	}
}

func (suite *GenerateSuite) SetupSuite() {
	var err error
	secrets, err := genv1alpha1.NewSecretsBundle(genv1alpha1.NewClock(), suite.genOptions...)
	suite.Require().NoError(err)
	suite.input, err = genv1alpha1.NewInput("test", "10.0.1.5", constants.DefaultKubernetesVersion, secrets, suite.genOptions...)
	suite.Require().NoError(err)
}

func (suite *GenerateSuite) TestGenerateInitSuccess() {
	_, err := genv1alpha1.Config(machine.TypeInit, suite.input)
	suite.Require().NoError(err)
}

func (suite *GenerateSuite) TestGenerateControlPlaneSuccess() {
	_, err := genv1alpha1.Config(machine.TypeControlPlane, suite.input)
	suite.Require().NoError(err)
}

func (suite *GenerateSuite) TestGenerateWorkerSuccess() {
	_, err := genv1alpha1.Config(machine.TypeJoin, suite.input)
	suite.Require().NoError(err)
}

func (suite *GenerateSuite) TestGenerateTalosconfigSuccess() {
	_, err := genv1alpha1.Talosconfig(suite.input)
	suite.Require().NoError(err)
}
