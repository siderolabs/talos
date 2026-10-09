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
	powerState hypervisor.VirtualMachinePowerState,
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
			domainStatus.TypedSpec().PowerState == hypervisor.VirtualMachinePowerStateUnknown &&
			domainStatus.TypedSpec().Error != "" &&
			status.TypedSpec().PowerState == hypervisor.VirtualMachinePowerStateUnknown &&
			status.TypedSpec().Stage == hypervisor.VirtualMachineStageError && status.TypedSpec().Error != ""
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
	s.assertDomainObservation(domain.Name, hypervisor.VirtualMachinePowerStateRunning, "")
	s.Require().EqualValues(1, subscriptions.Load())
	s.Require().EqualValues(1, s.opens.Load())

	s.Destroy(starting)
	s.assertWatchCanceled(watchCanceled)
	s.assertClientCloseCount(1)
	s.assertDomainObservation(domain.Name, hypervisor.VirtualMachinePowerStateUnknown, "virtqemud service is not ready")

	s.Create(newReadyVirtqemudService())
	s.assertDomainObservation(domain.Name, hypervisor.VirtualMachinePowerStateRunning, "")
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

	spec := newRenderableSpec("vm1", "stopped")
	s.Create(spec)
	s.start()
	s.assertStatus("vm1", "unknown", hypervisor.VirtualMachineStageUnknown, "domain has not been observed")
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
	spec := newRenderableSpec("vm1", "stopped")
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

	second := newRenderableSpec("vm2", "stopped")
	s.Create(second)
	s.assertStatus("vm2", "unknown", hypervisor.VirtualMachineStageUnknown, "domain has not been observed")
	s.assertStatus("vm1", "running", hypervisor.VirtualMachineStageReady, "")
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

	s.assertStatus(name, "running", hypervisor.VirtualMachineStagePending,
		`virtual machine "vm1": interface "net0": host link not found: "uplink"`)

	link := network.NewLinkStatus(network.NamespaceName, "macvlan0")
	link.TypedSpec().Type = nethelpers.LinkEther
	link.TypedSpec().Alias = "uplink"
	s.Create(link)

	s.assertStatus(name, "running", hypervisor.VirtualMachineStageReady, "")
}

// The same obstacle, before the domain was ever defined: the missing link is the reason, not the
// generic absence of an observation.
func (s *VirtualMachineStatusSuite) TestUnresolvedLinkExplainsUndefinedDomain() {
	spec := newRenderableSpec("vm1", "running")
	spec.TypedSpec().Interfaces = []hypervisor.VirtualMachineInterfaceSpec{{Name: "net0", Link: "uplink"}}
	s.Create(spec)
	s.start()

	s.assertStatus("vm1", "unknown", hypervisor.VirtualMachineStagePending,
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

	s.assertStatus(name, "running", hypervisor.VirtualMachineStagePending,
		`virtual machine "vm1": disk "install" is not ready: no disk status yet`)

	status := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, hypervisor.VirtualMachineDiskStatusID(name, disk))
	status.TypedSpec().VirtualMachine = name
	status.TypedSpec().Name = "install"
	status.TypedSpec().SourcePath = "/var/lib/libvirt/images/vm1-install.qcow2"
	status.TypedSpec().Format = "qcow2"
	status.TypedSpec().Phase = hypervisor.VirtualMachineDiskPhaseReady
	status.TypedSpec().Image = hypervisor.VirtualMachineDiskFromImageSpec{Library: "vm-images", File: "talos.iso"}
	s.Create(status)

	s.assertStatus(name, "running", hypervisor.VirtualMachineStageReady, "")
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

	s.assertStatus("vm1", "unknown", hypervisor.VirtualMachineStageError,
		`virtual machine "vm1": interface "net0": host link is not an Ethernet link: "lo"`)
}

// A disk nothing on the host will ever make attachable is an error rather than something to wait
// for: only a change to the configuration helps. A materialized disk whose format isn't qcow2 is
// the case here -- both linked and copy modes write a qcow2 file.
func (s *VirtualMachineStatusSuite) TestUnsupportedDiskIsError() {
	spec := newRenderableSpec("vm1", "running")
	spec.TypedSpec().Disks = []hypervisor.VirtualMachineDiskSpec{{
		Name:   "system",
		Pool:   "pool1",
		Size:   20 << 30,
		Format: hypervisorhelpers.VirtualMachineDiskFormatRaw.String(),
		Type:   hypervisorhelpers.VirtualMachineDiskTypeDisk.String(),
		Provision: hypervisor.VirtualMachineDiskProvisionSpec{
			FromImage: &hypervisor.VirtualMachineDiskFromImageSpec{Library: "images", File: "talos.qcow2"},
		},
	}}
	s.Create(spec)
	s.start()

	s.assertStatus("vm1", "unknown", hypervisor.VirtualMachineStageError,
		`virtual machine "vm1": disk "system": unsupported disk: a materialized disk must be qcow2, got "raw"`)
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

	s.assertStatus("vm1", "unknown", hypervisor.VirtualMachineStagePending,
		`virtual machine "vm1": disk "data" is not ready: no disk status yet`)
}

func (s *VirtualMachineStatusSuite) TestForeignDomainIsError() {
	spec := newRenderableSpec("vm1", "running")
	s.Create(spec)
	s.client.domains["vm1"] = libvirtdomain.Domain{Name: "vm1", UUID: uuid.New()}
	s.start()
	s.assertStatus("vm1", "unknown", hypervisor.VirtualMachineStageError, "domain name is occupied by another VM")
}

