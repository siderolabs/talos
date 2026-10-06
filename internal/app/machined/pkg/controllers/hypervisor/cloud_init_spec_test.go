// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

type CloudInitProjectionSuite struct {
	ctest.DefaultSuite
}

func TestCloudInitProjectionSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, &CloudInitProjectionSuite{
		ctest.DefaultSuite{
			Timeout: 30 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.CloudInitSpecController{}))
			},
		},
	})
}

func (s *CloudInitProjectionSuite) TestExternalVMUpdatesAndRemovesSeed() {
	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "external")
	vm.TypedSpec().CloudInit = &hypervisor.VirtualMachineCloudInitSpec{
		Library:  "images",
		MetaData: "instance-id: external\n",
		UserData: "secret",
	}

	s.Create(vm)

	ctest.AssertResource(s, "external", func(res *hypervisor.CloudInitSpec, a *assert.Assertions) {
		a.Equal("images", res.TypedSpec().Library)
		a.Equal("secret", res.TypedSpec().UserData)
		a.Equal("hypervisor.CloudInitSpecController", res.Metadata().Owner())
	})

	s.Destroy(vm)
	ctest.AssertNoResource[*hypervisor.CloudInitSpec](s, "external")
}
