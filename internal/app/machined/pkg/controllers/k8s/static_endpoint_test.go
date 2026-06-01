// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"net/netip"
	"net/url"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type StaticEndpointControllerSuite struct {
	ctest.DefaultSuite
}

func (suite *StaticEndpointControllerSuite) TestReconcile() {
	u, err := url.Parse("https://[2001:db8::1]:6443/")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
				},
			},
		),
	)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.ControlPlaneKubernetesEndpointsID},
		func(endpoint *k8s.Endpoint, assert *assert.Assertions) {
			assert.Equal([]netip.Addr{netip.MustParseAddr("2001:db8::1")}, endpoint.TypedSpec().Addresses)
			assert.Empty(endpoint.TypedSpec().Hosts)
		})

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), cfg.Metadata()))

	rtestutils.AssertNoResource[*k8s.Endpoint](suite.Ctx(), suite.T(), suite.State(), k8s.ControlPlaneKubernetesEndpointsID)
}

func (suite *StaticEndpointControllerSuite) TestReconcileHostname() {
	u, err := url.Parse("https://localhost:6443/")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
				},
			},
		),
	)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.ControlPlaneKubernetesEndpointsID},
		func(endpoint *k8s.Endpoint, assert *assert.Assertions) {
			// localhost might resolve to ::1 as well, check only for 127.0.0.1
			assert.Contains(endpoint.TypedSpec().Addresses, netip.MustParseAddr("127.0.0.1"))
			assert.Equal([]string{"localhost"}, endpoint.TypedSpec().Hosts)
		})
}

func TestStaticEndpointControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &StaticEndpointControllerSuite{
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&k8sctrl.StaticEndpointController{}))
			},
		},
	})
}
