// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type ProbeMergeSuite struct {
	ctest.DefaultSuite
}

func (suite *ProbeMergeSuite) TestMerge() {
	p1 := network.NewProbeSpec(network.ConfigNamespaceName, "configuration/tcp:proxy.example.com:3128")
	*p1.TypedSpec() = network.ProbeSpecSpec{
		Interval:         time.Second,
		FailureThreshold: 3,
		TCP: network.TCPProbeSpec{
			Endpoint: "proxy.example.com:3128",
			Timeout:  10 * time.Second,
		},
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	p2 := network.NewProbeSpec(network.ConfigNamespaceName, "platform/tcp:proxy.example.com:3128")
	*p2.TypedSpec() = network.ProbeSpecSpec{
		Interval:         5 * time.Second,
		FailureThreshold: 5,
		TCP: network.TCPProbeSpec{
			Endpoint: "proxy.example.com:3128",
			Timeout:  5 * time.Second,
		},
		ConfigLayer: network.ConfigPlatform,
	}

	p3 := network.NewProbeSpec(network.ConfigNamespaceName, "configuration/tcp:google.com:80")
	*p3.TypedSpec() = network.ProbeSpecSpec{
		Interval:         2 * time.Second,
		FailureThreshold: 4,
		TCP: network.TCPProbeSpec{
			Endpoint: "google.com:80",
			Timeout:  3 * time.Second,
		},
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{p1, p2, p3} {
		suite.Create(res)
	}

	ctest.AssertResources(suite, []resource.ID{"tcp:proxy.example.com:3128", "tcp:google.com:80"},
		func(p *network.ProbeSpec, asrt *assert.Assertions) {
			if p.Metadata().ID() == "tcp:proxy.example.com:3128" {
				asrt.Equal(time.Second, p.TypedSpec().Interval)
				asrt.Equal(3, p.TypedSpec().FailureThreshold)
				asrt.Equal("proxy.example.com:3128", p.TypedSpec().TCP.Endpoint)
				asrt.Equal(10*time.Second, p.TypedSpec().TCP.Timeout)
			}
		},
	)

	suite.Destroy(p3)

	ctest.AssertNoResource[*network.ProbeSpec](suite, "tcp:google.com:80")
}

func TestProbeMergeSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ProbeMergeSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(netctrl.NewProbeMergeController()))
			},
		},
	})
}
