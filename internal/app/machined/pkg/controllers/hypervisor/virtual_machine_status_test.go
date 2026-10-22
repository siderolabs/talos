// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"context"
	"errors"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

type openFailure struct{ err error }

type VirtualMachineStatusSuite struct {
	ctest.DefaultSuite
	client       *domainClient
	openErr      atomic.Pointer[openFailure]
	opens        atomic.Int32
	boundedOpens atomic.Int32
	events       chan struct{}
}

func (s *VirtualMachineStatusSuite) SetupTest() {
	s.DefaultSuite.SetupTest()
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineProfileSpecController{}))
	s.openErr.Store(nil)
	s.opens.Store(0)
	s.boundedOpens.Store(0)
	s.events = make(chan struct{}, 1)
	s.client = &domainClient{
		domains:         make(map[string]libvirtdomain.Domain),
		texts:           make(map[string]string),
		starts:          make(map[string]int),
		shutdowns:       make(map[string]int),
		reboots:         make(map[string]int),
		changed:         make(chan struct{}, 1),
		attempted:       make(chan struct{}, 1),
		attemptedRemove: make(chan struct{}, 1),

		attemptedShutdown: make(chan struct{}, 1),
		listed:            make(chan struct{}, 1),
	}

	machine := hardware.NewSystemInformation(hardware.SystemInformationID)
	machine.TypedSpec().UUID = machineUUID
	s.Create(machine)
	s.Create(newReadyVirtqemudService())
}

func TestVirtualMachineStatusSuite(t *testing.T) {
	t.Parallel()

	s := &VirtualMachineStatusSuite{}
	s.Timeout = 30 * time.Second

	suite.Run(t, s)
}

func (s *VirtualMachineStatusSuite) open(ctx context.Context) (libvirtdomain.Client, error) {
	s.opens.Add(1)

	if failure := s.openErr.Load(); failure != nil {
		return nil, failure.err
	}

	return s.client.open(ctx)
}

// openBounded wires OpenBounded to a wrapper whose Close is a no-op, so bounded-session
// accounting does not leak into assertions on the long-lived observation session's counters.
func (s *VirtualMachineStatusSuite) openBounded(_ context.Context) (libvirtdomain.Client, error) {
	s.boundedOpens.Add(1)

	return &boundedDomainClient{inner: s.client}, nil
}

// boundedDomainClient is the test fake used for guest-agent sub-sessions. Only the methods
// observeDomain actually calls are implemented; the rest panic to flag unexpected use.
type boundedDomainClient struct {
	inner *domainClient
}

func (c *boundedDomainClient) Close() {}

func (c *boundedDomainClient) GuestInterfaces(domain libvirtdomain.Domain) ([]libvirtdomain.GuestInterface, error) {
	return c.inner.GuestInterfaces(domain)
}

func (c *boundedDomainClient) Domains() ([]libvirtdomain.Domain, error) {
	panic("bounded client used for domain inventory")
}

func (c *boundedDomainClient) Active(libvirtdomain.Domain) (bool, error) {
	panic("bounded client used for active check")
}

func (c *boundedDomainClient) Info(libvirtdomain.Domain) (libvirtdomain.Info, error) {
	panic("bounded client used for info")
}

func (c *boundedDomainClient) Start(libvirtdomain.Domain, string, ...libvirtdomain.StartOption) error {
	panic("bounded client used for start")
}

func (c *boundedDomainClient) Remove(libvirtdomain.Domain) error {
	panic("bounded client used for remove")
}

func (c *boundedDomainClient) Shutdown(libvirtdomain.Domain) error {
	panic("bounded client used for shutdown")
}

func (c *boundedDomainClient) Reboot(libvirtdomain.Domain) error {
	panic("bounded client used for reboot")
}

func (s *VirtualMachineStatusSuite) startObserver() {
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainStatusController{
		Open: s.open, OpenBounded: s.openBounded, Watch: func(context.Context) (<-chan struct{}, error) { return s.events, nil },
	}))
}

func (s *VirtualMachineStatusSuite) TestLifecycleEventRefreshesUnmanagedDomainWithoutPolling() {
	domain := libvirtdomain.Domain{Name: "event-only", UUID: uuid.New()}

	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainStatusController{
		Open: s.open, OpenBounded: s.openBounded, Watch: func(context.Context) (<-chan struct{}, error) { return s.events, nil },
	}))

	s.Require().Eventually(func() bool {
		select {
		case <-s.client.listed:
			return true
		default:
			return false
		}
	}, 5*time.Second, 10*time.Millisecond)

	s.client.mu.Lock()
	s.client.domains[domain.Name] = domain
	s.client.mu.Unlock()

	s.events <- struct{}{}

	s.Require().Eventually(func() bool {
		status, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainStatus](s.Ctx(), s.State(), domain.Name)

		return err == nil && status.TypedSpec().UUID == domain.UUID.String()
	}, 5*time.Second, 10*time.Millisecond)
	s.Require().EqualValues(1, s.opens.Load(), "lifecycle events must reuse the startup inventory session")
}

func (s *VirtualMachineStatusSuite) registerReadinessObserver(
	watchCanceled chan<- struct{},
	subscriptions *atomic.Int32,
) {
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainStatusController{
		Open: func(ctx context.Context) (libvirtdomain.Client, error) {
			if subscriptions.Load() == 0 {
				return nil, errors.New("watch was not registered before open")
			}

			return s.open(ctx)
		},
		OpenBounded: s.openBounded,
		Watch: func(ctx context.Context) (<-chan struct{}, error) {
			subscriptions.Add(1)

			events := make(chan struct{}, 1)

			go func() {
				<-ctx.Done()

				watchCanceled <- struct{}{}
			}()

			return events, nil
		},
	}))
}

func (s *VirtualMachineStatusSuite) assertObserverIdle(subscriptions *atomic.Int32) {
	s.Require().Never(func() bool {
		return subscriptions.Load() != 0 || s.opens.Load() != 0
	}, 100*time.Millisecond, 10*time.Millisecond)
}

func (s *VirtualMachineStatusSuite) assertWatchCanceled(watchCanceled <-chan struct{}) {
	s.Require().Eventually(func() bool {
		select {
		case <-watchCanceled:
			return true
		default:
			return false
		}
	}, 5*time.Second, 10*time.Millisecond)
}

func (s *VirtualMachineStatusSuite) assertClientCloseCount(expected int) {
	s.Require().Eventually(func() bool {
		s.client.mu.Lock()
		defer s.client.mu.Unlock()

		return s.client.closes == expected
	}, 5*time.Second, 10*time.Millisecond)
}

