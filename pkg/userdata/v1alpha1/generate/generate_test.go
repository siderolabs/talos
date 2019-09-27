/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v2"

	"github.com/talos-systems/talos/pkg/constants"
	v1alpha1 "github.com/talos-systems/talos/pkg/userdata/v1alpha1"
	udgenv1alpha1 "github.com/talos-systems/talos/pkg/userdata/v1alpha1/generate"
)

type GenerateSuite struct {
	suite.Suite

	input *udgenv1alpha1.Input
}

func TestGenerateSuite(t *testing.T) {
	suite.Run(t, new(GenerateSuite))
}

func (suite *GenerateSuite) SetupSuite() {
	var err error
	suite.input, err = udgenv1alpha1.NewInput("test", []string{"10.0.1.5", "10.0.1.6", "10.0.1.7"}, constants.DefaultKubernetesVersion)
	suite.Require().NoError(err)
}

func (suite *GenerateSuite) TestGenerateInitSuccess() {
	suite.input.IP = net.ParseIP("10.0.1.5")
	dataString, err := udgenv1alpha1.Userdata(udgenv1alpha1.TypeInit, suite.input)
	suite.Require().NoError(err)
	data := &v1alpha1.NodeConfig{}
	err = yaml.Unmarshal([]byte(dataString), data)
	suite.Require().NoError(err)
}

func (suite *GenerateSuite) TestGenerateControlPlaneSuccess() {
	suite.input.IP = net.ParseIP("10.0.1.6")
	suite.input.Index = 1
	dataString, err := udgenv1alpha1.Userdata(udgenv1alpha1.TypeControlPlane, suite.input)
	suite.Require().NoError(err)
	data := &v1alpha1.NodeConfig{}
	err = yaml.Unmarshal([]byte(dataString), data)
	suite.Require().NoError(err)
}

func (suite *GenerateSuite) TestGenerateWorkerSuccess() {
	dataString, err := udgenv1alpha1.Userdata(udgenv1alpha1.TypeJoin, suite.input)
	suite.Require().NoError(err)
	data := &v1alpha1.NodeConfig{}
	err = yaml.Unmarshal([]byte(dataString), data)
	suite.Require().NoError(err)
}

func (suite *GenerateSuite) TestGenerateTalosconfigSuccess() {
	_, err := udgenv1alpha1.Talosconfig(suite.input)
	suite.Require().NoError(err)
}
