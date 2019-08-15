/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package manifest

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/talos/pkg/userdata"
	"gopkg.in/yaml.v2"
)

type validateSuite struct {
	suite.Suite
}

func TestValidateSuite(t *testing.T) {
	suite.Run(t, new(validateSuite))
}

func (suite *validateSuite) TestVerifyDevice() {
	// Start off with success and then remove bits
	data := &userdata.UserData{}
	err := yaml.Unmarshal([]byte(testConfig), data)
	suite.Require().NoError(err)

	suite.Require().NoError(VerifyBootDevice(data))
	suite.Require().NoError(VerifyDataDevice(data))

	// No impact because we can infer all data from the data device and
	// defaults.
	data.Install.Boot = nil
	suite.Require().NoError(VerifyBootDevice(data))
	data.Install.Ephemeral = &userdata.InstallDevice{
		Device: "/dev/sda",
	}
	suite.Require().NoError(VerifyDataDevice(data))
}
