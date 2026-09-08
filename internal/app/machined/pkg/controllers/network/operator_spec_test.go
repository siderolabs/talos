// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"context"
	"fmt"
	"io"
	"net/netip"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/lldp"
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
	lldp        []network.LLDPNeighborSpec
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

func (mock *mockOperator) LLDPNeighborSpecs() []network.LLDPNeighborSpec {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	return mock.lldp
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
		found := slices.Contains(runningIDs, id)

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

	ctest.AssertResources(
		suite,
		[]resource.ID{"dhcp4/eth0/eth0/10.5.0.2/24"},
		func(*network.AddressSpec, *assert.Assertions) {},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
	ctest.AssertResources(
		suite,
		[]resource.ID{"dhcp4/eth0/eth0"},
		func(*network.LinkSpec, *assert.Assertions) {},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
	ctest.AssertResources(
		suite,
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

	ctest.AssertResources(
		suite,
		[]resource.ID{"dhcp4/eth0/eth0/10.5.0.3/24"},
		func(*network.AddressSpec, *assert.Assertions) {},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
}

// lldpStatusListener is a socket substitute; the registered controller still runs
// the real LLDP operator and receives its normal output notifications.
type lldpStatusListener struct {
	reads    chan []byte
	failed   chan struct{}
	closed   chan struct{}
	once     sync.Once
	mu       sync.Mutex
	deadline time.Time
}

func (listener *lldpStatusListener) SetReadDeadline(deadline time.Time) error {
	listener.mu.Lock()
	defer listener.mu.Unlock()

	listener.deadline = deadline

	return nil
}

func (listener *lldpStatusListener) ReadFrame() ([]byte, error) {
	listener.mu.Lock()
	deadline := listener.deadline
	listener.mu.Unlock()

	var expired <-chan time.Time

	if !deadline.IsZero() {
		timer := time.NewTimer(time.Until(deadline))
		defer timer.Stop()

		expired = timer.C
	}

	select {
	case frame := <-listener.reads:
		return frame, nil
	case <-listener.failed:
		return nil, io.ErrUnexpectedEOF
	case <-listener.closed:
		return nil, io.EOF
	case <-expired:
		return nil, os.ErrDeadlineExceeded
	}
}

func (listener *lldpStatusListener) Close() error {
	listener.once.Do(func() { close(listener.closed) })

	return nil
}

type LLDPOperatorSpecSuite struct {
	ctest.DefaultSuite
}

func (suite *LLDPOperatorSpecSuite) TestSocketFailureAndLinkRecovery() {
	opened := make(chan *lldpStatusListener, 4)

	var (
		attempts atomic.Int32
		index    atomic.Uint32
	)

	require.NoError(suite.T(), suite.Runtime().RegisterController(&netctrl.OperatorSpecController{
		Factory: func(logger *zap.Logger, spec *network.OperatorSpecSpec) operator.Operator {
			return operator.NewLLDP(logger, spec.LinkName, spec.LLDP.LinkIndex, func(linkIndex uint32) (lldp.Listener, error) {
				attempts.Add(1)
				index.Store(linkIndex)

				listener := &lldpStatusListener{
					reads:  make(chan []byte, 1),
					failed: make(chan struct{}),
					closed: make(chan struct{}),
				}
				opened <- listener

				return listener, nil
			})
		},
	}))

	spec := network.NewOperatorSpec(network.NamespaceName, "lldp/eth0")
	*spec.TypedSpec() = network.OperatorSpecSpec{
		Operator:  network.OperatorLLDP,
		LinkName:  "eth0",
		RequireUp: true,
		LLDP:      network.LLDPOperatorSpec{LinkIndex: 7},
	}
	link := network.NewLinkStatus(network.NamespaceName, "eth0")
	link.TypedSpec().OperationalState = nethelpers.OperStateUp

	suite.Create(spec)
	suite.Create(link)
	synctest.Wait()

	require.EqualValues(suite.T(), 1, attempts.Load())
	require.EqualValues(suite.T(), 7, index.Load())
	require.Len(suite.T(), opened, 1)
	first := <-opened

	// Ethernet header, chassis MAC, interface-name port eth0, TTL 120, end.
	frame := []byte{
		1, 0x80, 0xc2, 0, 0, 0x0e, 2, 0, 0, 0, 0, 1, 0x88, 0xcc,
		2, 7, 4, 2, 0, 0, 0, 0, 1,
		4, 5, 5, 'e', 't', 'h', '0',
		6, 2, 0, 120, 0, 0,
	}
	first.reads <- frame

	synctest.Wait()

	status := network.NewLLDPNeighborStatus(network.NamespaceName, "eth0")
	published, err := ctest.GetUsingResource(suite, status)
	require.NoError(suite.T(), err)
	require.Len(suite.T(), published.TypedSpec().Neighbors, 1)

	// No resource write follows this failure: only the operator's cleanup
	// notification can make the registered controller remove the status.
	close(first.failed)
	synctest.Wait()

	select {
	case <-first.closed:
	default:
		suite.T().Fatal("failed listener was not closed")
	}

	_, err = ctest.GetUsingResource(suite, status)
	require.True(suite.T(), state.IsNotFoundError(err), "socket failure must remove status without a resource event; got %v", err)

	// Cross the former retry interval twice, without changing the spec.
	<-time.NewTimer(11 * time.Second).C
	synctest.Wait()
	require.EqualValues(suite.T(), 1, attempts.Load(), "unchanged spec must not reopen a failed socket")
	require.Empty(suite.T(), opened)

	ctest.UpdateWithConflicts(suite, link, func(link *network.LinkStatus) error {
		link.TypedSpec().OperationalState = nethelpers.OperStateDown

		return nil
	})
	synctest.Wait()
	require.EqualValues(suite.T(), 1, attempts.Load())

	ctest.UpdateWithConflicts(suite, link, func(link *network.LinkStatus) error {
		link.TypedSpec().OperationalState = nethelpers.OperStateUp

		return nil
	})
	synctest.Wait()
	require.EqualValues(suite.T(), 2, attempts.Load())
	require.Len(suite.T(), opened, 1)
	second := <-opened
	require.NotSame(suite.T(), first, second)

	second.reads <- frame

	synctest.Wait()

	republished, err := ctest.GetUsingResource(suite, status)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), published.TypedSpec(), republished.TypedSpec())
}

func TestLLDPOperatorSpecSuite(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// synctest forbids T.Run, so invoke the suite lifecycle directly rather
		// than using suite.Run. The runtime must be created inside the bubble.
		operatorSuite := &LLDPOperatorSpecSuite{
			DefaultSuite: ctest.DefaultSuite{Timeout: time.Minute},
		}
		operatorSuite.SetT(t)

		operatorSuite.SetupTest()
		defer operatorSuite.TearDownTest()

		operatorSuite.TestSocketFailureAndLinkRecovery()
	})
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
