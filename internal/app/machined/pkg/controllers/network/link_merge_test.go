// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sync/errgroup"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/logging"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type LinkMergeSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *LinkMergeSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.LinkMergeController{}))

	suite.startRuntime()
}

func (suite *LinkMergeSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *LinkMergeSuite) assertLinks(requiredIDs []string, check func(*network.LinkSpec, *assert.Assertions)) {
	assertResources(suite.ctx, suite.T(), suite.state, requiredIDs, check)
}

func (suite *LinkMergeSuite) assertNoLinks(id string) error {
	resources, err := suite.state.List(
		suite.ctx,
		resource.NewMetadata(network.NamespaceName, network.AddressStatusType, "", resource.VersionUndefined),
	)
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		if res.Metadata().ID() == id {
			return retry.ExpectedErrorf("link %q is still there", id)
		}
	}

	return nil
}

func (suite *LinkMergeSuite) TestMerge() {
	loopback := network.NewLinkSpec(network.ConfigNamespaceName, "default/lo")
	*loopback.TypedSpec() = network.LinkSpecSpec{
		Name:        "lo",
		Up:          true,
		ConfigLayer: network.ConfigDefault,
	}

	dhcp := network.NewLinkSpec(network.ConfigNamespaceName, "dhcp/eth0")
	*dhcp.TypedSpec() = network.LinkSpecSpec{
		Name:        "eth0",
		Up:          true,
		MTU:         1450,
		ConfigLayer: network.ConfigOperator,
	}

	static := network.NewLinkSpec(network.ConfigNamespaceName, "configuration/eth0")
	*static.TypedSpec() = network.LinkSpecSpec{
		Name:        "eth0",
		Up:          true,
		MTU:         1500,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{loopback, dhcp, static} {
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
	}

	suite.assertLinks(
		[]string{
			"lo",
			"eth0",
		}, func(r *network.LinkSpec, asrt *assert.Assertions) {
			switch r.Metadata().ID() {
			case "lo":
				asrt.Equal(*loopback.TypedSpec(), *r.TypedSpec())
			case "eth0":
				asrt.EqualValues(1500, r.TypedSpec().MTU) // static should override dhcp
			}
		},
	)

	suite.Require().NoError(suite.state.Destroy(suite.ctx, static.Metadata()))

	suite.assertLinks(
		[]string{
			"lo",
			"eth0",
		}, func(r *network.LinkSpec, asrt *assert.Assertions) {
			switch r.Metadata().ID() {
			case "lo":
				asrt.Equal(*loopback.TypedSpec(), *r.TypedSpec())
			case "eth0":
				// reconcile happens eventually, so give it some time
				asrt.EqualValues(1450, r.TypedSpec().MTU)
			}
		},
	)

	suite.Require().NoError(suite.state.Destroy(suite.ctx, loopback.Metadata()))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertNoLinks("lo")
			},
		),
	)
}

