// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type HostnameSpecSuite struct {
	ctest.DefaultSuite
}

func (suite *HostnameSpecSuite) TestSpec() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.HostnameSpecController{
		V1Alpha1Mode: v1alpha1runtime.ModeContainer, // run in container mode to skip _actually_ setting hostname
	}))

	spec := network.NewHostnameSpec(network.NamespaceName, "hostname")
	*spec.TypedSpec() = network.HostnameSpecSpec{
		Hostname:    "foo",
		Domainname:  "bar",
		ConfigLayer: network.ConfigDefault,
	}

	suite.Create(spec)

	ctest.AssertResource(suite, "hostname", func(r *network.HostnameStatus, asrt *assert.Assertions) {
		asrt.Equal("foo.bar", r.TypedSpec().FQDN())
	})
}

func TestHostnameSpecSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &HostnameSpecSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
		},
	})
}