func (s *VirtualMachineStatusSuite) assertDomainObservation(
	name string,
	powerState hypervisorhelpers.VirtualMachinePowerState,
	errorText string,
) {
	s.Require().Eventually(func() bool {
		status, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainStatus](s.Ctx(), s.State(), name)
		if err != nil {
			return false
		}

		return status.TypedSpec().PowerState == powerState && status.TypedSpec().Error == errorText
	}, 5*time.Second, 10*time.Millisecond)
}

func (s *VirtualMachineStatusSuite) assertManagedStatusUnavailable(name string) {
	s.Require().Eventually(func() bool {
		domainStatus, domainErr := safe.StateGetByID[*hypervisor.VirtualMachineDomainStatus](s.Ctx(), s.State(), name)
		if domainErr != nil {
			return false
		}

		status, statusErr := safe.StateGetByID[*hypervisor.VirtualMachineStatus](s.Ctx(), s.State(), name)
		if statusErr != nil {
			return false
		}

		return domainStatus.TypedSpec().UUID == libvirtdomain.UUID(uuid.MustParse(machineUUID), name).String() &&
			domainStatus.TypedSpec().PowerState == hypervisorhelpers.VirtualMachinePowerStateUnknown &&
			domainStatus.TypedSpec().Error != "" &&
			status.TypedSpec().PowerState == hypervisorhelpers.VirtualMachinePowerStateUnknown &&
			status.TypedSpec().Stage == hypervisorhelpers.VirtualMachineStageError && status.TypedSpec().Error != ""
	}, 5*time.Second, 10*time.Millisecond)
}

func (s *VirtualMachineStatusSuite) TestServiceReadinessControlsObserverLifecycle() {
	service, err := safe.StateGetByID[*v1alpha1.Service](s.Ctx(), s.State(), virtqemudServiceID)
	s.Require().NoError(err)
	s.Destroy(service)

	domain := libvirtdomain.Domain{Name: "gated", UUID: uuid.New()}
	s.client.domains[domain.Name] = domain

	watchCanceled := make(chan struct{}, 3)

	var subscriptions atomic.Int32

	s.registerReadinessObserver(watchCanceled, &subscriptions)
	s.assertObserverIdle(&subscriptions)

	starting := v1alpha1.NewService(virtqemudServiceID)
	starting.TypedSpec().Unknown = true
	s.Create(starting)
	s.assertObserverIdle(&subscriptions)

	ctest.UpdateWithConflicts(s, starting, func(resource *v1alpha1.Service) error {
		resource.TypedSpec().Running = true

		return nil
	})
	s.assertDomainObservation(domain.Name, hypervisorhelpers.VirtualMachinePowerStateRunning, "")
	s.Require().EqualValues(1, subscriptions.Load())
	s.Require().EqualValues(1, s.opens.Load())

	s.Destroy(starting)
	s.assertWatchCanceled(watchCanceled)
	s.assertClientCloseCount(1)
	s.assertDomainObservation(domain.Name, hypervisorhelpers.VirtualMachinePowerStateUnknown, "virtqemud service is not ready")

	s.Create(newReadyVirtqemudService())
	s.assertDomainObservation(domain.Name, hypervisorhelpers.VirtualMachinePowerStateRunning, "")
	s.Require().Eventually(func() bool {
		return subscriptions.Load() == 2 && s.opens.Load() == 2
	}, 5*time.Second, 10*time.Millisecond)

	s.TearDownTest()
	s.assertWatchCanceled(watchCanceled)
	s.assertClientCloseCount(2)
}

func (s *VirtualMachineStatusSuite) TestLifecycleEventRemovesDisappearedDomainWithoutPolling() {
	domain := libvirtdomain.Domain{Name: "event-only", UUID: uuid.New()}
	s.client.domains[domain.Name] = domain
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainStatusController{
		Open: s.open, OpenBounded: s.openBounded, Watch: func(context.Context) (<-chan struct{}, error) { return s.events, nil },
	}))

	s.Require().Eventually(func() bool {
		_, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainStatus](s.Ctx(), s.State(), domain.Name)

		return err == nil
	}, 5*time.Second, 10*time.Millisecond)

	s.client.mu.Lock()
	delete(s.client.domains, domain.Name)
	s.client.mu.Unlock()

	s.events <- struct{}{}

	s.Require().Eventually(func() bool {
		_, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainStatus](s.Ctx(), s.State(), domain.Name)

		return err != nil
	}, 5*time.Second, 10*time.Millisecond)
}

func (s *VirtualMachineStatusSuite) TestClosedLifecycleWatchKeepsObservationUntilSuccessfulScan() {
	domain := libvirtdomain.Domain{Name: "event-only", UUID: uuid.New()}
	s.client.domains[domain.Name] = domain

	resume := make(chan struct{})
	retrying := make(chan struct{}, 1)

	var subscriptions atomic.Int32

	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainStatusController{
		Open:        s.open,
		OpenBounded: s.openBounded,
		Watch: func(ctx context.Context) (<-chan struct{}, error) {
			switch subscriptions.Add(1) {
			case 1:
				return s.events, nil
			case 2:
				return nil, errors.New("watch unavailable")
			}

			retrying <- struct{}{}

			select {
			case <-resume:
				return make(chan struct{}), nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}))

	s.Require().Eventually(func() bool {
		_, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainStatus](s.Ctx(), s.State(), domain.Name)

		return err == nil
	}, 5*time.Second, 10*time.Millisecond)

	s.client.mu.Lock()
	delete(s.client.domains, domain.Name)
	s.client.mu.Unlock()
	close(s.events)

	s.Require().Eventually(func() bool {
		select {
		case <-retrying:
			return true
		default:
			return false
		}
	}, 5*time.Second, 10*time.Millisecond)

	status, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainStatus](s.Ctx(), s.State(), domain.Name)
	s.Require().NoError(err)
	s.Require().Equal(domain.UUID.String(), status.TypedSpec().UUID)

	close(resume)
	s.Require().Eventually(func() bool {
		_, getErr := safe.StateGetByID[*hypervisor.VirtualMachineDomainStatus](s.Ctx(), s.State(), domain.Name)

		return getErr != nil
	}, 5*time.Second, 10*time.Millisecond)
}

func (s *VirtualMachineStatusSuite) start() {
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainSpecController{}))
	s.startObserver()
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineStatusController{}))
}

