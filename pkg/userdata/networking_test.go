/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"github.com/hashicorp/go-multierror"
	"golang.org/x/xerrors"
)

func (suite *validateSuite) TestValidateDevice() {
	var err error

	// Test for missing required sections
	dev := &Device{}
	err = dev.Validate(CheckDeviceInterface())
	suite.Require().Error(err)
	// Embedding the check in suite.Assert().Equal(true, xerrors.Is had issues )
	if !xerrors.Is(err.(*multierror.Error).Errors[0], ErrRequiredSection) {
		suite.T().Errorf("%+v", err)

	}

	dev.Interface = "eth0"
	err = dev.Validate(CheckDeviceInterface())
	suite.Require().NoError(err)

	err = dev.Validate(CheckDeviceAddressing())
	suite.Require().Error(err)

	// Ensure only a single addressing scheme is specified
	dev.DHCP = true
	dev.CIDR = "1.0.0.1/32"
	err = dev.Validate(CheckDeviceAddressing())
	suite.Require().Error(err)

	dev.DHCP = false
	err = dev.Validate(CheckDeviceAddressing())
	suite.Require().NoError(err)

	dev.Routes = []Route{}
	err = dev.Validate(CheckDeviceRoutes())
	suite.Require().NoError(err)

	// nolint: gofmt
	dev.Routes = []Route{Route{Gateway: "yolo"}}
	err = dev.Validate(CheckDeviceRoutes())
	suite.Require().Error(err)

	// nolint: gofmt
	dev.Routes = []Route{Route{Gateway: "yolo", Network: "totes"}}
	err = dev.Validate(CheckDeviceRoutes())
	suite.Require().Error(err)

	// nolint: gofmt
	dev.Routes = []Route{Route{Gateway: "192.168.1.1", Network: "192.168.1.0/24"}}
	err = dev.Validate(CheckDeviceRoutes())
	suite.Require().NoError(err)
}
