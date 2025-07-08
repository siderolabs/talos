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
	runtimectrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type SBOMItemSuite struct {
	ctest.DefaultSuite
}

func TestSBOMItemSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &SBOMItemSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.SBOMItemController{
					SPDXPath: "./testdata/spdx",
				}))
			},
		},
	})
}

func (suite *SBOMItemSuite) TestReconcile() {
	ctest.AssertResource(suite, "apparmor-x86_64", func(item *runtime.SBOMItem, asrt *assert.Assertions) {
		asrt.Equal("apparmor-x86_64", item.TypedSpec().Name)
		asrt.Equal("v3.1.7", item.TypedSpec().Version)
		asrt.Equal("GPL-2.0-or-later", item.TypedSpec().License)
		asrt.Contains(item.TypedSpec().CPEs, "cpe:2.3:a:apparmor:apparmor:v3.1.7:*:*:*:*:*:*:*")
		asrt.Contains(item.TypedSpec().CPEs, "cpe:2.3:a:canonical:apparmor:v3.1.7:*:*:*:*:*:*:*")
		asrt.Empty(item.TypedSpec().PURLs)
	})

	ctest.AssertResource(suite, "cel.dev/expr", func(item *runtime.SBOMItem, asrt *assert.Assertions) {
		asrt.Equal("cel.dev/expr", item.TypedSpec().Name)
		asrt.Equal("v0.24.0", item.TypedSpec().Version)
		asrt.Empty(item.TypedSpec().License)
		asrt.Empty(item.TypedSpec().CPEs)
		asrt.Contains(item.TypedSpec().PURLs, "pkg:golang/cel.dev/expr@v0.24.0")
	})
}
