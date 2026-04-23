// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package perf_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/perf"
	perfresource "github.com/siderolabs/talos/pkg/machinery/resources/perf"
)

type PerfSuite struct {
	ctest.DefaultSuite
}

func (suite *PerfSuite) TestReconcile() {
	suite.Require().NoError(suite.Runtime().RegisterController(&perf.StatsController{}))

	ctest.AssertResource(suite, perfresource.CPUID, func(r *perfresource.CPU, asrt *assert.Assertions) {
		asrt.NotEmpty(r.TypedSpec().CPU)
	})

	ctest.AssertResource(suite, perfresource.MemoryID, func(r *perfresource.Memory, asrt *assert.Assertions) {
		asrt.NotZero(r.TypedSpec().MemTotal)
	})
}

func TestPerfSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &PerfSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
		},
	})
}