func (s *VirtualMachineStatusSuite) TestUnsupportedPowerStateIsError() {
	spec := newRenderableSpec("vm1", "suspended")
	s.Create(spec)
	s.start()
	s.assertStatus("vm1", "unknown", hypervisor.VirtualMachineStageError, `unsupported power state "suspended"`)
}

func (s *VirtualMachineStatusSuite) TestDaemonUnavailable() {
	s.openErr.Store(&openFailure{err: errors.New("daemon unavailable")})

	spec := newRenderableSpec("vm1", "running")
	s.Create(spec)
	s.start()
	s.assertStatus("vm1", "unknown", hypervisor.VirtualMachineStageUnknown, "domain has not been observed")
}

func (s *VirtualMachineStatusSuite) TestLibvirtOutageMarksObservationUnknownUntilSuccessfulScan() {
	name := "vm1"
	s.client.domains[name] = libvirtdomain.Domain{Name: name, UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), name)}

	spec := newRenderableSpec(name, "running")
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
	s.assertStatus(name, "unknown", hypervisor.VirtualMachineStageUnknown, "domain has not been observed")
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

	s.assertStatus(name, "running", hypervisor.VirtualMachineStagePending,
		`virtual machine "seeded": cloud-init seed is not ready`)

	seed := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, name)
	*seed.TypedSpec() = hypervisor.CloudInitSpecSpec{
		Library:  vm.TypedSpec().CloudInit.Library,
		MetaData: vm.TypedSpec().CloudInit.MetaData,
		UserData: vm.TypedSpec().CloudInit.UserData,
	}
	s.Create(seed)
	s.assertStatus(name, "running", hypervisor.VirtualMachineStagePending,
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
	s.assertStatus(name, "running", hypervisor.VirtualMachineStagePending,
		`virtual machine "seeded": cloud-init seed is not ready`)

	ctest.UpdateWithConflicts(s, asset, func(current *hypervisor.CloudInitStatus) error {
		current.TypedSpec().Error = ""

		return nil
	})
	s.assertStatus(name, "running", hypervisor.VirtualMachineStagePending,
		`virtual machine "seeded": cloud-init seed is not ready`)

	ctest.UpdateWithConflicts(s, asset, func(current *hypervisor.CloudInitStatus) error {
		current.TypedSpec().Phase = hypervisor.CloudInitPhaseReady

		return nil
	})
	s.assertStatus(name, "running", hypervisor.VirtualMachineStagePending,
		`virtual machine "seeded": cloud-init seed is not ready`)

	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "images")
	*library.TypedSpec() = hypervisor.ContentLibraryStatusSpec{
		Path:     "/images",
		VolumeID: "volume-a",
		Phase:    hypervisor.ContentLibraryPhaseReady,
	}
	s.Create(library)
	s.assertStatus(name, "running", hypervisor.VirtualMachineStageReady, "")

	ctest.UpdateWithConflicts(s, seed, func(current *hypervisor.CloudInitSpec) error {
		current.TypedSpec().UserData = "different seed"

		return nil
	})
	s.assertStatus(name, "running", hypervisor.VirtualMachineStagePending,
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
	s.assertStatus(name, "running", hypervisor.VirtualMachineStageReady, "")

	ctest.UpdateWithConflicts(s, library, func(current *hypervisor.ContentLibraryStatus) error {
		current.TypedSpec().Phase = hypervisor.ContentLibraryPhaseNotReady

		return nil
	})
	s.assertStatus(name, "running", hypervisor.VirtualMachineStagePending,
		`virtual machine "seeded": cloud-init seed is not ready`)

	ctest.UpdateWithConflicts(s, library, func(current *hypervisor.ContentLibraryStatus) error {
		current.TypedSpec().Phase = hypervisor.ContentLibraryPhaseReady

		return nil
	})
	s.assertStatus(name, "running", hypervisor.VirtualMachineStageReady, "")

	s.Destroy(asset)
	s.assertStatus(name, "running", hypervisor.VirtualMachineStagePending,
		`virtual machine "seeded": cloud-init seed is not ready`)

	republished := hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, hypervisor.CloudInitStatusID(name, *seed.TypedSpec()))
	*republished.TypedSpec() = *asset.TypedSpec()
	republished.TypedSpec().Phase = hypervisor.CloudInitPhaseReady
	republished.TypedSpec().Error = ""
	republished.TypedSpec().ObservedGeneration = projected.Metadata().Version().String()
	s.Create(republished)
	s.assertStatus(name, "running", hypervisor.VirtualMachineStageReady, "")

	ctest.UpdateWithConflicts(s, republished, func(current *hypervisor.CloudInitStatus) error {
		current.TypedSpec().Phase = hypervisor.CloudInitPhaseNotReady
		current.TypedSpec().Error = "seed generation failed"

		return nil
	})
	s.assertStatus(name, "running", hypervisor.VirtualMachineStagePending,
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
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainSpecController{}))
	s.start()
	s.assertStatus(name, "running", hypervisor.VirtualMachineStageReady, "")

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
	s.assertStatus(name, "running", hypervisor.VirtualMachineStagePending,
		`virtual machine "seeded-old-domain": cloud-init seed is not ready`)
}

func (s *VirtualMachineStatusSuite) TestStatusRemovedWithSpec() {
	spec := newRenderableSpec("vm1", "stopped")
	s.Create(spec)
	s.start()
	s.assertStatus("vm1", "unknown", hypervisor.VirtualMachineStageUnknown, "domain has not been observed")
	s.Destroy(spec)
	s.Require().Eventually(func() bool {
		_, err := safe.StateGetByID[*hypervisor.VirtualMachineStatus](s.Ctx(), s.State(), "vm1")

		return err != nil
	}, 5*time.Second, 10*time.Millisecond)
}
