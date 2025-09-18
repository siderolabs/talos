// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/operator"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type OperatorSpecSuite struct {
	ctest.DefaultSuite
}

type mockOperator struct {
	spec     network.OperatorSpecSpec
	notifyCh chan<- struct{}
	panicked bool

	mu          sync.Mutex
	addresses   []network.AddressSpecSpec
	links       []network.LinkSpecSpec
	routes      []network.RouteSpecSpec
	hostname    []network.HostnameSpecSpec
	resolvers   []network.ResolverSpecSpec
	timeservers []network.TimeServerSpecSpec
}

var (
	runningOperators   = map[string]*mockOperator{}
	runningOperatorsMu sync.Mutex
)

func (mock *mockOperator) Prefix() string {
	return fmt.Sprintf("%s/%s", mock.spec.Operator, mock.spec.LinkName)
}

func (mock *mockOperator) Run(ctx context.Context, notifyCh chan<- struct{}) {
	mock.notifyCh = notifyCh

	{
		runningOperatorsMu.Lock()

		runningOperators[mock.Prefix()] = mock

		runningOperatorsMu.Unlock()
	}

	defer func() {
		runningOperatorsMu.Lock()
		delete(runningOperators, mock.Prefix())
		runningOperatorsMu.Unlock()
	}()

	if mock.spec.Operator == network.OperatorDHCP6 {
		// DHCP6 operator panics on odd run
		if !mock.panicked {
			mock.panicked = true

			panic("oh no, IPv6!!!")
		}
	}

	<-ctx.Done()
}

func (mock *mockOperator) notify() {
	mock.notifyCh <- struct{}{}
}

func (mock *mockOperator) AddressSpecs() []network.AddressSpecSpec {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	return mock.addresses
}

func (mock *mockOperator) LinkSpecs() []network.LinkSpecSpec {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	return mock.links
}

func (mock *mockOperator) RouteSpecs() []network.RouteSpecSpec {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	return mock.routes
}

func (mock *mockOperator) HostnameSpecs() []network.HostnameSpecSpec {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	return mock.hostname
}

func (mock *mockOperator) ResolverSpecs() []network.ResolverSpecSpec {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	return mock.resolvers
}

func (mock *mockOperator) TimeServerSpecs() []network.TimeServerSpecSpec {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	return mock.timeservers
}

func (suite *OperatorSpecSuite) newOperator(_ *zap.Logger, spec *network.OperatorSpecSpec) operator.Operator {
	return &mockOperator{
		spec: *spec,
	}
}

func (suite *OperatorSpecSuite) assertRunning(runningIDs []string, assertFunc func(*mockOperator) error) error {
	runningOperatorsMu.Lock()
	defer runningOperatorsMu.Unlock()

	for _, id := range runningIDs {
		op, exists := runningOperators[id]

		if !exists {
			return retry.ExpectedErrorf("operator %q is not running", id)
		}

		if err := assertFunc(op); err != nil {
			return retry.ExpectedError(err)
		}
	}

	for id := range runningOperators {
		found := false

		for _, expectedID := range runningIDs {
			if expectedID == id {
				found = true

				break
			}
		}

		if !found {
			return retry.ExpectedErrorf("operator %s should not be running", id)
		}
	}

	return nil
}

