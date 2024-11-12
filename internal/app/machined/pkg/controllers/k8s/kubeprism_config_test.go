// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"net/url"
	"testing"

	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	clusterctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type KubePrismConfigControllerSuite struct {
	ctest.DefaultSuite
}

func (suite *KubePrismConfigControllerSuite) TestGeneration() {
	cfg := &v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineFeatures: &v1alpha1.FeaturesConfig{
				KubePrismSupport: &v1alpha1.KubePrism{
					ServerEnabled: pointer.To(true),
					ServerPort:    7445,
				},
			},
		},
		ClusterConfig: &v1alpha1.ClusterConfig{
			ControlPlane: &v1alpha1.ControlPlaneConfig{
				Endpoint: &v1alpha1.Endpoint{
					URL: must(url.Parse("https://example.com"))(suite.Require()),
				},
				LocalAPIServerPort: 6445,
			},
		},
	}

	mc := config.NewMachineConfig(container.NewV1Alpha1(cfg))
	suite.Create(mc)

	endpoints := k8s.NewKubePrismEndpoints(k8s.NamespaceName, k8s.KubePrismEndpointsID)
	endpoints.TypedSpec().Endpoints = []k8s.KubePrismEndpoint{
		{Host: "example.com", Port: 443},
		{Host: "localhost", Port: 6445},
		{Host: "192.168.3.4", Port: 6446},
		{Host: "192.168.3.6", Port: 6443},
	}

	suite.Create(endpoints)

	ctest.AssertResource(suite, k8s.KubePrismConfigID, func(e *k8s.KubePrismConfig, asrt *assert.Assertions) {
		asrt.Equal(
			&k8s.KubePrismConfigSpec{
				Host: "127.0.0.1",
				Port: 7445,
				Endpoints: []k8s.KubePrismEndpoint{
					{Host: "example.com", Port: 443},
					{Host: "localhost", Port: 6445},
					{Host: "192.168.3.4", Port: 6446},
					{Host: "192.168.3.6", Port: 6443},
				},
			},
			e.TypedSpec(),
		)
	})

	ctest.UpdateWithConflicts(suite, mc, func(cfg *config.MachineConfig) error {
		balancer := cfg.Config().Machine().Features().KubePrism().(*v1alpha1.KubePrism)
		balancer.ServerEnabled = pointer.To(false)

		return nil
	})

	ctest.AssertNoResource[*k8s.KubePrismConfig](suite, k8s.KubePrismConfigID)

	ctest.UpdateWithConflicts(suite, mc, func(cfg *config.MachineConfig) error {
		balancer := cfg.Config().Machine().Features().KubePrism().(*v1alpha1.KubePrism)
		balancer.ServerEnabled = pointer.To(true)
		balancer.ServerPort = 7446

		return nil
	})

	ctest.AssertResource(suite, k8s.KubePrismConfigID, func(e *k8s.KubePrismConfig, asrt *assert.Assertions) {
		asrt.Equal(
			&k8s.KubePrismConfigSpec{
				Host: "127.0.0.1",
				Port: 7446,
				Endpoints: []k8s.KubePrismEndpoint{
					{Host: "example.com", Port: 443},
					{Host: "localhost", Port: 6445},
					{Host: "192.168.3.4", Port: 6446},
					{Host: "192.168.3.6", Port: 6443},
				},
			},
			e.TypedSpec(),
		)
	})
	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), mc.Metadata()))

	ctest.AssertNoResource[*k8s.KubePrismConfig](suite, k8s.KubePrismConfigID)

	suite.Create(mc)

	ctest.AssertResource(suite, k8s.KubePrismConfigID, func(e *k8s.KubePrismConfig, asrt *assert.Assertions) {
		asrt.Equal(
			&k8s.KubePrismConfigSpec{
				Host: "127.0.0.1",
				Port: 7445,
				Endpoints: []k8s.KubePrismEndpoint{
					{Host: "example.com", Port: 443},
					{Host: "localhost", Port: 6445},
					{Host: "192.168.3.4", Port: 6446},
					{Host: "192.168.3.6", Port: 6443},
				},
			},
			e.TypedSpec(),
		)
	})
}

func TestEndpointsBalancerConfigControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &KubePrismConfigControllerSuite{
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(clusterctrl.NewKubePrismConfigController()))
			},
		},
	})
}
