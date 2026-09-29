// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

type openFailure struct{ err error }

type VirtualMachineStatusSuite struct {
	ctest.DefaultSuite
	client  *domainClient
	openErr atomic.Pointer[openFailure]
	opens   atomic.Int32
	events  chan struct{}
}

func (s *VirtualMachineStatusSuite) SetupTest() {
	s.DefaultSuite.SetupTest()
	s.openErr.Store(nil)
	s.opens.Store(0)
	s.events = make(chan struct{}, 1)
	s.client = &domainClient{
		domains:         make(map[string]libvirtdomain.Domain),
		texts:           make(map[string]string),
		starts:          make(map[string]int),
		changed:         make(chan struct{}, 1),
		attempted:       make(chan struct{}, 1),
		attemptedRemove: make(chan struct{}, 1),
		listed:          make(chan struct{}, 1),
	}

	machine := hardware.NewSystemInformation(hardware.SystemInformationID)
	machine.TypedSpec().UUID = machineUUID
	s.Create(machine)
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

func (s *VirtualMachineStatusSuite) startObserver() {
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainStatusController{
		Open: s.open, Watch: func(context.Context) (<-chan struct{}, error) { return s.events, nil },
	}))
}

func (s *VirtualMachineStatusSuite) TestLifecycleEventRefreshesUnmanagedDomainWithoutPolling() {
	domain := libvirtdomain.Domain{Name: "event-only", UUID: uuid.New()}

	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainStatusController{
		Open: s.open, Watch: func(context.Context) (<-chan struct{}, error) { return s.events, nil },
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

func (s *VirtualMachineStatusSuite) TestLifecycleEventRemovesDisappearedDomainWithoutPolling() {
	domain := libvirtdomain.Domain{Name: "event-only", UUID: uuid.New()}
	s.client.domains[domain.Name] = domain
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainStatusController{
		Open: s.open, Watch: func(context.Context) (<-chan struct{}, error) { return s.events, nil },
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
		Open: s.open,
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
			status.TypedSpec().PowerState == hypervisor.VirtualMachinePowerStateRunning &&
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

func (s *VirtualMachineStatusSuite) TestAbsentDomainIsNotReadyDuringFailedScan() {
	s.openErr.Store(&openFailure{err: errors.New("daemon unavailable")})

	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm1")
	spec.TypedSpec().PowerState = "stopped"
	s.Create(spec)
	s.start()
	s.assertStatus("vm1", "unknown", hypervisor.VirtualMachineStageUnknown, "domain has not been observed")
}

func (s *VirtualMachineStatusSuite) assertStatus(name, state string, stage hypervisor.VirtualMachineStage, errorText string) {
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
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm1")
	spec.TypedSpec().PowerState = "stopped"
	s.Create(spec)
	s.start()
	s.assertStatus("vm1", "unknown", hypervisor.VirtualMachineStageUnknown, "domain has not been observed")

	updated, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](s.Ctx(), s.State(), "vm1")
	s.Require().NoError(err)

	updated.TypedSpec().PowerState = "running"
	s.Update(updated)
	s.assertStatus("vm1", "unknown", hypervisor.VirtualMachineStageUnknown, "domain has not been observed")

	domainSpec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "vm1")
	domainSpec.TypedSpec().PowerState = "running"
	domainSpec.TypedSpec().DomainXML = `<domain><name>vm1</name></domain>`
	s.Create(domainSpec)
	s.assertStatus("vm1", "unknown", hypervisor.VirtualMachineStageUnknown, "domain has not been observed")

	s.client.mu.Lock()
	s.client.domains["vm1"] = libvirtdomain.Domain{Name: "vm1", UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), "vm1")}
	s.client.mu.Unlock()

	s.events <- struct{}{}

	s.assertStatus("vm1", "running", hypervisor.VirtualMachineStageReady, "")

	second := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm2")
	second.TypedSpec().PowerState = "stopped"
	s.Create(second)
	s.assertStatus("vm2", "unknown", hypervisor.VirtualMachineStageUnknown, "domain has not been observed")
	s.assertStatus("vm1", "running", hypervisor.VirtualMachineStageReady, "")
}

func (s *VirtualMachineStatusSuite) TestForeignDomainIsError() {
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm1")
	spec.TypedSpec().PowerState = "running"
	s.Create(spec)
	s.client.domains["vm1"] = libvirtdomain.Domain{Name: "vm1", UUID: uuid.New()}
	s.start()
	s.assertStatus("vm1", "unknown", hypervisor.VirtualMachineStageError, "domain name is occupied by another VM")
}

func (s *VirtualMachineStatusSuite) TestUnsupportedPowerStateIsError() {
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm1")
	spec.TypedSpec().PowerState = "suspended"
	s.Create(spec)
	s.start()
	s.assertStatus("vm1", "unknown", hypervisor.VirtualMachineStageError, `unsupported power state "suspended"`)
}

func (s *VirtualMachineStatusSuite) TestDaemonUnavailable() {
	s.openErr.Store(&openFailure{err: errors.New("daemon unavailable")})

	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm1")
	spec.TypedSpec().PowerState = "running"
	s.Create(spec)
	s.start()
	s.assertStatus("vm1", "unknown", hypervisor.VirtualMachineStageUnknown, "domain has not been observed")
}

func (s *VirtualMachineStatusSuite) TestLibvirtOutageKeepsLastObservationUntilSuccessfulScan() {
	name := "vm1"
	s.client.domains[name] = libvirtdomain.Domain{Name: name, UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), name)}

	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, name)
	spec.TypedSpec().PowerState = "running"
	s.Create(spec)

	resume := make(chan struct{})

	var subscriptions atomic.Int32

	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainStatusController{
		Open: s.open,
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
	s.assertStatus(name, "running", hypervisor.VirtualMachineStageReady, "")

	s.openErr.Store(&openFailure{err: errors.New("daemon unavailable")})
	close(s.events)
	s.Require().Eventually(func() bool { return subscriptions.Load() >= 3 }, 5*time.Second, 10*time.Millisecond)

	status, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainStatus](s.Ctx(), s.State(), name)
	s.Require().NoError(err)
	s.Require().Equal(libvirtdomain.UUID(uuid.MustParse(machineUUID), name).String(), status.TypedSpec().UUID)
	s.assertStatus(name, "running", hypervisor.VirtualMachineStageReady, "")

	s.client.mu.Lock()
	delete(s.client.domains, name)
	s.client.mu.Unlock()
	s.openErr.Store(nil)
	close(resume)

	s.Require().Eventually(func() bool {
		_, getErr := safe.StateGetByID[*hypervisor.VirtualMachineDomainStatus](s.Ctx(), s.State(), name)

		return getErr != nil
	}, 10*time.Second, 10*time.Millisecond)
	s.assertStatus(name, "unknown", hypervisor.VirtualMachineStageUnknown, "domain has not been observed")
}

func (s *VirtualMachineStatusSuite) TestStatusRemovedWithSpec() {
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm1")
	spec.TypedSpec().PowerState = "stopped"
	s.Create(spec)
	s.start()
	s.assertStatus("vm1", "unknown", hypervisor.VirtualMachineStageUnknown, "domain has not been observed")
	s.Destroy(spec)
	s.Require().Eventually(func() bool {
		_, err := safe.StateGetByID[*hypervisor.VirtualMachineStatus](s.Ctx(), s.State(), "vm1")

		return err != nil
	}, 5*time.Second, 10*time.Millisecond)
}