func (s *VirtualMachineStatusSuite) TestUnmanagedDomainIsObservedWithoutSpec() {
	domain := libvirtdomain.Domain{Name: "foreign", UUID: uuid.New()}
	s.client.domains[domain.Name] = domain
	s.startObserver()

	s.Require().Eventually(func() bool {
		status, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainStatus](s.Ctx(), s.State(), domain.Name)

		return err == nil && status.TypedSpec().UUID == domain.UUID.String() &&
			status.TypedSpec().PowerState == hypervisorhelpers.VirtualMachinePowerStateRunning &&
			status.TypedSpec().State == 1 && status.TypedSpec().MaxMemoryKiB == 1048576 &&
			status.TypedSpec().MemoryKiB == 524288 && status.TypedSpec().VCPUs == 2
	}, 5*time.Second, 10*time.Millisecond)
}

func (s *VirtualMachineStatusSuite) TestDisappearedDomainIsRemovedAfterSuccessfulScan() {
	domain := libvirtdomain.Domain{Name: "foreign", UUID: uuid.New()}
	s.client.domains[domain.Name] = domain
	s.startObserver()

	s.Require().Eventually(func() bool {
		_, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainStatus](s.Ctx(), s.State(), domain.Name)

		return err == nil
	}, 5*time.Second, 10*time.Millisecond)

	s.client.mu.Lock()
	delete(s.client.domains, domain.Name)
	s.client.mu.Unlock()

	s.events <- struct{}{}

	s.Require().Eventually(func() bool {
		_, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainStatus](s.Ctx(), s.State(), domain.Name)

		return err != nil
	}, 5*time.Second, 10*time.Millisecond)
}

func (s *VirtualMachineStatusSuite) TestFailedStatusWriteStillRemovesDisappearedDomains() {
	old := libvirtdomain.Domain{Name: "old", UUID: uuid.New()}
	s.client.domains[old.Name] = old
	s.startObserver()

	s.Require().Eventually(func() bool {
		_, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainStatus](s.Ctx(), s.State(), old.Name)

		return err == nil
	}, 5*time.Second, 10*time.Millisecond)

	blocked := hypervisor.NewVirtualMachineDomainStatus(hypervisor.NamespaceName, "blocked")
	s.Create(blocked)

	s.client.mu.Lock()
	delete(s.client.domains, old.Name)
	s.client.domains["blocked"] = libvirtdomain.Domain{Name: "blocked", UUID: uuid.New()}
	s.client.mu.Unlock()

	s.events <- struct{}{}

	s.Require().Eventually(func() bool {
		_, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainStatus](s.Ctx(), s.State(), old.Name)

		return err != nil
	}, 5*time.Second, 10*time.Millisecond)
}

func (s *VirtualMachineStatusSuite) TestFailedInventoryKeepsObservationUntilSuccessfulScan() {
	domain := libvirtdomain.Domain{Name: "foreign", UUID: uuid.New()}
	s.client.domains[domain.Name] = domain
	s.startObserver()

	s.Require().Eventually(func() bool {
		_, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainStatus](s.Ctx(), s.State(), domain.Name)

		return err == nil
	}, 5*time.Second, 10*time.Millisecond)

	s.client.mu.Lock()
	delete(s.client.domains, domain.Name)
	s.client.listErr = errors.New("inventory unavailable")
	s.client.mu.Unlock()

	s.events <- struct{}{}

	s.Require().Eventually(func() bool { return s.opens.Load() >= 2 }, 5*time.Second, 10*time.Millisecond)

	status, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainStatus](s.Ctx(), s.State(), domain.Name)
	s.Require().NoError(err)
	s.Require().Equal(domain.UUID.String(), status.TypedSpec().UUID)

	s.client.mu.Lock()
	s.client.listErr = nil
	s.client.mu.Unlock()

	s.events <- struct{}{}

	s.Require().Eventually(func() bool {
		_, getErr := safe.StateGetByID[*hypervisor.VirtualMachineDomainStatus](s.Ctx(), s.State(), domain.Name)

		return getErr != nil
	}, 5*time.Second, 10*time.Millisecond)
}

// A stopped virtual machine with no domain is ready without a domain status: a domain status means
// libvirt lists the domain, which other consumers such as CPU partitioning rely on.
func (s *VirtualMachineStatusSuite) TestUnclaimedStoppedVMIsReadyWithoutDomainStatus() {
	const name = "vm1"

	spec := newRenderableSpec(name, "stopped")
	s.Create(spec)
	s.start()

	s.assertStatus(name, "stopped", hypervisorhelpers.VirtualMachineStageReady, "")
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainStatus](s, name)

	// A real, still-defined domain is not a completed stop.
	s.client.mu.Lock()
	s.client.domains[name] = libvirtdomain.Domain{Name: name, UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), name)}
	s.client.mu.Unlock()

	s.events <- struct{}{}

	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStagePending, "domain is still defined")

	s.client.mu.Lock()
	delete(s.client.domains, name)
	s.client.mu.Unlock()

	s.events <- struct{}{}

	s.assertStatus(name, "stopped", hypervisorhelpers.VirtualMachineStageReady, "")
}

func (s *VirtualMachineStatusSuite) TestStoppedObservationWaitsForRuntimeClaim() {
	const name = "stopping"

	s.Create(newRenderableSpec(name, "stopped"))
	definition := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, name)
	definition.TypedSpec().PowerState = "stopped"
	owner := (&hypervisorctrl.VirtualMachineController{}).Name()
	definition.Metadata().Finalizers().Add(owner)
	s.Create(definition)

	// The claim is released only once VirtualMachineController finds the domain gone, so until
	// then an absent observation may be a scan which has not caught up.
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineStatusController{}))
	s.assertStatus(name, "unknown", hypervisorhelpers.VirtualMachineStageUnknown, "domain has not been observed")

	s.Require().NoError(s.State().RemoveFinalizer(s.Ctx(), definition.Metadata(), owner))
	s.assertStatus(name, "stopped", hypervisorhelpers.VirtualMachineStageReady, "")
}

func (s *VirtualMachineStatusSuite) TestRunningStatusWaitsForAMissingRenderedDefinition() {
	const name = "missing-definition"

	s.Create(newRenderableSpec(name, "running"))
	observed := hypervisor.NewVirtualMachineDomainStatus(hypervisor.NamespaceName, name)
	observed.TypedSpec().UUID = libvirtdomain.UUID(uuid.MustParse(machineUUID), name).String()
	observed.TypedSpec().PowerState = hypervisorhelpers.VirtualMachinePowerStateRunning
	observed.TypedSpec().State = 1
	s.Create(observed)
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineStatusController{}))

	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStagePending, "domain definition has not caught up")
}

