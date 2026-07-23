// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package runtime_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func TestBootIDSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &BootIDSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrl.BootIDController{}))
			},
		},
	})
}

type BootIDSuite struct {
	ctest.DefaultSuite
}

func (suite *BootIDSuite) TestBootID() {
	ctest.AssertResource(suite, runtime.BootIDID, func(res *runtime.BootID, asrt *assert.Assertions) {
		asrt.NotEmpty(res.TypedSpec().BootID)
	})
}