func (suite *OperatorSpecSuite) TestScheduling() {
	specDHCP := network.NewOperatorSpec(network.NamespaceName, "dhcp4/eth0")
	*specDHCP.TypedSpec() = network.OperatorSpecSpec{
		Operator:  network.OperatorDHCP4,
		LinkName:  "eth0",
		RequireUp: true,
		DHCP4: network.DHCP4OperatorSpec{
			RouteMetric: 1024,
		},
	}

	specVIP := network.NewOperatorSpec(network.NamespaceName, "vip/eth0")
	*specVIP.TypedSpec() = network.OperatorSpecSpec{
		Operator:  network.OperatorVIP,
		LinkName:  "eth0",
		RequireUp: false,
		VIP: network.VIPOperatorSpec{
			IP: netip.MustParseAddr("1.2.3.4"),
		},
	}

	suite.Create(specDHCP)
	suite.Create(specVIP)

	// operators shouldn't be running yet, as link state is not known yet
	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRunning(
					nil, func(op *mockOperator) error {
						return nil
					},
				)
			},
		),
	)

	linkState := network.NewLinkStatus(network.NamespaceName, "eth0")
	*linkState.TypedSpec() = network.LinkStatusSpec{
		OperationalState: nethelpers.OperStateDown,
	}

	suite.Create(linkState)

	// vip operator should be scheduled now, as VIP operator doesn't require link to be up
	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRunning(
					[]string{"vip/eth0"}, func(op *mockOperator) error {
						suite.Assert().Equal(netip.MustParseAddr("1.2.3.4"), op.spec.VIP.IP)

						return nil
					},
				)
			},
		),
	)

	ctest.UpdateWithConflicts(suite, linkState, func(r *network.LinkStatus) error {
		r.TypedSpec().OperationalState = nethelpers.OperStateUp

		return nil
	})

	// now all operators should be scheduled
	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRunning(
					[]string{"dhcp4/eth0", "vip/eth0"},
					func(op *mockOperator) error {
						switch op.spec.Operator { //nolint:exhaustive
						case network.OperatorDHCP4:
							suite.Assert().EqualValues(1024, op.spec.DHCP4.RouteMetric)
						case network.OperatorVIP:
							suite.Assert().Equal(netip.MustParseAddr("1.2.3.4"), op.spec.VIP.IP)
						default:
							panic("unreachable")
						}

						return nil
					},
				)
			},
		),
	)

	// change the spec, operator should be rescheduled
	ctest.UpdateWithConflicts(suite, specVIP, func(r *network.OperatorSpec) error {
		r.TypedSpec().VIP.IP = netip.MustParseAddr("3.4.5.6")

		return nil
	})

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRunning(
					[]string{"dhcp4/eth0", "vip/eth0"},
					func(op *mockOperator) error {
						switch op.spec.Operator { //nolint:exhaustive
						case network.OperatorDHCP4:
							suite.Assert().EqualValues(1024, op.spec.DHCP4.RouteMetric)
						case network.OperatorVIP:
							if op.spec.VIP.IP.Compare(netip.MustParseAddr("3.4.5.6")) != 0 {
								return retry.ExpectedErrorf("unexpected vip: %s", op.spec.VIP.IP)
							}
						default:
							panic("unreachable")
						}

						return nil
					},
				)
			},
		),
	)

	// bring down the interface, operator should be stopped
	ctest.UpdateWithConflicts(suite, linkState, func(r *network.LinkStatus) error {
		r.TypedSpec().OperationalState = nethelpers.OperStateDown

		return nil
	})

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRunning(
					[]string{"vip/eth0"}, func(op *mockOperator) error {
						return nil
					},
				)
			},
		),
	)
}

func (suite *OperatorSpecSuite) TestPanic() {
	specPanic := network.NewOperatorSpec(network.NamespaceName, "dhcp6/eth0")
	*specPanic.TypedSpec() = network.OperatorSpecSpec{
		Operator:  network.OperatorDHCP6,
		LinkName:  "eth0",
		RequireUp: true,
		DHCP6: network.DHCP6OperatorSpec{
			RouteMetric: 1024,
		},
	}

	suite.Create(specPanic)

	linkState := network.NewLinkStatus(network.NamespaceName, "eth0")
	*linkState.TypedSpec() = network.LinkStatusSpec{
		OperationalState: nethelpers.OperStateUp,
	}

	suite.Create(linkState)

	// DHCP6 operator should panic and then restart
	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRunning([]string{"dhcp6/eth0"}, func(op *mockOperator) error { return nil })
			},
		),
	)

	// bring down the interface, operator should be stopped
	ctest.UpdateWithConflicts(suite, linkState, func(r *network.LinkStatus) error {
		r.TypedSpec().OperationalState = nethelpers.OperStateDown

		return nil
	})

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRunning(
					nil, func(op *mockOperator) error {
						return nil
					},
				)
			},
		),
	)
}