func (s *VirtualMachineStatusSuite) TestRunningStatusWaitsForCurrentRenderedDefinition() {
	const name = "render-lag"

	vm := newRenderableSpec(name, "running")
	s.Create(vm)
	s.client.domains[name] = libvirtdomain.Domain{
		Name: name,
		UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), name),
	}

	s.start()
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStageReady, "")

	old, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), name)
	s.Require().NoError(err)
	s.Require().NotEmpty(old.TypedSpec().DomainXML)

	ctest.UpdateWithConflicts(s, vm, func(current *hypervisor.VirtualMachineSpec) error {
		current.TypedSpec().Memory.Size = 2 << 30

		return nil
	})

	updated, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](s.Ctx(), s.State(), name)
	s.Require().NoError(err)
	s.Require().Eventually(func() bool {
		current, getErr := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), name)

		return getErr == nil && current.TypedSpec().ObservedGeneration == hypervisor.VirtualMachineSpecGeneration(updated) &&
			current.TypedSpec().DomainXML != old.TypedSpec().DomainXML
	}, 5*time.Second, 10*time.Millisecond)

	current, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), name)
	s.Require().NoError(err)
	s.Require().NotEqual(old.TypedSpec().ObservedGeneration, current.TypedSpec().ObservedGeneration)

	// Simulate a delayed renderer publication after the desired spec has changed.
	ctest.UpdateWithConflicts(s, current, func(domain *hypervisor.VirtualMachineDomainSpec) error {
		*domain.TypedSpec() = *old.TypedSpec()

		return nil
	}, state.WithUpdateOwner((&hypervisorctrl.VirtualMachineDomainSpecController{}).Name()))
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStagePending, "domain definition has not caught up")

	// Numeric versions can be reused across spec incarnations. Even a matching number
	// cannot certify a definition whose rendered XML still belongs to the old intent.
	ctest.UpdateWithConflicts(s, current, func(domain *hypervisor.VirtualMachineDomainSpec) error {
		domain.TypedSpec().ObservedGeneration = hypervisor.VirtualMachineSpecGeneration(updated)

		return nil
	}, state.WithUpdateOwner((&hypervisorctrl.VirtualMachineDomainSpecController{}).Name()))
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStagePending, "domain definition has not caught up")

	ctest.UpdateWithConflicts(s, current, func(domain *hypervisor.VirtualMachineDomainSpec) error {
		*domain.TypedSpec() = *current.TypedSpec()

		return nil
	}, state.WithUpdateOwner((&hypervisorctrl.VirtualMachineDomainSpecController{}).Name()))
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStageReady, "")
}

func (s *VirtualMachineStatusSuite) TestRunningStatusRejectsAnIdenticalDefinitionFromAPreviousIncarnation() {
	const name = "same-name-incarnation"

	original := newRenderableSpec(name, "running")
	s.Create(original)
	s.client.domains[name] = libvirtdomain.Domain{Name: name, UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), name)}
	s.start()
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStageReady, "")

	old, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), name)
	s.Require().NoError(err)
	s.Require().NoError(s.State().Destroy(s.Ctx(), original.Metadata()))
	s.Create(newRenderableSpec(name, "running"))
	current, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](s.Ctx(), s.State(), name)
	s.Require().NoError(err)

	generation := hypervisor.VirtualMachineSpecGeneration(current)
	s.Require().NotEqual(old.TypedSpec().ObservedGeneration, generation)
	s.Require().Eventually(func() bool {
		definition, getErr := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), name)

		return getErr == nil && definition.TypedSpec().ObservedGeneration == generation
	}, 5*time.Second, 10*time.Millisecond)

	// The VM name, numeric resource version and rendered bytes are identical.
	// Only the source incarnation distinguishes this stale definition.
	ctest.UpdateWithConflicts(s, old, func(definition *hypervisor.VirtualMachineDomainSpec) error {
		*definition.TypedSpec() = *old.TypedSpec()

		return nil
	}, state.WithUpdateOwner((&hypervisorctrl.VirtualMachineDomainSpecController{}).Name()))
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStagePending, "domain definition has not caught up")

	ctest.UpdateWithConflicts(s, old, func(definition *hypervisor.VirtualMachineDomainSpec) error {
		definition.TypedSpec().ObservedGeneration = generation

		return nil
	}, state.WithUpdateOwner((&hypervisorctrl.VirtualMachineDomainSpecController{}).Name()))
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStageReady, "")
}

func (s *VirtualMachineStatusSuite) TestStatusReportsCurrentSpecGeneration() {
	const name = "generation"

	spec := newRenderableSpec(name, "stopped")
	s.Create(spec)
	s.start()

	stored, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](s.Ctx(), s.State(), name)
	s.Require().NoError(err)

	initial := hypervisor.VirtualMachineSpecGeneration(stored)

	s.Require().Eventually(func() bool {
		status, getErr := safe.StateGetByID[*hypervisor.VirtualMachineStatus](s.Ctx(), s.State(), name)
		definition, domainErr := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), name)

		return getErr == nil && domainErr == nil && status.TypedSpec().ObservedGeneration == hypervisor.VirtualMachineStatusGeneration(initial, definition) &&
			status.TypedSpec().Stage == hypervisorhelpers.VirtualMachineStageReady
	}, 5*time.Second, 10*time.Millisecond)

	ctest.UpdateWithConflicts(s, stored, func(current *hypervisor.VirtualMachineSpec) error {
		current.TypedSpec().PowerState = "running"

		return nil
	})

	updated, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](s.Ctx(), s.State(), name)
	s.Require().NoError(err)
	s.Require().NotEqual(initial, hypervisor.VirtualMachineSpecGeneration(updated))

	s.Require().Eventually(func() bool {
		status, getErr := safe.StateGetByID[*hypervisor.VirtualMachineStatus](s.Ctx(), s.State(), name)

		definition, domainErr := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), name)

		return getErr == nil && domainErr == nil && definition.TypedSpec().ObservedGeneration == hypervisor.VirtualMachineSpecGeneration(updated) &&
			status.TypedSpec().ObservedGeneration == hypervisor.VirtualMachineStatusGeneration(hypervisor.VirtualMachineSpecGeneration(updated), definition) &&
			status.TypedSpec().Stage != hypervisorhelpers.VirtualMachineStageReady
	}, 5*time.Second, 10*time.Millisecond)
}

// A running virtual machine is not taken for one which stopped because the scan fails.
func (s *VirtualMachineStatusSuite) TestAbsentDomainIsNotReadyDuringFailedScan() {
	s.openErr.Store(&openFailure{err: errors.New("daemon unavailable")})

	spec := newRenderableSpec("vm1", "running")
	s.Create(spec)
	s.start()
	s.assertStatus("vm1", "unknown", hypervisorhelpers.VirtualMachineStageUnknown, "domain has not been observed")
}

