/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package system_test

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/talos/internal/app/init/pkg/system"
)

type SystemServicesSuite struct {
	suite.Suite
}

func (suite *SystemServicesSuite) TestStartShutdown() {
	prevShutdownHackySleep := system.ShutdownHackySleep
	defer func() { system.ShutdownHackySleep = prevShutdownHackySleep }()

	system.ShutdownHackySleep = 0

	system.Services(nil).Start(&MockService{name: "containerd"}, &MockService{name: "proxyd"})
	system.Services(nil).Shutdown()
}

func TestSystemServicesSuite(t *testing.T) {
	suite.Run(t, new(SystemServicesSuite))
}