func (suite *OperatorSpecSuite) TestOperatorOutputs() {
	specDHCP := network.NewOperatorSpec(network.NamespaceName, "dhcp4/eth0")
	*specDHCP.TypedSpec() = network.OperatorSpecSpec{
		Operator:  network.OperatorDHCP4,
		LinkName:  "eth0",
		RequireUp: true,
		DHCP4: network.DHCP4OperatorSpec{
			RouteMetric: 1024,
		},
	}

	suite.Create(specDHCP)

	linkState := network.NewLinkStatus(network.NamespaceName, "eth0")
	*linkState.TypedSpec() = network.LinkStatusSpec{
		OperationalState: nethelpers.OperStateUp,
	}

	suite.Create(linkState)

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRunning(
					[]string{"dhcp4/eth0"}, func(op *mockOperator) error {
						return nil
					},
				)
			},
		),
	)

	// pretend dhcp has some specs ready
	runningOperatorsMu.Lock()

	dhcpMock := runningOperators["dhcp4/eth0"]

	runningOperatorsMu.Unlock()

	dhcpMock.mu.Lock()
	dhcpMock.addresses = []network.AddressSpecSpec{
		{
			Address:     netip.MustParsePrefix("10.5.0.2/24"),
			LinkName:    "eth0",
			Family:      nethelpers.FamilyInet4,
			Scope:       nethelpers.ScopeGlobal,
			Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
			ConfigLayer: network.ConfigOperator,
		},
	}
	dhcpMock.links = []network.LinkSpecSpec{
		{
			Name:        "eth0",
			Up:          true,
			ConfigLayer: network.ConfigOperator,
		},
	}
	dhcpMock.hostname = []network.HostnameSpecSpec{
		{
			Hostname:    "foo",
			ConfigLayer: network.ConfigOperator,
		},
	}
	dhcpMock.mu.Unlock()

	dhcpMock.notify()

	ctest.AssertResources(suite,
		[]resource.ID{"dhcp4/eth0/eth0/10.5.0.2/24"},
		func(*network.AddressSpec, *assert.Assertions) {},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
	ctest.AssertResources(suite,
		[]resource.ID{"dhcp4/eth0/eth0"},
		func(*network.LinkSpec, *assert.Assertions) {},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
	ctest.AssertResources(suite,
		[]resource.ID{"dhcp4/eth0/hostname"},
		func(*network.HostnameSpec, *assert.Assertions) {},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)

	// update specs
	dhcpMock.mu.Lock()
	dhcpMock.addresses = []network.AddressSpecSpec{
		{
			Address:     netip.MustParsePrefix("10.5.0.3/24"),
			LinkName:    "eth0",
			Family:      nethelpers.FamilyInet4,
			Scope:       nethelpers.ScopeGlobal,
			Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
			ConfigLayer: network.ConfigOperator,
		},
	}
	dhcpMock.mu.Unlock()

	dhcpMock.notify()

	ctest.AssertResources(suite,
		[]resource.ID{"dhcp4/eth0/eth0/10.5.0.3/24"},
		func(*network.AddressSpec, *assert.Assertions) {},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
}

func TestOperatorSpecSuite(t *testing.T) {
	t.Parallel()

	operatorSuite := &OperatorSpecSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
		},
	}

	operatorSuite.DefaultSuite.AfterSetup = func(suite *ctest.DefaultSuite) {
		runningOperators = map[string]*mockOperator{}

		suite.Require().NoError(
			suite.Runtime().RegisterController(
				&netctrl.OperatorSpecController{
					Factory: operatorSuite.newOperator,
				},
			),
		)
	}

	suite.Run(t, operatorSuite)
}
