/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package system_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
)

type SystemServicesSuite struct {
	suite.Suite
}

func (suite *SystemServicesSuite) TestStartShutdown() {
	system.Services(nil).Start(
		&MockService{name: "containerd"},
		&MockService{name: "proxyd", dependencies: []string{"containerd"}},
		&MockService{name: "trustd", dependencies: []string{"containerd", "proxyd"}},
		&MockService{name: "osd", dependencies: []string{"containerd"}},
	)
	time.Sleep(10 * time.Millisecond)
	system.Services(nil).Shutdown()
}

func TestSystemServicesSuite(t *testing.T) {
	suite.Run(t, new(SystemServicesSuite))
}

func (suite *SystemServicesSuite) TestStartStop() {
	system.Services(nil).Start(
		&MockService{name: "yolo"},
	)
	time.Sleep(10 * time.Millisecond)
	err := system.Services(nil).Stop(
		context.TODO(), "yolo",
	)
	suite.Assert().NoError(err)
}
