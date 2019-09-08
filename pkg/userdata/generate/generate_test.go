/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate_test

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/talos/pkg/userdata"
	"github.com/talos-systems/talos/pkg/userdata/generate"
	"gopkg.in/yaml.v2"
)

var (
	input   *generate.Input
	inputv6 *generate.Input
)

type GenerateSuite struct {
	suite.Suite
}

func TestGenerateSuite(t *testing.T) {
	suite.Run(t, new(GenerateSuite))
}

func (suite *GenerateSuite) SetupSuite() {
	var err error
	input, err = generate.NewInput("test", []string{"10.0.1.5", "10.0.1.6", "10.0.1.7"})
	suite.Require().NoError(err)

	inputv6, err = generate.NewInput("test", []string{"2001:db8::1", "2001:db8::2", "2001:db8::3"})
	suite.Require().NoError(err)
}

// TODO: this is triggering a false positive for the dupl test, between TestGenerateControlPlaneSuccess
// nolint: dupl
func (suite *GenerateSuite) TestGenerateInitSuccess() {
	input.IP = net.ParseIP("10.0.1.5")
	dataString, err := generate.Userdata(generate.TypeInit, input)
	suite.Require().NoError(err)
	data := &userdata.UserData{}
	err = yaml.Unmarshal([]byte(dataString), data)
	suite.Require().NoError(err)

	inputv6.IP = net.ParseIP("2001:db8::1")
	dataString, err = generate.Userdata(generate.TypeInit, inputv6)
	suite.Require().NoError(err)
	data = &userdata.UserData{}
	err = yaml.Unmarshal([]byte(dataString), data)
	suite.Require().NoError(err)
}

// TODO: this is triggering a false positive for the dupl test, between TestGenerateInitSuccess
// nolint: dupl
func (suite *GenerateSuite) TestGenerateControlPlaneSuccess() {
	input.IP = net.ParseIP("10.0.1.6")
	dataString, err := generate.Userdata(generate.TypeControlPlane, input)
	suite.Require().NoError(err)
	data := &userdata.UserData{}
	err = yaml.Unmarshal([]byte(dataString), data)
	suite.Require().NoError(err)

	inputv6.IP = net.ParseIP("2001:db8::2")
	dataString, err = generate.Userdata(generate.TypeControlPlane, inputv6)
	suite.Require().NoError(err)
	data = &userdata.UserData{}
	err = yaml.Unmarshal([]byte(dataString), data)
	suite.Require().NoError(err)
}

func (suite *GenerateSuite) TestGenerateWorkerSuccess() {
	dataString, err := generate.Userdata(generate.TypeJoin, input)
	suite.Require().NoError(err)
	data := &userdata.UserData{}
	err = yaml.Unmarshal([]byte(dataString), data)
	suite.Require().NoError(err)

	dataString, err = generate.Userdata(generate.TypeJoin, inputv6)
	suite.Require().NoError(err)
	data = &userdata.UserData{}
	err = yaml.Unmarshal([]byte(dataString), data)
	suite.Require().NoError(err)
}

func (suite *GenerateSuite) TestGenerateTalosconfigSuccess() {
	_, err := generate.Talosconfig(input)
	suite.Require().NoError(err)

	_, err = generate.Talosconfig(inputv6)
	suite.Require().NoError(err)
}

func (suite *GenerateSuite) TestGetAPIServerEndpoint() {
	ep := input.GetAPIServerEndpoint("6443")
	suite.Require().Equal(input.MasterIPs[0]+":6443", ep)

	ep = input.GetAPIServerEndpoint("443")
	suite.Require().Equal(input.MasterIPs[0]+":443", ep)

	ep = inputv6.GetAPIServerEndpoint("6443")
	suite.Require().Equal(fmt.Sprintf("[%s]:6443", inputv6.MasterIPs[0]), ep)

	ep = input.GetAPIServerEndpoint("")
	suite.Require().Equal(input.MasterIPs[0], ep)

	ep = inputv6.GetAPIServerEndpoint("")
	suite.Require().Equal(fmt.Sprintf("[%s]", inputv6.MasterIPs[0]), ep)

	inputv6.IP = net.ParseIP("2001:db8::1")
	inputv6.Index = 0
	suite.Require().Equal(
		fmt.Sprintf("[%s]", inputv6.MasterIPs[0]),
		inputv6.GetAPIServerEndpoint(""),
	)

	inputv6.IP = net.ParseIP("2001:db8::2")
	inputv6.Index = 1
	suite.Require().Equal(
		fmt.Sprintf("[%s]", inputv6.MasterIPs[0]),
		inputv6.GetAPIServerEndpoint(""),
	)

	inputv6.IP = net.ParseIP("2001:db8::3")
	inputv6.Index = 2
	suite.Require().Equal(
		fmt.Sprintf("[%s]", inputv6.MasterIPs[1]),
		inputv6.GetAPIServerEndpoint(""),
	)

	inputv6.IP = net.ParseIP("2001:db8::d")
	inputv6.Index = 0
	suite.Require().Equal(
		fmt.Sprintf("[%s]", inputv6.MasterIPs[0]),
		inputv6.GetAPIServerEndpoint(""),
	)
}
