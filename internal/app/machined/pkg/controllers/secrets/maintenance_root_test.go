// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	secretsctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/secrets"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

func TestMaintenanceRootSuite(t *testing.T) {
	suite.Run(t, &MaintenanceRootSuite{
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&secretsctrl.MaintenanceRootController{}))
			},
		},
	})
}

type MaintenanceRootSuite struct {
	ctest.DefaultSuite
}

func (suite *MaintenanceRootSuite) TestReconcile() {
	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{secrets.MaintenanceRootID},
		func(root *secrets.MaintenanceRoot, asrt *assert.Assertions) {
			asrt.NotEmpty(root.TypedSpec().CA)
		})
}
