/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate_test

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v2"

	v1alpha1 "github.com/talos-systems/talos/pkg/config/types/v1alpha1"
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
	suite.input, err = genv1alpha1.NewInput("test", []string{"10.0.1.5", "10.0.1.6", "10.0.1.7"}, constants.DefaultKubernetesVersion)
	suite.Require().NoError(err)
}

func (suite *GenerateSuite) TestGenerateInitSuccess() {
	dataString, err := genv1alpha1.Config(genv1alpha1.TypeInit, suite.input)
	suite.Require().NoError(err)
	data := &v1alpha1.Config{}
	err = yaml.Unmarshal([]byte(dataString), data)
	suite.Require().NoError(err)
}

func (suite *GenerateSuite) TestGenerateControlPlaneSuccess() {
	dataString, err := genv1alpha1.Config(genv1alpha1.TypeControlPlane, suite.input)
	suite.Require().NoError(err)
	data := &v1alpha1.Config{}
	err = yaml.Unmarshal([]byte(dataString), data)
	suite.Require().NoError(err)
}

func (suite *GenerateSuite) TestGenerateWorkerSuccess() {
	dataString, err := genv1alpha1.Config(genv1alpha1.TypeJoin, suite.input)
	suite.Require().NoError(err)
	data := &v1alpha1.Config{}
	err = yaml.Unmarshal([]byte(dataString), data)
	suite.Require().NoError(err)
}

func (suite *GenerateSuite) TestGenerateTalosconfigSuccess() {
	_, err := genv1alpha1.Talosconfig(suite.input)
	suite.Require().NoError(err)
}
