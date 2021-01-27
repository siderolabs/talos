// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

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
	system.Services(nil).LoadAndStart(
		&MockService{name: "containerd"},
		&MockService{name: "trustd", dependencies: []string{"containerd"}},
		&MockService{name: "machined", dependencies: []string{"containerd", "trustd"}},
	)
	time.Sleep(10 * time.Millisecond)

	suite.Require().NoError(system.Services(nil).Unload(context.Background(), "trustd", "notrunning"))

	system.Services(nil).Shutdown(context.TODO())
}

func TestSystemServicesSuite(t *testing.T) {
	suite.Run(t, new(SystemServicesSuite))
}

func (suite *SystemServicesSuite) TestStartStop() {
	system.Services(nil).LoadAndStart(
		&MockService{name: "yolo"},
	)

	time.Sleep(10 * time.Millisecond)

	err := system.Services(nil).Stop(
		context.TODO(), "yolo",
	)
	suite.Assert().NoError(err)
}