// newRenderableSpec builds a spec that renders, so a test about something other than rendering
// does not trip the renderer's own validation.
func newRenderableSpec(name, powerState string) *hypervisor.VirtualMachineSpec {
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, name)
	spec.TypedSpec().CPU = hypervisor.VirtualMachineCPUSpec{Count: 1}
	spec.TypedSpec().Memory = hypervisor.VirtualMachineMemorySpec{Size: 1 << 30}
	spec.TypedSpec().Firmware = hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"}
	spec.TypedSpec().PowerState = powerState

	return spec
}

func (s *VirtualMachineStatusSuite) assertStatus(name, state string, stage hypervisorhelpers.VirtualMachineStage, errorText string) {
	s.Require().Eventually(func() bool {
		status, err := safe.StateGetByID[*hypervisor.VirtualMachineStatus](s.Ctx(), s.State(), name)
		if err != nil {
			return false
		}

		actual := status.TypedSpec()

		return actual.PowerState.String() == state && actual.Stage == stage && actual.Error == errorText
	}, 5*time.Second, 10*time.Millisecond)
}

func (s *VirtualMachineStatusSuite) TestStoppedAndRunningAreObserved() {
	spec := newRenderableSpec("vm1", "stopped")
	s.Create(spec)
	s.start()
	s.assertStatus("vm1", "stopped", hypervisorhelpers.VirtualMachineStageReady, "")

	updated, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](s.Ctx(), s.State(), "vm1")
	s.Require().NoError(err)

	updated.TypedSpec().PowerState = "running"
	s.Update(updated)
	s.assertStatus("vm1", "unknown", hypervisorhelpers.VirtualMachineStageUnknown, "domain has not been observed")

	s.client.mu.Lock()
	s.client.domains["vm1"] = libvirtdomain.Domain{Name: "vm1", UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), "vm1")}
	s.client.mu.Unlock()

	s.events <- struct{}{}

	s.assertStatus("vm1", "running", hypervisorhelpers.VirtualMachineStageReady, "")

	second := newRenderableSpec("vm2", "stopped")
	s.Create(second)
	s.assertStatus("vm2", "stopped", hypervisorhelpers.VirtualMachineStageReady, "")
	s.assertStatus("vm1", "running", hypervisorhelpers.VirtualMachineStageReady, "")
}

// Guest-agent interfaces surface on the VirtualMachineStatus, loopback addresses are dropped
// regardless of interface name, and the per-interface view is preserved.
func (s *VirtualMachineStatusSuite) TestGuestAgentAddressesSurfaceOnStatus() {
	name := "vm1"
	s.client.domains[name] = libvirtdomain.Domain{Name: name, UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), name)}
	s.client.guestInterfaces = []libvirtdomain.GuestInterface{
		{
			// Linux loopback: all addresses must be dropped.
			Name: "lo",
			IPs: []netip.Prefix{
				netip.MustParsePrefix("127.0.0.1/8"),
				netip.MustParsePrefix("::1/128"),
			},
		},
		{
			// Non-loopback interface with a mix of real and loopback-range addresses.
			Name:         "eth0",
			HardwareAddr: "52:54:00:12:34:56",
			IPs: []netip.Prefix{
				netip.MustParsePrefix("10.0.0.5/24"),
				netip.MustParsePrefix("127.0.0.2/8"),
				netip.MustParsePrefix("fe80::1/64"),
			},
		},
	}

	spec := newRenderableSpec(name, "running")
	s.Create(spec)

	domainSpec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, name)
	domainSpec.TypedSpec().PowerState = "running"
	domainSpec.TypedSpec().DomainXML = `<domain><name>vm1</name></domain>`
	s.Create(domainSpec)
	s.start()

	s.assertGuestAddresses(name, []string{"10.0.0.5/24", "fe80::1/64"}, 2)

	// A refresh after the guest reports new addresses must propagate.
	s.client.mu.Lock()
	s.client.guestInterfaces[1].IPs = []netip.Prefix{
		netip.MustParsePrefix("10.0.0.6/24"),
	}
	s.client.mu.Unlock()

	s.events <- struct{}{}

	s.assertGuestAddresses(name, []string{"10.0.0.6/24"}, 2)
}

// Addresses must drop when observation stops: the status's error field already names the outage,
// and keeping the last known values would misreport reachability. virtqemud leaving triggers
// markUnavailable across every observed domain.
func (s *VirtualMachineStatusSuite) TestObservationOutageClearsGuestAddresses() {
	name := "vm1"
	s.client.domains[name] = libvirtdomain.Domain{Name: name, UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), name)}
	s.client.guestInterfaces = []libvirtdomain.GuestInterface{
		{
			Name:         "eth0",
			HardwareAddr: "52:54:00:12:34:56",
			IPs: []netip.Prefix{
				netip.MustParsePrefix("10.0.0.5/24"),
			},
		},
	}

	spec := newRenderableSpec(name, "running")
	s.Create(spec)

	domainSpec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, name)
	domainSpec.TypedSpec().PowerState = "running"
	domainSpec.TypedSpec().DomainXML = `<domain><name>vm1</name></domain>`
	s.Create(domainSpec)
	s.start()

	s.assertGuestAddresses(name, []string{"10.0.0.5/24"}, 1)

	// virtqemud away triggers markUnavailable, which must drop interfaces as well as power state.
	service, err := safe.StateGetByID[*v1alpha1.Service](s.Ctx(), s.State(), virtqemudServiceID)
	s.Require().NoError(err)
	s.Destroy(service)

	s.Require().Eventually(func() bool {
		status, err := safe.StateGetByID[*hypervisor.VirtualMachineStatus](s.Ctx(), s.State(), name)
		if err != nil {
			return false
		}

		return len(status.TypedSpec().Addresses) == 0 && len(status.TypedSpec().Interfaces) == 0
	}, 5*time.Second, 10*time.Millisecond)
}

func (s *VirtualMachineStatusSuite) assertGuestAddresses(name string, want []string, interfaces int) {
	s.Require().Eventually(func() bool {
		status, err := safe.StateGetByID[*hypervisor.VirtualMachineStatus](s.Ctx(), s.State(), name)
		if err != nil {
			return false
		}

		actual := status.TypedSpec()
		if len(actual.Addresses) != len(want) || len(actual.Interfaces) != interfaces {
			return false
		}

		for i, addr := range want {
			if actual.Addresses[i] != addr {
				return false
			}
		}

		return true
	}, 5*time.Second, 10*time.Millisecond)
}

