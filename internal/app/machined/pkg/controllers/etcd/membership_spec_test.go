// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	etcdctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/etcd"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func TestMembershipSpecSuite(t *testing.T) {
	t.Parallel()

	s := &MembershipSpecSuite{}

	s.DefaultSuite = ctest.DefaultSuite{
		Timeout: time.Minute,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&etcdctrl.MembershipSpecController{}))
		},
	}

	suite.Run(t, s)
}

type MembershipSpecSuite struct {
	ctest.DefaultSuite
}

func (suite *MembershipSpecSuite) createSpec(extraArgs map[string]etcd.ArgValues) *etcd.Spec {
	spec := etcd.NewSpec(etcd.NamespaceName, etcd.SpecID)
	spec.TypedSpec().Name = "node1"
	spec.TypedSpec().AdvertisedAddresses = []netip.Addr{netip.MustParseAddr("10.0.0.1")}
	spec.TypedSpec().Image = "etcd:v1"
	spec.TypedSpec().ExtraArgs = extraArgs
	suite.Create(spec)

	return spec
}

func (suite *MembershipSpecSuite) createMachineType(machineType machine.Type) {
	machineTypeRes := config.NewMachineType()
	machineTypeRes.SetMachineType(machineType)
	suite.Create(machineTypeRes)
}

func (suite *MembershipSpecSuite) TestControlPlane() {
	spec := suite.createSpec(nil)

	// the machine type is not known yet
	ctest.AssertNoResource[*etcd.MembershipSpec](suite, etcd.MembershipSpecID)

	suite.createMachineType(machine.TypeControlPlane)

	// META is not required for the machines which never bootstrap the cluster on their own
	ctest.AssertResource(suite, etcd.MembershipSpecID, func(res *etcd.MembershipSpec, asrt *assert.Assertions) {
		asrt.Equal(etcd.MembershipSpecSpec{
			Name:                "node1",
			AdvertisedAddresses: []netip.Addr{netip.MustParseAddr("10.0.0.1")},
		}, *res.TypedSpec())
	})

	// the changes are propagated
	ctest.UpdateWithConflicts(suite, spec, func(spec *etcd.Spec) error {
		spec.TypedSpec().AdvertisedAddresses = []netip.Addr{netip.MustParseAddr("10.0.0.2")}
		spec.TypedSpec().ExtraArgs = map[string]etcd.ArgValues{
			"initial-cluster": {Values: []string{"node1=https://10.0.0.2:2380"}},
		}

		return nil
	})

	ctest.AssertResource(suite, etcd.MembershipSpecID, func(res *etcd.MembershipSpec, asrt *assert.Assertions) {
		asrt.Equal(etcd.MembershipSpecSpec{
			Name:                 "node1",
			AdvertisedAddresses:  []netip.Addr{netip.MustParseAddr("10.0.0.2")},
			ManualInitialCluster: true,
		}, *res.TypedSpec())
	})

	// etcd is not configured anymore
	suite.Destroy(spec)

	ctest.AssertNoResource[*etcd.MembershipSpec](suite, etcd.MembershipSpecID)
}

func (suite *MembershipSpecSuite) TestInit() {
	suite.createSpec(nil)
	suite.createMachineType(machine.TypeInit)

	// it is not known yet if the machine was upgraded
	ctest.AssertNoResource[*etcd.MembershipSpec](suite, etcd.MembershipSpecID)

	suite.Create(runtime.NewMetaLoaded())

	ctest.AssertResource(suite, etcd.MembershipSpecID, func(res *etcd.MembershipSpec, asrt *assert.Assertions) {
		asrt.True(res.TypedSpec().InitNode)
		asrt.True(res.TypedSpec().AutoBootstrap)
	})

	// the upgraded machine doesn't bootstrap the cluster on its own
	suite.Create(runtime.NewMetaKey(runtime.NamespaceName, runtime.MetaKeyTagToID(meta.Upgrade)))

	ctest.AssertResource(suite, etcd.MembershipSpecID, func(res *etcd.MembershipSpec, asrt *assert.Assertions) {
		asrt.True(res.TypedSpec().InitNode)
		asrt.False(res.TypedSpec().AutoBootstrap)
	})
}
