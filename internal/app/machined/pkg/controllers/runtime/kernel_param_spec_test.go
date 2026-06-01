// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimecontrollers "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	krnl "github.com/siderolabs/talos/pkg/kernel"
	"github.com/siderolabs/talos/pkg/machinery/kernel"
	runtimeresource "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type KernelParamSpecSuite struct {
	ctest.DefaultSuite
}

func (suite *KernelParamSpecSuite) TestParamsSynced() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimecontrollers.KernelParamSpecController{}))

	const value = "500000"

	var def string

	spec := runtimeresource.NewKernelParamSpec(runtimeresource.NamespaceName, procSysfsFileMax)
	spec.TypedSpec().Value = value

	suite.Create(spec)

	ctest.AssertResource(suite, procSysfsFileMax, func(r *runtimeresource.KernelParamStatus, asrt *assert.Assertions) {
		def = r.TypedSpec().Default

		asrt.Equal(value, r.TypedSpec().Current)
	})

	prop, err := krnl.ReadParam(&kernel.Param{Key: procSysfsFileMax})
	suite.Require().NoError(err)
	suite.Require().Equal(value, strings.TrimSpace(string(prop)))

	suite.Destroy(spec)

	ctest.AssertNoResource[*runtimeresource.KernelParamStatus](suite, procSysfsFileMax)

	prop, err = krnl.ReadParam(&kernel.Param{Key: procSysfsFileMax})
	suite.Require().NoError(err)
	suite.Require().Equal(def, strings.TrimSpace(string(prop)))
}

func (suite *KernelParamSpecSuite) TestParamsUnsupported() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimecontrollers.KernelParamSpecController{}))

	const id = "proc.sys.some.really.not.existing.sysctl"

	spec := runtimeresource.NewKernelParamSpec(runtimeresource.NamespaceName, id)
	spec.TypedSpec().Value = "value"
	spec.TypedSpec().IgnoreErrors = true

	suite.Create(spec)

	ctest.AssertResource(suite, id, func(r *runtimeresource.KernelParamStatus, asrt *assert.Assertions) {
		asrt.True(r.TypedSpec().Unsupported)
	})
}

func TestKernelParamSpecSuite(t *testing.T) {
	t.Parallel()

	if os.Geteuid() != 0 {
		t.Skip("skipping test because it requires root privileges")
	}

	suite.Run(t, &KernelParamSpecSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 15 * time.Second,
		},
	})
}