func (suite *LinkMergeSuite) TestMergeLogicalLink() {
	bondPlatform := network.NewLinkSpec(network.ConfigNamespaceName, "platform/bond0")
	*bondPlatform.TypedSpec() = network.LinkSpecSpec{
		Name:    "bond0",
		Logical: true,
		Up:      true,
		BondMaster: network.BondMasterSpec{
			Mode: nethelpers.BondMode8023AD,
		},
		ConfigLayer: network.ConfigPlatform,
	}

	bondMachineConfig := network.NewLinkSpec(network.ConfigNamespaceName, "config/bond0")
	*bondMachineConfig.TypedSpec() = network.LinkSpecSpec{
		Name:        "bond0",
		MTU:         1450,
		Up:          true,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{bondPlatform, bondMachineConfig} {
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
	}

	suite.assertLinks(
		[]string{
			"bond0",
		}, func(r *network.LinkSpec, asrt *assert.Assertions) {
			asrt.True(r.TypedSpec().Logical)
			asrt.EqualValues(1450, r.TypedSpec().MTU)
		},
	)
}

func (suite *LinkMergeSuite) TestMergeFlapping() {
	// simulate two conflicting link definitions which are getting removed/added constantly
	dhcp := network.NewLinkSpec(network.ConfigNamespaceName, "dhcp/eth0")
	*dhcp.TypedSpec() = network.LinkSpecSpec{
		Name:        "eth0",
		Up:          true,
		MTU:         1450,
		ConfigLayer: network.ConfigOperator,
	}

	static := network.NewLinkSpec(network.ConfigNamespaceName, "configuration/eth0")
	*static.TypedSpec() = network.LinkSpecSpec{
		Name:        "eth0",
		Up:          true,
		MTU:         1500,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	resources := []resource.Resource{dhcp, static}

	flipflop := func(idx int) func() error {
		return func() error {
			for i := 0; i < 500; i++ {
				if err := suite.state.Create(suite.ctx, resources[idx]); err != nil {
					return err
				}

				if err := suite.state.Destroy(suite.ctx, resources[idx].Metadata()); err != nil {
					return err
				}

				time.Sleep(time.Millisecond)
			}

			return suite.state.Create(suite.ctx, resources[idx])
		}
	}

	var eg errgroup.Group

	eg.Go(flipflop(0))
	eg.Go(flipflop(1))
	eg.Go(
		func() error {
			// add/remove finalizer to the merged resource
			for i := 0; i < 1000; i++ {
				if err := suite.state.AddFinalizer(
					suite.ctx,
					resource.NewMetadata(
						network.NamespaceName,
						network.LinkSpecType,
						"eth0",
						resource.VersionUndefined,
					),
					"foo",
				); err != nil {
					if !state.IsNotFoundError(err) {
						return err
					}

					continue
				}

				suite.T().Log("finalizer added")

				time.Sleep(10 * time.Millisecond)

				if err := suite.state.RemoveFinalizer(
					suite.ctx,
					resource.NewMetadata(
						network.NamespaceName,
						network.LinkSpecType,
						"eth0",
						resource.VersionUndefined,
					),
					"foo",
				); err != nil {
					if err != nil && !state.IsNotFoundError(err) {
						return err
					}
				}
			}

			return nil
		},
	)

	suite.Require().NoError(eg.Wait())

	suite.assertLinks(
		[]string{
			"eth0",
		}, func(r *network.LinkSpec, asrt *assert.Assertions) {
			asrt.EqualValues(1500, r.TypedSpec().MTU)
			asrt.EqualValues(resource.PhaseRunning, r.Metadata().Phase())
		},
	)
}

func (suite *LinkMergeSuite) TestMergeWireguard() {
	static := network.NewLinkSpec(network.ConfigNamespaceName, "configuration/kubespan")
	*static.TypedSpec() = network.LinkSpecSpec{
		Name: "kubespan",
		Wireguard: network.WireguardSpec{
			ListenPort: 1234,
			Peers: []network.WireguardPeer{
				{
					PublicKey: "bGsc2rOpl6JHd/Pm4fYrIkEABL0ZxW7IlaSyh77IMhw=",
					Endpoint:  "127.0.0.1:9999",
				},
			},
		},
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	kubespanOperator := network.NewLinkSpec(network.ConfigNamespaceName, "kubespan/kubespan")
	*kubespanOperator.TypedSpec() = network.LinkSpecSpec{
		Name: "kubespan",
		Wireguard: network.WireguardSpec{
			PrivateKey: "IG9MqCII7z54Ysof1fQ9a7WcMNG+qNJRMyRCQz3JTUY=",
			ListenPort: 3456,
			Peers: []network.WireguardPeer{
				{
					PublicKey: "RXdQkMTD1Jcxd/Wizr9k8syw8ANs57l5jTormDVHAVs=",
					Endpoint:  "127.0.0.1:1234",
				},
			},
		},
		ConfigLayer: network.ConfigOperator,
	}

	for _, res := range []resource.Resource{static, kubespanOperator} {
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
	}

	suite.assertLinks(
		[]string{
			"kubespan",
		}, func(r *network.LinkSpec, asrt *assert.Assertions) {
			asrt.Equal(
				"IG9MqCII7z54Ysof1fQ9a7WcMNG+qNJRMyRCQz3JTUY=",
				r.TypedSpec().Wireguard.PrivateKey,
			)
			asrt.Equal(1234, r.TypedSpec().Wireguard.ListenPort)
			asrt.Len(r.TypedSpec().Wireguard.Peers, 2)

			asrt.Equal(
				network.WireguardPeer{
					PublicKey: "RXdQkMTD1Jcxd/Wizr9k8syw8ANs57l5jTormDVHAVs=",
					Endpoint:  "127.0.0.1:1234",
				},
				r.TypedSpec().Wireguard.Peers[0],
			)

			asrt.Equal(
				network.WireguardPeer{
					PublicKey: "bGsc2rOpl6JHd/Pm4fYrIkEABL0ZxW7IlaSyh77IMhw=",
					Endpoint:  "127.0.0.1:9999",
				},
				r.TypedSpec().Wireguard.Peers[1],
			)
		},
	)

	suite.Require().NoError(suite.state.Destroy(suite.ctx, kubespanOperator.Metadata()))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertNoLinks("kubespan")
			},
		),
	)
}

func (suite *LinkMergeSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestLinkMergeSuite(t *testing.T) {
	suite.Run(t, new(LinkMergeSuite))
}