// A pending graceful stop is reported from the stop mode the domain spec carries: a stop forced by
// the domain spec is not one the operator can escape by forcing it.
func (s *VirtualMachineStatusSuite) TestPendingStopFollowsTheDomainSpec() {
	s.Create(newRenderableSpec("vm1", "stopped"))

	domainSpec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "vm1")
	domainSpec.TypedSpec().PowerState = "stopped"
	domainSpec.TypedSpec().StopMode = hypervisorhelpers.StopModeForced.String()
	domainSpec.TypedSpec().DomainXML = `<domain><name>vm1</name></domain>`
	s.Create(domainSpec)

	s.client.mu.Lock()
	s.client.domains["vm1"] = libvirtdomain.Domain{Name: "vm1", UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), "vm1")}
	s.client.mu.Unlock()

	s.start()

	s.events <- struct{}{}

	s.assertStatus("vm1", "running", hypervisorhelpers.VirtualMachineStagePending, "domain is still defined")

	updated, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), "vm1")
	s.Require().NoError(err)

	updated.TypedSpec().StopMode = hypervisorhelpers.StopModeGraceful.String()
	s.Update(updated)

	s.assertStatus("vm1", "running", hypervisorhelpers.VirtualMachineStagePending, hypervisor.GracefulStopPendingError)
}

// A link the host lacks costs the spec its power intent, so the running domain is on its way out:
// readiness would be a lie.
func (s *VirtualMachineStatusSuite) TestUnresolvedLinkHoldsBackReadiness() {
	name := "vm1"
	s.client.domains[name] = libvirtdomain.Domain{Name: name, UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), name)}

	spec := newRenderableSpec(name, "running")
	spec.TypedSpec().Interfaces = []hypervisor.VirtualMachineInterfaceSpec{{Name: "net0", Link: "uplink"}}
	s.Create(spec)
	s.start()

	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStagePending,
		`virtual machine "vm1": interface "net0": host link not found: "uplink"`)

	link := network.NewLinkStatus(network.NamespaceName, "macvlan0")
	link.TypedSpec().Type = nethelpers.LinkEther
	link.TypedSpec().Alias = "uplink"
	s.Create(link)

	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStageReady, "")
}

// The same obstacle, before the domain was ever defined: the missing link is the reason, not the
// generic absence of an observation.
func (s *VirtualMachineStatusSuite) TestUnresolvedLinkExplainsUndefinedDomain() {
	spec := newRenderableSpec("vm1", "running")
	spec.TypedSpec().Interfaces = []hypervisor.VirtualMachineInterfaceSpec{{Name: "net0", Link: "uplink"}}
	s.Create(spec)
	s.start()

	s.assertStatus("vm1", "unknown", hypervisorhelpers.VirtualMachineStagePending,
		`virtual machine "vm1": interface "net0": host link not found: "uplink"`)
}

// A disk whose status has not been published yet is the same kind of obstacle as a link the host
// has not brought up: worth waiting for, not a config error.
func (s *VirtualMachineStatusSuite) TestUnresolvedDiskHoldsBackReadiness() {
	name := "vm1"
	s.client.domains[name] = libvirtdomain.Domain{Name: name, UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), name)}

	disk := hypervisor.VirtualMachineDiskSpec{
		Name: "install",
		Bus:  hypervisorhelpers.VirtualMachineDiskBusSATA.String(),
		Type: hypervisorhelpers.VirtualMachineDiskTypeCDROM.String(),
		Provision: hypervisor.VirtualMachineDiskProvisionSpec{
			FromImage: &hypervisor.VirtualMachineDiskFromImageSpec{Library: "vm-images", File: "talos.iso"},
		},
	}

	spec := newRenderableSpec(name, "running")
	spec.TypedSpec().Disks = []hypervisor.VirtualMachineDiskSpec{disk}
	s.Create(spec)
	s.start()

	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStagePending,
		`virtual machine "vm1": disk "install" is not ready: no disk status yet`)

	status := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, hypervisor.VirtualMachineDiskStatusID(name, disk))
	status.TypedSpec().VirtualMachine = name
	status.TypedSpec().Name = "install"
	status.TypedSpec().SourcePath = "/var/lib/libvirt/images/vm1-install.qcow2"
	status.TypedSpec().Format = "qcow2"
	status.TypedSpec().Phase = hypervisor.VirtualMachineDiskPhaseReady
	status.TypedSpec().Image = hypervisor.VirtualMachineDiskFromImageSpec{Library: "vm-images", File: "talos.iso"}
	s.Create(status)

	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStageReady, "")
}

// A link of the wrong type is not something to wait for: the config has to change, so the stage
// says error rather than pending.
func (s *VirtualMachineStatusSuite) TestNonEthernetLinkIsError() {
	link := network.NewLinkStatus(network.NamespaceName, "lo")
	link.TypedSpec().Type = nethelpers.LinkLoopbck
	s.Create(link)

	spec := newRenderableSpec("vm1", "running")
	spec.TypedSpec().Interfaces = []hypervisor.VirtualMachineInterfaceSpec{{Name: "net0", Link: "lo"}}
	s.Create(spec)
	s.start()

	s.assertStatus("vm1", "unknown", hypervisorhelpers.VirtualMachineStageError,
		`virtual machine "vm1": interface "net0": host link is not an Ethernet link: "lo"`)
}

// A disk this slice does not provision is not something to wait for: no host resource will ever
// resolve it, so the stage says error rather than pending.
// A disk nothing on the host will ever make attachable is an error rather than something to wait
// for: only a change to the configuration helps. Copying a disk from a content library image is not
// implemented, so that is the case here.
func (s *VirtualMachineStatusSuite) TestUnsupportedDiskIsError() {
	spec := newRenderableSpec("vm1", "running")
	spec.TypedSpec().Disks = []hypervisor.VirtualMachineDiskSpec{{
		Name:   "system",
		Pool:   "pool1",
		Size:   20 << 30,
		Format: hypervisorhelpers.VirtualMachineDiskFormatQCOW2.String(),
		Type:   hypervisorhelpers.VirtualMachineDiskTypeDisk.String(),
		Provision: hypervisor.VirtualMachineDiskProvisionSpec{
			FromImage: &hypervisor.VirtualMachineDiskFromImageSpec{Library: "images", File: "talos.qcow2"},
		},
	}}
	s.Create(spec)
	s.start()

	s.assertStatus("vm1", "unknown", hypervisorhelpers.VirtualMachineStageError,
		`virtual machine "vm1": disk "system": unsupported disk: a disk is only provisioned from provision.blank today`)
}

