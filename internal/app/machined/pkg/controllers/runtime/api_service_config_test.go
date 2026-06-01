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
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

type APIServiceConfigControllerSuite struct {
	ctest.DefaultSuite
}

func TestAPIServiceConfigControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &APIServiceConfigControllerSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&runtime.APIServiceConfigController{}))
			},
		},
	})
}

func (suite *APIServiceConfigControllerSuite) TestMaintenanceMode() {
	request := runtimeres.NewMaintenanceServiceRequest()
	suite.Create(request)

	ctest.AssertResource(
		suite, runtimeres.MaintenanceServiceRequestID,
		func(req *runtimeres.MaintenanceServiceRequest, asrt *assert.Assertions) {
			asrt.False(req.Metadata().Finalizers().Empty())
		},
	)

	cfg := runtimeres.NewMaintenanceServiceConfig()
	cfg.TypedSpec().ListenAddress = ":1"
	suite.Create(cfg)

	ctest.AssertResource(
		suite, runtimeres.APIServiceConfigID,
		func(cfg *runtimeres.APIServiceConfig, asrt *assert.Assertions) {
			asrt.Equal(":1", cfg.TypedSpec().ListenAddress)
			asrt.True(cfg.TypedSpec().NodeRoutingDisabled)
			asrt.True(cfg.TypedSpec().ReadonlyRoleMode)
			asrt.True(cfg.TypedSpec().SkipVerifyingClientCert)
		},
	)

	_, err := suite.State().Teardown(suite.Ctx(), request.Metadata())
	suite.Require().NoError(err)

	ctest.AssertNoResource[*runtimeres.APIServiceConfig](suite, runtimeres.APIServiceConfigID)

	ctest.AssertResource(
		suite, runtimeres.MaintenanceServiceRequestID,
		func(req *runtimeres.MaintenanceServiceRequest, asrt *assert.Assertions) {
			asrt.True(req.Metadata().Finalizers().Empty())
		},
	)

	suite.Destroy(request)
}

func (suite *APIServiceConfigControllerSuite) TestRegularMode() {
	cert := secrets.NewAPI()
	suite.Create(cert)

	ctest.AssertResource(
		suite, runtimeres.APIServiceConfigID,
		func(cfg *runtimeres.APIServiceConfig, asrt *assert.Assertions) {
			asrt.Equal(":50000", cfg.TypedSpec().ListenAddress)
			asrt.False(cfg.TypedSpec().NodeRoutingDisabled)
			asrt.False(cfg.TypedSpec().ReadonlyRoleMode)
			asrt.False(cfg.TypedSpec().SkipVerifyingClientCert)
		},
	)
}
