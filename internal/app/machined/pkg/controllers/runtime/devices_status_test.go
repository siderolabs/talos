// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimecontrollers "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

type DevicesStatusSuite struct {
	ctest.DefaultSuite
}

func (suite *DevicesStatusSuite) TestContainerMode() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimecontrollers.DevicesStatusController{
		V1Alpha1Mode: machineruntime.ModeContainer,
	}))

	// in container mode, devices are immediately ready without any udevd service
	ctest.AssertResource(suite, runtimeres.DevicesID, func(r *runtimeres.DevicesStatus, asrt *assert.Assertions) {
		asrt.True(r.TypedSpec().Ready)
	})
}

func (suite *DevicesStatusSuite) TestWaitForUdevd() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimecontrollers.DevicesStatusController{
		V1Alpha1Mode: machineruntime.ModeMetal,
	}))

	// no udevd service yet => no status
	ctest.AssertNoResource[*runtimeres.DevicesStatus](suite, runtimeres.DevicesID)

	// udevd loaded but not yet running/healthy => still no status
	svc := v1alpha1.NewService("udevd")
	suite.Create(svc)

	ctest.AssertNoResource[*runtimeres.DevicesStatus](suite, runtimeres.DevicesID)

	// udevd running but not healthy => still no status
	ctest.UpdateWithConflicts(suite, svc, func(s *v1alpha1.Service) error {
		s.TypedSpec().Running = true
		s.TypedSpec().Healthy = false

		return nil
	})

	ctest.AssertNoResource[*runtimeres.DevicesStatus](suite, runtimeres.DevicesID)

	// udevd running & healthy => devices ready
	ctest.UpdateWithConflicts(suite, svc, func(s *v1alpha1.Service) error {
		s.TypedSpec().Running = true
		s.TypedSpec().Healthy = true

		return nil
	})

	ctest.AssertResource(suite, runtimeres.DevicesID, func(r *runtimeres.DevicesStatus, asrt *assert.Assertions) {
		asrt.True(r.TypedSpec().Ready)
	})
}

func TestDevicesStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &DevicesStatusSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 15 * time.Second,
		},
	})
}