// A blank disk whose volume is not there yet is worth waiting for, so it is Pending rather than an
// error: the pool may simply not have reconciled.
func (s *VirtualMachineStatusSuite) TestBlankDiskAwaitingItsVolumeIsPending() {
	spec := newRenderableSpec("vm1", "running")
	spec.TypedSpec().Disks = []hypervisor.VirtualMachineDiskSpec{{
		Name:      "data",
		Pool:      "pool1",
		Size:      20 << 30,
		Format:    hypervisorhelpers.VirtualMachineDiskFormatQCOW2.String(),
		Type:      hypervisorhelpers.VirtualMachineDiskTypeDisk.String(),
		Provision: hypervisor.VirtualMachineDiskProvisionSpec{Blank: true},
	}}
	s.Create(spec)
	s.start()

	s.assertStatus("vm1", "unknown", hypervisorhelpers.VirtualMachineStagePending,
		`virtual machine "vm1": disk "data" is not ready: no disk status yet`)
}

func (s *VirtualMachineStatusSuite) TestForeignDomainIsError() {
	spec := newRenderableSpec("vm1", "running")
	s.Create(spec)
	s.client.domains["vm1"] = libvirtdomain.Domain{Name: "vm1", UUID: uuid.New()}
	s.start()
	s.assertStatus("vm1", "unknown", hypervisorhelpers.VirtualMachineStageError, "domain name is occupied by another VM")
}

func (s *VirtualMachineStatusSuite) TestUnsupportedPowerStateIsError() {
	spec := newRenderableSpec("vm1", "suspended")
	s.Create(spec)
	s.start()
	s.assertStatus("vm1", "unknown", hypervisorhelpers.VirtualMachineStageError, `unsupported power state "suspended"`)
}

func (s *VirtualMachineStatusSuite) TestDaemonUnavailable() {
	s.openErr.Store(&openFailure{err: errors.New("daemon unavailable")})

	spec := newRenderableSpec("vm1", "running")
	s.Create(spec)
	s.start()
	s.assertStatus("vm1", "unknown", hypervisorhelpers.VirtualMachineStageUnknown, "domain has not been observed")
}

func (s *VirtualMachineStatusSuite) TestLibvirtOutageMarksObservationUnknownUntilSuccessfulScan() {
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainSpecController{}))

	name := "vm1"
	s.client.domains[name] = libvirtdomain.Domain{Name: name, UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), name)}

	spec := newRenderableSpec(name, "running")
	s.Create(spec)

	resume := make(chan struct{})

	var subscriptions atomic.Int32

	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainStatusController{
		Open:        s.open,
		OpenBounded: s.openBounded,
		Watch: func(ctx context.Context) (<-chan struct{}, error) {
			switch subscriptions.Add(1) {
			case 1:
				return s.events, nil
			case 2:
				return make(chan struct{}), nil
			}

			select {
			case <-resume:
				return make(chan struct{}), nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}))
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineStatusController{}))
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStageReady, "")

	s.openErr.Store(&openFailure{err: errors.New("daemon unavailable")})
	close(s.events)
	s.Require().Eventually(func() bool { return subscriptions.Load() >= 3 }, 5*time.Second, 10*time.Millisecond)

	s.assertManagedStatusUnavailable(name)

	s.client.mu.Lock()
	delete(s.client.domains, name)
	s.client.mu.Unlock()
	s.openErr.Store(nil)
	close(resume)

	s.Require().Eventually(func() bool {
		_, getErr := safe.StateGetByID[*hypervisor.VirtualMachineDomainStatus](s.Ctx(), s.State(), name)

		return getErr != nil
	}, 10*time.Second, 10*time.Millisecond)
	s.assertStatus(name, "unknown", hypervisorhelpers.VirtualMachineStageUnknown, "domain has not been observed")
}

func (s *VirtualMachineStatusSuite) TestCloudInitReadinessFollowsSeedAndLibraryEvents() {
	const name = "seeded"

	s.client.domains[name] = libvirtdomain.Domain{
		Name: name,
		UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), name),
	}

	vm := newRenderableSpec(name, "running")
	vm.TypedSpec().CloudInit = &hypervisor.VirtualMachineCloudInitSpec{
		Library:  "images",
		MetaData: "instance-id: seeded\n",
		UserData: "#cloud-config\n",
	}
	s.Create(vm)
	s.start()

	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStagePending,
		`virtual machine "seeded": cloud-init seed is not ready`)

	seed := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, name)
	*seed.TypedSpec() = hypervisor.CloudInitSpecSpec{
		Library:  vm.TypedSpec().CloudInit.Library,
		MetaData: vm.TypedSpec().CloudInit.MetaData,
		UserData: vm.TypedSpec().CloudInit.UserData,
	}
	s.Create(seed)
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStagePending,
		`virtual machine "seeded": cloud-init seed is not ready`)

	projected, err := safe.StateGetByID[*hypervisor.CloudInitSpec](s.Ctx(), s.State(), name)
	s.Require().NoError(err)

	asset := hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, hypervisor.CloudInitStatusID(name, *seed.TypedSpec()))
	*asset.TypedSpec() = hypervisor.CloudInitStatusSpec{
		VirtualMachine:     name,
		Library:            "images",
		Name:               "seed.iso",
		Path:               "/images",
		VolumeID:           "volume-a",
		Digest:             "sha256:seed",
		InputDigest:        seed.TypedSpec().InputDigest(),
		ObservedGeneration: projected.Metadata().Version().String(),
		Error:              "seed generation failed",
	}
	s.Create(asset)
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStagePending,
		`virtual machine "seeded": cloud-init seed is not ready`)

	ctest.UpdateWithConflicts(s, asset, func(current *hypervisor.CloudInitStatus) error {
		current.TypedSpec().Error = ""

		return nil
	})
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStagePending,
		`virtual machine "seeded": cloud-init seed is not ready`)

	ctest.UpdateWithConflicts(s, asset, func(current *hypervisor.CloudInitStatus) error {
		current.TypedSpec().Phase = hypervisor.CloudInitPhaseReady

		return nil
	})
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStagePending,
		`virtual machine "seeded": cloud-init seed is not ready`)

	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "images")
	*library.TypedSpec() = hypervisor.ContentLibraryStatusSpec{
		Path:     "/images",
		VolumeID: "volume-a",
		Phase:    hypervisor.ContentLibraryPhaseReady,
	}
	s.Create(library)
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStageReady, "")

	ctest.UpdateWithConflicts(s, seed, func(current *hypervisor.CloudInitSpec) error {
		current.TypedSpec().UserData = "different seed"

		return nil
	})
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStagePending,
		`virtual machine "seeded": cloud-init seed is not ready`)

	ctest.UpdateWithConflicts(s, seed, func(current *hypervisor.CloudInitSpec) error {
		current.TypedSpec().UserData = vm.TypedSpec().CloudInit.UserData

		return nil
	})
	projected, err = safe.StateGetByID[*hypervisor.CloudInitSpec](s.Ctx(), s.State(), name)
	s.Require().NoError(err)

	ctest.UpdateWithConflicts(s, asset, func(current *hypervisor.CloudInitStatus) error {
		current.TypedSpec().ObservedGeneration = projected.Metadata().Version().String()

		return nil
	})
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStageReady, "")

	ctest.UpdateWithConflicts(s, library, func(current *hypervisor.ContentLibraryStatus) error {
		current.TypedSpec().Phase = hypervisor.ContentLibraryPhaseNotReady

		return nil
	})
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStagePending,
		`virtual machine "seeded": cloud-init seed is not ready`)

	ctest.UpdateWithConflicts(s, library, func(current *hypervisor.ContentLibraryStatus) error {
		current.TypedSpec().Phase = hypervisor.ContentLibraryPhaseReady

		return nil
	})
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStageReady, "")

	s.Destroy(asset)
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStagePending,
		`virtual machine "seeded": cloud-init seed is not ready`)

	republished := hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, hypervisor.CloudInitStatusID(name, *seed.TypedSpec()))
	*republished.TypedSpec() = *asset.TypedSpec()
	republished.TypedSpec().Phase = hypervisor.CloudInitPhaseReady
	republished.TypedSpec().Error = ""
	republished.TypedSpec().ObservedGeneration = projected.Metadata().Version().String()
	s.Create(republished)
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStageReady, "")

	ctest.UpdateWithConflicts(s, republished, func(current *hypervisor.CloudInitStatus) error {
		current.TypedSpec().Phase = hypervisor.CloudInitPhaseNotReady
		current.TypedSpec().Error = "seed generation failed"

		return nil
	})
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStagePending,
		`virtual machine "seeded": cloud-init seed is not ready`)
}

