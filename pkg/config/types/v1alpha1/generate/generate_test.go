// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	genv1alpha1 "github.com/talos-systems/talos/pkg/config/types/v1alpha1/generate"
	"github.com/talos-systems/talos/pkg/constants"
)

type GenerateSuite struct {
	suite.Suite

	input *genv1alpha1.Input
}

func TestGenerateSuite(t *testing.T) {
	suite.Run(t, new(GenerateSuite))
}

func (suite *GenerateSuite) SetupSuite() {
	var err error
	suite.input, err = genv1alpha1.NewInput("test", "10.0.1.5", constants.DefaultKubernetesVersion)
	suite.Require().NoError(err)
}

func (suite *GenerateSuite) TestGenerateInitSuccess() {
	_, err := genv1alpha1.Config(runtime.MachineTypeInit, suite.input)
	suite.Require().NoError(err)
}

func (suite *GenerateSuite) TestGenerateControlPlaneSuccess() {
	_, err := genv1alpha1.Config(runtime.MachineTypeControlPlane, suite.input)
	suite.Require().NoError(err)
}

func (suite *GenerateSuite) TestGenerateWorkerSuccess() {
	_, err := genv1alpha1.Config(runtime.MachineTypeJoin, suite.input)
	suite.Require().NoError(err)
}

func (suite *GenerateSuite) TestGenerateTalosconfigSuccess() {
	_, err := genv1alpha1.Talosconfig(suite.input)
	suite.Require().NoError(err)
}
