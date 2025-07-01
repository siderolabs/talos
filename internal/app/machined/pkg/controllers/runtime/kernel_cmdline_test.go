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
	runtimectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func TestKernelCmdlineSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &KernelCmdlineSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrl.KernelCmdlineController{}))
			},
		},
	})
}

type KernelCmdlineSuite struct {
	ctest.DefaultSuite
}

func (suite *KernelCmdlineSuite) TestKernelCmdline() {
	ctest.AssertResource(suite, runtime.KernelCmdlineID, func(res *runtime.KernelCmdline, asrt *assert.Assertions) {
		asrt.NotEmpty(res.TypedSpec().Cmdline)
	})
}