func (s *VirtualMachineStatusSuite) TestCloudInitInvalidationStopsOldDomainWithoutReportingReady() {
	const name = "seeded-old-domain"

	s.client.domains[name] = libvirtdomain.Domain{
		Name: name,
		UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), name),
	}

	vm := newRenderableSpec(name, "running")
	s.Create(vm)
	s.start()
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStageReady, "")

	s.Require().Eventually(func() bool {
		domain, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), name)

		return err == nil && domain.TypedSpec().PowerState == "running" && domain.TypedSpec().DomainXML != ""
	}, 5*time.Second, 10*time.Millisecond)

	ctest.UpdateWithConflicts(s, vm, func(current *hypervisor.VirtualMachineSpec) error {
		current.TypedSpec().CloudInit = &hypervisor.VirtualMachineCloudInitSpec{
			Library:  "images",
			UserData: "#cloud-config\n",
		}

		return nil
	})

	s.Require().Eventually(func() bool {
		domain, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), name)

		return err == nil && domain.TypedSpec().PowerState == "stopped" && domain.TypedSpec().DomainXML != ""
	}, 5*time.Second, 10*time.Millisecond)
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStagePending,
		`virtual machine "seeded-old-domain": cloud-init seed is not ready`)
}

func (s *VirtualMachineStatusSuite) TestStatusRemovedWithSpec() {
	spec := newRenderableSpec("vm1", "stopped")
	s.Create(spec)
	s.start()
	s.assertStatus("vm1", "stopped", hypervisorhelpers.VirtualMachineStageReady, "")
	s.Destroy(spec)
	s.Require().Eventually(func() bool {
		_, err := safe.StateGetByID[*hypervisor.VirtualMachineStatus](s.Ctx(), s.State(), "vm1")

		return err != nil
	}, 5*time.Second, 10*time.Millisecond)
	s.Require().Eventually(func() bool {
		_, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainStatus](s.Ctx(), s.State(), "vm1")

		return err != nil
	}, 5*time.Second, 10*time.Millisecond)
}

func newPlacedSpec(name, cpus, nodes string) *hypervisor.VirtualMachineSpec {
	spec := newRenderableSpec(name, "running")
	spec.TypedSpec().CPU.Pins = []hypervisor.VirtualMachineVCPUPinSpec{{VCPU: 0, CPUs: cpus}}
	spec.TypedSpec().Memory.NUMA = &hypervisor.VirtualMachineMemoryNUMASpec{Mode: "strict", Nodes: nodes}

	return spec
}

// Placement waiting for the inventory is pending; host IDs the host cannot use are an error naming
// the ID; the observed power state of a running domain is kept either way.
func (s *VirtualMachineStatusSuite) TestHostPlacementStatus() {
	name := "vm1"
	s.client.domains[name] = libvirtdomain.Domain{Name: name, UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), name)}

	s.Create(newPlacedSpec(name, "3", "0"))
	s.Create(newPlacedSpec("vm2", "4", "2"))
	s.Create(newRenderableSpec("plain", "running"))
	s.start()

	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStagePending,
		`virtual machine "vm1": explicit host placement: host NUMA topology is not available yet`)
	s.assertStatus("vm2", "unknown", hypervisorhelpers.VirtualMachineStagePending,
		`virtual machine "vm2": explicit host placement: host NUMA topology is not available yet`)
	s.assertStatus("plain", "unknown", hypervisorhelpers.VirtualMachineStageUnknown, "domain has not been observed")

	s.Create(newNUMATopology())
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStageError,
		`virtual machine "vm1": invalid host placement: vCPU 0 pin names offline host CPU 3`)
	// Remote memory on a CPUless node is a valid selection.
	s.assertStatus("vm2", "unknown", hypervisorhelpers.VirtualMachineStageUnknown, "domain has not been observed")

	ctest.UpdateWithConflicts(s, hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, name),
		func(spec *hypervisor.VirtualMachineSpec) error {
			spec.TypedSpec().Memory.NUMA.Nodes = "3"
			spec.TypedSpec().CPU.Pins[0].CPUs = "6"

			return nil
		})
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStageError,
		`virtual machine "vm1": invalid host placement: memory nodeset names host NUMA node 3 without memory`)

	ctest.UpdateWithConflicts(s, hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, name),
		func(spec *hypervisor.VirtualMachineSpec) error {
			spec.TypedSpec().Memory.NUMA.Nodes = "2"

			return nil
		})
	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStageReady, "")
}
