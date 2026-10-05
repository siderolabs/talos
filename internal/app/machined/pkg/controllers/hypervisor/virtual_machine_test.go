// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/google/uuid"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

const (
	machineUUID        = "c737f778-82a1-48dd-990b-67901031bcc5"
	virtqemudServiceID = "ext-virtqemud"
	// diskStatusVM is the virtual machine the disk-holding cases work on.
	diskStatusVM = "vm"
	// diskStatusLibrary is the content library those cases resolve their images against.
	diskStatusLibrary = "vm-images"
)

type domainClient struct {
	mu              sync.Mutex
	domains         map[string]libvirtdomain.Domain
	texts           map[string]string
	starts          map[string]int
	opens           int
	closes          int
	removeErr       error
	startErr        error
	listErr         error
	changed         chan struct{}
	attempted       chan struct{}
	attemptedRemove chan struct{}
	listed          chan struct{}
}

func (c *domainClient) open(context.Context) (libvirtdomain.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.opens++

	return c, nil
}

func (c *domainClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closes++
}

func (c *domainClient) Domains() ([]libvirtdomain.Domain, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case c.listed <- struct{}{}:
	default:
	}

	if c.listErr != nil {
		return nil, c.listErr
	}

	return slices.Collect(maps.Values(c.domains)), nil
}

func (c *domainClient) Active(domain libvirtdomain.Domain) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	existing, present := c.domains[domain.Name]

	return present && existing.UUID == domain.UUID, nil
}

func (c *domainClient) Info(domain libvirtdomain.Domain) (libvirtdomain.Info, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.domains[domain.Name]; !ok || existing.UUID != domain.UUID {
		return libvirtdomain.Info{}, fmt.Errorf("domain %q disappeared", domain.Name)
	}

	return libvirtdomain.Info{State: 1, MaxMemoryKiB: 1048576, MemoryKiB: 524288, VCPUs: 2}, nil
}

func (c *domainClient) Define(domain libvirtdomain.Domain, text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case c.attempted <- struct{}{}:
	default:
	}

	if existing, ok := c.domains[domain.Name]; ok && existing.UUID != domain.UUID {
		return fmt.Errorf("domain %q is not owned", domain.Name)
	}

	c.domains[domain.Name] = domain

	c.texts[domain.Name] = text

	select {
	case c.changed <- struct{}{}:
	default:
	}

	return nil
}

func (c *domainClient) Start(domain libvirtdomain.Domain, text string) error {
	c.mu.Lock()

	if c.startErr != nil {
		err := c.startErr
		c.mu.Unlock()

		return err
	}

	_, exists := c.domains[domain.Name]

	unchanged := exists && c.texts[domain.Name] == text
	if !unchanged {
		c.starts[domain.Name]++
	}
	c.mu.Unlock()

	if unchanged {
		return nil
	}

	return c.Define(domain, text)
}

func (c *domainClient) Remove(domain libvirtdomain.Domain) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case c.attemptedRemove <- struct{}{}:
	default:
	}

	if c.removeErr != nil {
		return c.removeErr
	}

	if existing, ok := c.domains[domain.Name]; ok && existing.UUID != domain.UUID {
		return fmt.Errorf("domain %q is not owned", domain.Name)
	}

	delete(c.domains, domain.Name)
	delete(c.texts, domain.Name)

	select {
	case c.changed <- struct{}{}:
	default:
	}

	return nil
}

type VirtualMachineDomainSuite struct {
	ctest.DefaultSuite
	client *domainClient
}

func (s *VirtualMachineDomainSuite) SetupTest() {
	s.DefaultSuite.SetupTest()
	s.client = &domainClient{
		domains:         make(map[string]libvirtdomain.Domain),
		texts:           make(map[string]string),
		starts:          make(map[string]int),
		changed:         make(chan struct{}, 1),
		attempted:       make(chan struct{}, 1),
		attemptedRemove: make(chan struct{}, 1),
		listed:          make(chan struct{}, 1),
	}

	system := hardware.NewSystemInformation(hardware.SystemInformationID)
	system.TypedSpec().UUID = machineUUID
	s.Create(system)
	s.Create(newReadyVirtqemudService())
	s.Create(newVirtualMachineLogMount())
}

func newVirtualMachineLogMount() *block.VolumeMountStatus {
	mount := block.NewVolumeMountStatus(block.NamespaceName, "runtime.LogPersistenceController-"+constants.LogMountPoint)
	mount.TypedSpec().VolumeID = constants.LogVolumeID
	mount.TypedSpec().Target = constants.LogMountPoint

	return mount
}

func newReadyVirtqemudService() *v1alpha1.Service {
	service := v1alpha1.NewService(virtqemudServiceID)
	service.TypedSpec().Running = true
	service.TypedSpec().Unknown = true

	return service
}

func (s *VirtualMachineDomainSuite) start() {
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineController{Open: s.client.open}))
}

func (s *VirtualMachineDomainSuite) assertDomain(name, text string, present bool) {
	s.Require().Eventually(func() bool {
		s.client.mu.Lock()
		defer s.client.mu.Unlock()

		domain, ok := s.client.domains[name]
		if !present {
			return !ok
		}

		return ok && domain.UUID == libvirtdomain.UUID(uuid.MustParse(machineUUID), name) && s.client.texts[name] == text
	}, 5*time.Second, 10*time.Millisecond)
}

func (s *VirtualMachineDomainSuite) assertFinalizer(name string, present bool) {
	s.Require().Eventually(func() bool {
		spec, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), name)

		return err == nil && spec.Metadata().Finalizers().Has("hypervisor.VirtualMachineController") == present
	}, 5*time.Second, 10*time.Millisecond)
}

func (s *VirtualMachineDomainSuite) TestWaitsForLogVolume() {
	s.Destroy(newVirtualMachineLogMount())
	s.start()

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "log-gated")
	spec.TypedSpec().DomainXML = `<domain><name>log-gated</name><devices><serial type="pty"/></devices></domain>`
	spec.TypedSpec().PowerState = "running"
	s.Create(spec)

	s.Require().Never(func() bool {
		s.client.mu.Lock()
		defer s.client.mu.Unlock()

		return s.client.starts["log-gated"] != 0
	}, 100*time.Millisecond, 10*time.Millisecond)

	mount := newVirtualMachineLogMount()
	s.Create(mount)
	s.assertDomain("log-gated", spec.TypedSpec().DomainXML, true)

	ready, err := s.State().Teardown(s.Ctx(), spec.Metadata())
	s.Require().NoError(err)
	s.Require().False(ready)
	s.assertDomain("log-gated", "", false)
	s.assertFinalizer("log-gated", false)
	s.Destroy(spec)

	current, err := safe.StateGetByID[*block.VolumeMountStatus](s.Ctx(), s.State(), mount.Metadata().ID())
	s.Require().NoError(err)
	s.Require().False(current.Metadata().Finalizers().Has("hypervisor.VirtualMachineController"), "LOG lifetime remains owned by log persistence")
}

func (s *VirtualMachineDomainSuite) TestLogMountTeardownPreservesServiceShutdownOrdering() {
	s.start()

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "log-teardown")
	spec.TypedSpec().DomainXML = `<domain><name>log-teardown</name><devices><serial type="pty"/></devices></domain>`
	spec.TypedSpec().PowerState = "running"
	s.Create(spec)
	s.assertDomain("log-teardown", spec.TypedSpec().DomainXML, true)
	s.assertFinalizer("log-teardown", true)

	mount := newVirtualMachineLogMount()
	s.Require().NoError(s.State().AddFinalizer(s.Ctx(), mount.Metadata(), "runtime.LogPersistenceController"))
	ready, err := s.State().Teardown(s.Ctx(), mount.Metadata())
	s.Require().NoError(err)
	s.Require().False(ready)

	second := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "waiting")
	second.TypedSpec().DomainXML = `<domain><name>waiting</name><devices><serial type="pty"/></devices></domain>`
	second.TypedSpec().PowerState = "running"
	s.Create(second)

	s.Require().Never(func() bool {
		s.client.mu.Lock()
		defer s.client.mu.Unlock()

		_, running := s.client.domains["log-teardown"]

		return s.client.starts["waiting"] != 0 || !running
	}, 100*time.Millisecond, 10*time.Millisecond)

	current, err := safe.StateGetByID[*block.VolumeMountStatus](s.Ctx(), s.State(), mount.Metadata().ID())
	s.Require().NoError(err)
	s.Require().True(current.Metadata().Finalizers().Has("runtime.LogPersistenceController"))

	// Cleanup of configured stopped domains must still work during mount teardown.
	ctest.UpdateWithConflicts(s, spec, func(current *hypervisor.VirtualMachineDomainSpec) error {
		current.TypedSpec().PowerState = "stopped"

		return nil
	})
	s.assertDomain("log-teardown", "", false)
	s.assertFinalizer("log-teardown", false)
}

func (s *VirtualMachineDomainSuite) TestRejectsUnusableLogMount() {
	s.start()

	for _, test := range []struct {
		name   string
		mutate func(*block.VolumeMountStatus)
	}{
		{
			name: "wrong-volume",
			mutate: func(mount *block.VolumeMountStatus) {
				mount.TypedSpec().VolumeID = "other"
			},
		},
		{
			name: "wrong-target",
			mutate: func(mount *block.VolumeMountStatus) {
				mount.TypedSpec().Target = "/other"
			},
		},
		{
			name: "read-only",
			mutate: func(mount *block.VolumeMountStatus) {
				mount.TypedSpec().ReadOnly = true
			},
		},
		{
			name: "detached",
			mutate: func(mount *block.VolumeMountStatus) {
				mount.TypedSpec().Detached = true
			},
		},
	} {
		s.Run(test.name, func() {
			mount := newVirtualMachineLogMount()
			ctest.UpdateWithConflicts(s, mount, func(current *block.VolumeMountStatus) error {
				*current.TypedSpec() = *mount.TypedSpec()
				test.mutate(current)

				return nil
			})

			name := "unusable-log-" + test.name
			spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, name)
			spec.TypedSpec().DomainXML = `<domain><name>` + name + `</name><devices><serial type="pty"/></devices></domain>`
			spec.TypedSpec().PowerState = "running"
			s.Create(spec)
			s.Require().Never(func() bool {
				s.client.mu.Lock()
				defer s.client.mu.Unlock()

				return s.client.starts[name] != 0
			}, 100*time.Millisecond, 10*time.Millisecond)
		})
	}
}

func (s *VirtualMachineDomainSuite) TestCreateUpdateRemove() {
	s.start()

	first := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "first")
	first.TypedSpec().DomainXML = `<domain><name>first</name><vcpu>1</vcpu></domain>`
	first.TypedSpec().PowerState = "running"
	s.Create(first)
	s.assertDomain("first", first.TypedSpec().DomainXML, true)
	s.assertFinalizer("first", true)
	s.client.mu.Lock()
	starts := s.client.starts["first"]
	s.client.mu.Unlock()
	s.Require().Equal(1, starts, "running guest must be started")

	second := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "second")
	second.TypedSpec().DomainXML = `<domain><name>second</name><vcpu>1</vcpu></domain>`
	second.TypedSpec().PowerState = "running"
	s.Create(second)
	s.assertDomain("second", second.TypedSpec().DomainXML, true)

	updated, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), "first")
	s.Require().NoError(err)

	updated.TypedSpec().DomainXML = `<domain><name>first</name><vcpu>2</vcpu></domain>`
	s.Update(updated)
	s.assertDomain("first", updated.TypedSpec().DomainXML, true)

	ready, err := s.State().Teardown(s.Ctx(), first.Metadata())
	s.Require().NoError(err)
	s.Require().False(ready, "the finalizer must prevent immediate deletion")
	s.assertDomain("first", "", false)
	s.assertFinalizer("first", false)
	s.Destroy(first)
	s.assertDomain("second", second.TypedSpec().DomainXML, true)
}

func (s *VirtualMachineDomainSuite) TestServiceReadinessGatesReconciliationAndCleanup() {
	service, err := safe.StateGetByID[*v1alpha1.Service](s.Ctx(), s.State(), virtqemudServiceID)
	s.Require().NoError(err)
	s.Destroy(service)

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "gated")
	spec.TypedSpec().DomainXML = `<domain><name>gated</name><vcpu>1</vcpu></domain>`
	spec.TypedSpec().PowerState = "running"
	s.Create(spec)
	s.start()

	s.Require().Never(func() bool {
		s.client.mu.Lock()
		defer s.client.mu.Unlock()

		return s.client.opens != 0
	}, 100*time.Millisecond, 10*time.Millisecond)

	starting := v1alpha1.NewService(virtqemudServiceID)
	starting.TypedSpec().Unknown = true
	s.Create(starting)
	s.Require().Never(func() bool {
		s.client.mu.Lock()
		defer s.client.mu.Unlock()

		return s.client.opens != 0
	}, 100*time.Millisecond, 10*time.Millisecond)

	ctest.UpdateWithConflicts(s, starting, func(resource *v1alpha1.Service) error {
		resource.TypedSpec().Running = true

		return nil
	})
	s.assertDomain("gated", spec.TypedSpec().DomainXML, true)
	s.assertFinalizer("gated", true)

	ready, err := s.State().Teardown(s.Ctx(), starting.Metadata())
	s.Require().NoError(err)
	s.Require().True(ready)
	s.Destroy(starting)

	updated := ctest.UpdateWithConflicts(s, spec, func(resource *hypervisor.VirtualMachineDomainSpec) error {
		resource.TypedSpec().DomainXML = `<domain><name>gated</name><vcpu>2</vcpu></domain>`

		return nil
	})

	s.client.mu.Lock()
	opensWhileReady := s.client.opens
	s.client.mu.Unlock()

	s.Require().Never(func() bool {
		s.client.mu.Lock()
		defer s.client.mu.Unlock()

		return s.client.opens != opensWhileReady || s.client.texts["gated"] == updated.TypedSpec().DomainXML
	}, 100*time.Millisecond, 10*time.Millisecond)
	s.assertFinalizer("gated", true)

	teardownReady, err := s.State().Teardown(s.Ctx(), updated.Metadata())
	s.Require().NoError(err)
	s.Require().False(teardownReady)
	s.assertFinalizer("gated", true)
	s.assertDomain("gated", spec.TypedSpec().DomainXML, true)

	s.Create(newReadyVirtqemudService())
	s.assertDomain("gated", "", false)
	s.assertFinalizer("gated", false)
}

func (s *VirtualMachineDomainSuite) TestUnclaimedDomainsAreNeverRemoved() {
	foreign := libvirtdomain.Domain{Name: "external", UUID: uuid.New()}
	unclaimed := libvirtdomain.Domain{Name: "unclaimed", UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), "unclaimed")}
	s.client.domains[foreign.Name] = foreign
	s.client.domains[unclaimed.Name] = unclaimed
	s.start()

	select {
	case <-s.client.attemptedRemove:
		s.FailNow("controller attempted to remove an unclaimed domain")
	case <-time.After(100 * time.Millisecond):
	}

	s.client.mu.Lock()
	defer s.client.mu.Unlock()

	s.Require().Equal(foreign, s.client.domains[foreign.Name])
	s.Require().Equal(unclaimed, s.client.domains[unclaimed.Name])
}

func (s *VirtualMachineDomainSuite) TestMatchingUUIDDoesNotGrantOwnership() {
	name := "matching-uuid"
	unclaimed := libvirtdomain.Domain{Name: name, UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), name)}
	s.client.domains[name] = unclaimed
	s.client.texts[name] = "unclaimed XML"

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, name)
	spec.TypedSpec().DomainXML = `<domain><name>matching-uuid</name></domain>`
	spec.TypedSpec().PowerState = "running"
	s.Create(spec)
	s.start()

	select {
	case <-s.client.listed:
	case <-s.Ctx().Done():
		s.FailNow("controller did not reconcile domains")
	}

	s.assertFinalizer(name, false)
	s.client.mu.Lock()
	defer s.client.mu.Unlock()

	s.Require().Equal("unclaimed XML", s.client.texts[name])
}

func (s *VirtualMachineDomainSuite) TestForeignNameCollision() {
	foreign := libvirtdomain.Domain{Name: "first", UUID: uuid.New()}
	s.client.domains[foreign.Name] = foreign
	s.start()

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, foreign.Name)
	spec.TypedSpec().DomainXML = `<domain><name>first</name></domain>`
	spec.TypedSpec().PowerState = "running"
	s.Create(spec)

	select {
	case <-s.client.listed:
	case <-s.Ctx().Done():
		s.FailNow("controller did not inspect the colliding domain")
	}

	s.client.mu.Lock()
	actual := s.client.domains[foreign.Name]
	s.client.mu.Unlock()

	s.Require().Equal(foreign, actual)

	ready, err := s.State().Teardown(s.Ctx(), spec.Metadata())
	s.Require().NoError(err)
	s.Require().True(ready)
	s.assertFinalizer(foreign.Name, false)
	s.Destroy(spec)
}

func (s *VirtualMachineDomainSuite) TestStoppedNeverClaimsOrStartsDomain() {
	foreign := libvirtdomain.Domain{Name: "foreign-stopped", UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), "foreign-stopped")}
	s.client.domains[foreign.Name] = foreign

	for _, name := range []string{"new-stopped", foreign.Name} {
		spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, name)
		spec.TypedSpec().DomainXML = `<domain><name>` + name + `</name></domain>`
		spec.TypedSpec().PowerState = "stopped"
		s.Create(spec)
	}

	s.start()

	select {
	case <-s.client.listed:
	case <-s.Ctx().Done():
		s.FailNow("controller did not inspect stopped specs")
	}

	s.assertFinalizer("new-stopped", false)
	s.assertFinalizer(foreign.Name, false)

	s.client.mu.Lock()
	starts := len(s.client.starts)
	actual := s.client.domains[foreign.Name]
	_, exists := s.client.domains["new-stopped"]
	s.client.mu.Unlock()

	s.Require().Zero(starts)
	s.Require().Equal(foreign, actual)
	s.Require().False(exists)
}

func (s *VirtualMachineDomainSuite) TestStoppedRemovesClaimedDomain() {
	s.start()

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "first")
	spec.TypedSpec().DomainXML = `<domain><name>first</name></domain>`
	spec.TypedSpec().PowerState = "running"
	s.Create(spec)
	s.assertDomain("first", spec.TypedSpec().DomainXML, true)
	s.assertFinalizer("first", true)

	ctest.UpdateWithConflicts(s, spec, func(res *hypervisor.VirtualMachineDomainSpec) error {
		res.TypedSpec().PowerState = "stopped"

		return nil
	})
	s.assertDomain("first", "", false)
	s.assertFinalizer("first", false)
}

func (s *VirtualMachineDomainSuite) TestUnexpectedShutdownRestartsRunningDomain() {
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineController{
		Open: s.client.open,
	}))

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "restart")
	spec.TypedSpec().DomainXML = `<domain><name>restart</name></domain>`
	spec.TypedSpec().PowerState = "running"
	s.Create(spec)
	s.assertDomain("restart", spec.TypedSpec().DomainXML, true)
	s.assertFinalizer("restart", true)

	status := hypervisor.NewVirtualMachineDomainStatus(hypervisor.NamespaceName, "restart")
	s.Create(status)

	s.client.mu.Lock()
	delete(s.client.domains, "restart")
	delete(s.client.texts, "restart")
	s.client.mu.Unlock()
	s.Destroy(status)

	s.Require().Eventually(func() bool {
		s.client.mu.Lock()
		defer s.client.mu.Unlock()

		_, exists := s.client.domains["restart"]

		return exists && s.client.starts["restart"] >= 2
	}, 5*time.Second, 10*time.Millisecond, "running transient domain was not restored")
}

func (s *VirtualMachineDomainSuite) TestSuspendedDoesNotChangeRunningDomain() {
	s.start()

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "suspended")
	spec.TypedSpec().DomainXML = `<domain><name>suspended</name></domain>`
	spec.TypedSpec().PowerState = "running"
	s.Create(spec)
	s.assertDomain("suspended", spec.TypedSpec().DomainXML, true)
	s.assertFinalizer("suspended", true)

	select {
	case <-s.client.listed:
	default:
	}

	ctest.UpdateWithConflicts(s, spec, func(res *hypervisor.VirtualMachineDomainSpec) error {
		res.TypedSpec().PowerState = "suspended"

		return nil
	})

	select {
	case <-s.client.listed:
	case <-s.Ctx().Done():
		s.FailNow("controller did not inspect suspended domain")
	}

	s.client.mu.Lock()
	starts := s.client.starts["suspended"]
	_, exists := s.client.domains["suspended"]
	s.client.mu.Unlock()
	s.Require().True(exists)
	s.Require().Equal(1, starts)
	s.assertFinalizer("suspended", true)
}

func (s *VirtualMachineDomainSuite) TestReplacedClaimedDomainIsNotRemoved() {
	s.start()

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "replaced")
	spec.TypedSpec().DomainXML = `<domain><name>replaced</name></domain>`
	spec.TypedSpec().PowerState = "running"
	s.Create(spec)
	s.assertFinalizer("replaced", true)

	foreign := libvirtdomain.Domain{Name: "replaced", UUID: uuid.New()}

	s.client.mu.Lock()
	s.client.domains[foreign.Name] = foreign
	s.client.mu.Unlock()

	ready, err := s.State().Teardown(s.Ctx(), spec.Metadata())
	s.Require().NoError(err)
	s.Require().False(ready)

	s.Require().Eventually(func() bool {
		select {
		case <-s.client.attemptedRemove:
			return true
		default:
			return false
		}
	}, 5*time.Second, 10*time.Millisecond)

	s.client.mu.Lock()
	actual := s.client.domains[foreign.Name]
	s.client.mu.Unlock()
	s.Require().Equal(foreign, actual)
	s.assertFinalizer("replaced", true)
}

func (s *VirtualMachineDomainSuite) TestFailedRemovalKeepsSpecFinalizer() {
	s.start()

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "first")
	spec.TypedSpec().DomainXML = `<domain><name>first</name></domain>`
	spec.TypedSpec().PowerState = "running"
	s.Create(spec)
	s.assertDomain("first", spec.TypedSpec().DomainXML, true)
	s.assertFinalizer("first", true)

	s.client.mu.Lock()
	s.client.removeErr = errors.New("daemon disconnected")
	s.client.mu.Unlock()

	ready, err := s.State().Teardown(s.Ctx(), spec.Metadata())
	s.Require().NoError(err)
	s.Require().False(ready)

	select {
	case <-s.client.attemptedRemove:
	case <-s.Ctx().Done():
		s.FailNow("controller did not attempt domain cleanup")
	}

	s.assertDomain("first", spec.TypedSpec().DomainXML, true)
	s.assertFinalizer("first", true)

	s.client.mu.Lock()
	s.client.removeErr = nil
	s.client.mu.Unlock()

	s.assertDomain("first", "", false)
	s.assertFinalizer("first", false)
	s.Destroy(spec)
}

func TestVirtualMachineDomainSuite(t *testing.T) {
	suite.Run(t, new(VirtualMachineDomainSuite))
}

// newDiskStatus publishes a disk status a domain definition can name. Every case here works on one
// virtual machine, so the status always belongs to diskStatusVM.
func (s *VirtualMachineDomainSuite) newDiskStatus(id string) *hypervisor.VirtualMachineDiskStatus {
	status := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, id)
	status.TypedSpec().VirtualMachine = diskStatusVM
	status.TypedSpec().Name = "install"
	status.TypedSpec().Ready = true
	s.Create(status)

	return status
}

// newImageDiskStatus publishes a disk status resolved against a content library file.
func (s *VirtualMachineDomainSuite) newImageDiskStatus(id, sourcePath string, image hypervisor.VirtualMachineDiskFromImageSpec) *hypervisor.VirtualMachineDiskStatus {
	status := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, id)
	status.TypedSpec().VirtualMachine = diskStatusVM
	status.TypedSpec().Name = "install"
	status.TypedSpec().Ready = true
	status.TypedSpec().SourcePath = sourcePath
	status.TypedSpec().Image = image
	s.Create(status)

	return status
}

// newContentLibraryStatus publishes a ready library a disk status can name.
func (s *VirtualMachineDomainSuite) newContentLibraryStatus(path string) *hypervisor.ContentLibraryStatus {
	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, diskStatusLibrary)
	library.TypedSpec().VolumeID = "u-" + diskStatusLibrary
	library.TypedSpec().Ready = true
	library.TypedSpec().Path = path
	s.Create(library)

	return library
}

func (s *VirtualMachineDomainSuite) assertNeverStarted() {
	s.Require().Never(func() bool {
		s.client.mu.Lock()
		defer s.client.mu.Unlock()

		_, started := s.client.domains[diskStatusVM]

		return started
	}, 200*time.Millisecond, 10*time.Millisecond)
}

func (s *VirtualMachineDomainSuite) assertDiskHeld(id string, held bool) {
	s.Require().Eventually(func() bool {
		status, err := safe.StateGetByID[*hypervisor.VirtualMachineDiskStatus](s.Ctx(), s.State(), id)

		return err == nil && status.Metadata().Finalizers().Has("hypervisor.VirtualMachineController") == held
	}, 5*time.Second, 10*time.Millisecond)
}

// A domain reads its disks from the moment it starts, and goes on reading them until libvirt is
// told otherwise. The hold is taken before the definition is handed over, and the one a replaced
// definition no longer names is given back only once the replacement has actually been made.
func (s *VirtualMachineDomainSuite) TestHoldsTheDisksOfARunningDomain() {
	s.start()

	first := s.newDiskStatus("vm/install@aaaaaaaaaaaa")

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "vm")
	spec.TypedSpec().DomainXML = `<domain><name>vm</name><vcpu>1</vcpu></domain>`
	spec.TypedSpec().PowerState = "running"
	spec.TypedSpec().Disks = []string{first.Metadata().ID()}
	s.Create(spec)

	s.assertDomain("vm", spec.TypedSpec().DomainXML, true)
	s.assertDiskHeld(first.Metadata().ID(), true)

	second := s.newDiskStatus("vm/install@bbbbbbbbbbbb")

	updated := ctest.UpdateWithConflicts(s, spec, func(res *hypervisor.VirtualMachineDomainSpec) error {
		res.TypedSpec().DomainXML = `<domain><name>vm</name><vcpu>2</vcpu></domain>`
		res.TypedSpec().Disks = []string{second.Metadata().ID()}

		return nil
	})

	s.assertDomain("vm", updated.TypedSpec().DomainXML, true)
	s.assertDiskHeld(second.Metadata().ID(), true)
	s.assertDiskHeld(first.Metadata().ID(), false)

	ready, err := s.State().Teardown(s.Ctx(), spec.Metadata())
	s.Require().NoError(err)
	s.Require().False(ready, "the finalizer must prevent immediate deletion")

	s.assertDomain("vm", "", false)
	s.assertDiskHeld(second.Metadata().ID(), false)
	s.assertFinalizer("vm", false)
}

// Holding a disk status which is on its way out would block its teardown forever. The domain is
// left unstarted instead; the definition it is waiting for is on its way.
func (s *VirtualMachineDomainSuite) TestRefusesToStartOnADiskOnItsWayOut() {
	status := s.newDiskStatus("vm/install@aaaaaaaaaaaa")
	s.AddFinalizer(status.Metadata(), "somebody-else")

	ready, err := s.State().Teardown(s.Ctx(), status.Metadata())
	s.Require().NoError(err)
	s.Require().False(ready)

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "vm")
	spec.TypedSpec().DomainXML = `<domain><name>vm</name><vcpu>1</vcpu></domain>`
	spec.TypedSpec().PowerState = "running"
	spec.TypedSpec().Disks = []string{status.Metadata().ID()}
	s.Create(spec)
	s.start()

	s.Require().Never(func() bool {
		s.client.mu.Lock()
		defer s.client.mu.Unlock()

		_, started := s.client.domains["vm"]

		return started
	}, 200*time.Millisecond, 10*time.Millisecond)

	s.assertDiskHeld(status.Metadata().ID(), false)
}

func (s *VirtualMachineDomainSuite) TestReleasesDisksOfADomainThatNeverStarted() {
	status := s.newDiskStatus("vm/install@aaaaaaaaaaaa")

	s.client.startErr = errors.New("daemon disconnected")

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "vm")
	spec.TypedSpec().DomainXML = `<domain><name>vm</name><vcpu>1</vcpu></domain>`
	spec.TypedSpec().PowerState = "running"
	spec.TypedSpec().Disks = []string{status.Metadata().ID()}
	s.Create(spec)
	s.start()

	s.assertDiskHeld(status.Metadata().ID(), true)
	s.assertFinalizer("vm", true)

	ready, err := s.State().Teardown(s.Ctx(), spec.Metadata())
	s.Require().NoError(err)
	s.Require().False(ready, "the claim must prevent immediate deletion")

	s.assertDiskHeld(status.Metadata().ID(), false)
	s.assertFinalizer("vm", false)

	s.Destroy(spec)
	s.assertDiskHeld(status.Metadata().ID(), false)
}

// A hold reachable from no domain spec at all is one nothing can give back on the spec's behalf:
// a hold taken just before this controller was restarted, or one left by an older generation. It
// has to be swept on the virtual machine's name alone.
func (s *VirtualMachineDomainSuite) TestReleasesDisksOfADestroyedSpec() {
	status := s.newDiskStatus("vm/install@aaaaaaaaaaaa")
	s.AddFinalizer(status.Metadata(), "hypervisor.VirtualMachineController")

	s.start()

	s.assertDiskHeld(status.Metadata().ID(), false)
}

// A domain this controller never claimed is one it will not remove, and it goes on reading its
// disks for as long as it is there. Stopping the spec must not give those holds back under it.
func (s *VirtualMachineDomainSuite) TestKeepsTheDisksOfAnUnclaimedDomainStillPresent() {
	unclaimed := libvirtdomain.Domain{
		Name: diskStatusVM,
		UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), diskStatusVM),
	}
	s.client.domains[unclaimed.Name] = unclaimed

	status := s.newDiskStatus("vm/install@aaaaaaaaaaaa")
	s.AddFinalizer(status.Metadata(), "hypervisor.VirtualMachineController")

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, diskStatusVM)
	spec.TypedSpec().DomainXML = `<domain><name>vm</name><vcpu>1</vcpu></domain>`
	spec.TypedSpec().PowerState = "stopped"
	spec.TypedSpec().Disks = []string{status.Metadata().ID()}
	s.Create(spec)
	s.start()

	select {
	case <-s.client.listed:
	case <-s.Ctx().Done():
		s.FailNow("controller did not inspect the unclaimed domain")
	}

	select {
	case <-s.client.attemptedRemove:
		s.FailNow("controller attempted to remove an unclaimed domain")
	case <-time.After(100 * time.Millisecond):
	}

	s.Require().Never(func() bool {
		held, err := safe.StateGetByID[*hypervisor.VirtualMachineDiskStatus](s.Ctx(), s.State(), status.Metadata().ID())

		return err != nil || !held.Metadata().Finalizers().Has("hypervisor.VirtualMachineController")
	}, 200*time.Millisecond, 10*time.Millisecond)
}

// A definition can name a disk status which is not published yet. That is something to wait for,
// not a failure of the controller: the other domains keep being reconciled meanwhile.
func (s *VirtualMachineDomainSuite) TestWaitsForADiskStatusThatIsNotThereYet() {
	const id = "vm/install@aaaaaaaaaaaa"

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "vm")
	spec.TypedSpec().DomainXML = `<domain><name>vm</name><vcpu>1</vcpu></domain>`
	spec.TypedSpec().PowerState = "running"
	spec.TypedSpec().Disks = []string{id}
	s.Create(spec)

	other := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "other")
	other.TypedSpec().DomainXML = `<domain><name>other</name><vcpu>1</vcpu></domain>`
	other.TypedSpec().PowerState = "running"
	s.Create(other)
	s.start()

	s.assertDomain("other", other.TypedSpec().DomainXML, true)
	s.assertDomain("vm", "", false)

	s.newDiskStatus(id)

	s.assertDomain("vm", spec.TypedSpec().DomainXML, true)
	s.assertDiskHeld(id, true)
}

// A file being replaced is one whose contents are about to stop being what the disk status resolved
// against. The domain waits rather than starting on them, and it keeps its holds while it waits: the
// content library service finds those holds and refuses, so only one of the two gives way.
func (s *VirtualMachineDomainSuite) TestRefusesToStartOnAnImageBeingReplaced() {
	library := s.newContentLibraryStatus(s.T().TempDir())
	s.AddFinalizer(library.Metadata(), hypervisor.ContentLibraryMutationFinalizer("image.raw"))

	status := s.newImageDiskStatus("vm/install@aaaaaaaaaaaa", "",
		hypervisor.VirtualMachineDiskFromImageSpec{Library: diskStatusLibrary, File: "image.raw"})

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, diskStatusVM)
	spec.TypedSpec().DomainXML = `<domain><name>vm</name><vcpu>1</vcpu></domain>`
	spec.TypedSpec().PowerState = "running"
	spec.TypedSpec().Disks = []string{status.Metadata().ID()}
	s.Create(spec)
	s.start()

	s.assertNeverStarted()
	s.assertDiskHeld(status.Metadata().ID(), true)

	s.RemoveFinalizer(library.Metadata(), hypervisor.ContentLibraryMutationFinalizer("image.raw"))

	s.assertDomain(diskStatusVM, spec.TypedSpec().DomainXML, true)
}

// Another file of the same library is nothing this domain reads.
func (s *VirtualMachineDomainSuite) TestStartsOnAnImageAnotherFileIsBeingReplacedAlongside() {
	library := s.newContentLibraryStatus(s.T().TempDir())
	s.AddFinalizer(library.Metadata(), hypervisor.ContentLibraryMutationFinalizer("other.raw"))

	status := s.newImageDiskStatus("vm/install@aaaaaaaaaaaa", "",
		hypervisor.VirtualMachineDiskFromImageSpec{Library: diskStatusLibrary, File: "image.raw"})

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, diskStatusVM)
	spec.TypedSpec().DomainXML = `<domain><name>vm</name><vcpu>1</vcpu></domain>`
	spec.TypedSpec().PowerState = "running"
	spec.TypedSpec().Disks = []string{status.Metadata().ID()}
	s.Create(spec)
	s.start()

	s.assertDomain(diskStatusVM, spec.TypedSpec().DomainXML, true)
}

// The disk status was published against the contents the file had when it resolved, and a library
// file is not immutable. The digest is checked again under the hold, which is the only moment at
// which what was read is what libvirt goes on to open.
func (s *VirtualMachineDomainSuite) TestRefusesToStartOnAnImageWhichNoLongerMatchesItsDigest() {
	path := s.T().TempDir()
	sourcePath := filepath.Join(path, "image.raw")
	s.Require().NoError(os.WriteFile(sourcePath, []byte("talos"), 0o600))

	library := s.newContentLibraryStatus(path)

	status := s.newImageDiskStatus("vm/install@aaaaaaaaaaaa", sourcePath, hypervisor.VirtualMachineDiskFromImageSpec{
		Library: diskStatusLibrary,
		File:    "image.raw",
		Digest:  digest.FromString("talos").String(),
	})

	// Replaced after the status was published, exactly as an upload would have.
	s.Require().NoError(os.WriteFile(sourcePath, []byte("not talos"), 0o600))

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, diskStatusVM)
	spec.TypedSpec().DomainXML = `<domain><name>vm</name><vcpu>1</vcpu></domain>`
	spec.TypedSpec().PowerState = "running"
	spec.TypedSpec().Disks = []string{status.Metadata().ID()}
	s.Create(spec)
	s.start()

	s.assertNeverStarted()

	s.Require().NoError(os.WriteFile(sourcePath, []byte("talos"), 0o600))

	// A library file changing is not an event of its own: what republishes the fingerprint is what
	// tells this controller to look again.
	ctest.UpdateWithConflicts(s, library, func(res *hypervisor.ContentLibraryStatus) error {
		res.TypedSpec().Fingerprint = "changed"

		return nil
	})

	s.assertDomain(diskStatusVM, spec.TypedSpec().DomainXML, true)
}

func (s *VirtualMachineDomainSuite) TestVerifiesAnImageSwappedUnderARunningDomain() {
	path := s.T().TempDir()

	firstPath := filepath.Join(path, "first.raw")
	s.Require().NoError(os.WriteFile(firstPath, []byte("talos"), 0o600))

	secondPath := filepath.Join(path, "second.raw")
	s.Require().NoError(os.WriteFile(secondPath, []byte("not talos"), 0o600))

	library := s.newContentLibraryStatus(path)

	first := s.newImageDiskStatus("vm/install@aaaaaaaaaaaa", firstPath, hypervisor.VirtualMachineDiskFromImageSpec{
		Library: diskStatusLibrary,
		File:    "first.raw",
		Digest:  digest.FromString("talos").String(),
	})

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, diskStatusVM)
	spec.TypedSpec().DomainXML = `<domain><name>vm</name><vcpu>1</vcpu></domain>`
	spec.TypedSpec().PowerState = "running"
	spec.TypedSpec().Disks = []string{first.Metadata().ID()}
	s.Create(spec)
	s.start()

	s.assertDomain(diskStatusVM, spec.TypedSpec().DomainXML, true)

	// The second image does not hash to what its status pinned, exactly as a file replaced after
	// that status was published would not.
	second := s.newImageDiskStatus("vm/install@bbbbbbbbbbbb", secondPath, hypervisor.VirtualMachineDiskFromImageSpec{
		Library: diskStatusLibrary,
		File:    "second.raw",
		Digest:  digest.FromString("talos").String(),
	})

	swapped := `<domain><name>vm</name><vcpu>2</vcpu></domain>`

	ctest.UpdateWithConflicts(s, spec, func(res *hypervisor.VirtualMachineDomainSpec) error {
		res.TypedSpec().DomainXML = swapped
		res.TypedSpec().Disks = []string{second.Metadata().ID()}

		return nil
	})

	// The running domain is left alone, still on the definition which was verified.
	s.Require().Never(func() bool {
		s.client.mu.Lock()
		defer s.client.mu.Unlock()

		return s.client.texts[diskStatusVM] == swapped
	}, 200*time.Millisecond, 10*time.Millisecond)

	// The first image is still read by the domain which is still running, so its hold stays.
	s.assertDiskHeld(first.Metadata().ID(), true)

	s.Require().NoError(os.WriteFile(secondPath, []byte("talos"), 0o600))

	ctest.UpdateWithConflicts(s, library, func(res *hypervisor.ContentLibraryStatus) error {
		res.TypedSpec().Fingerprint = "changed"

		return nil
	})

	s.assertDomain(diskStatusVM, swapped, true)
	s.assertDiskHeld(second.Metadata().ID(), true)
	s.assertDiskHeld(first.Metadata().ID(), false)
}

// Releasing a hold releases what was verified under it: from the moment the hold is gone the file
// may be replaced again, so the next domain to read it has to hash it again rather than trust what
// the last one found.
func (s *VirtualMachineDomainSuite) TestReverifiesAnImageHeldAgainAfterAStop() {
	path := s.T().TempDir()
	sourcePath := filepath.Join(path, "image.raw")
	s.Require().NoError(os.WriteFile(sourcePath, []byte("talos"), 0o600))

	s.newContentLibraryStatus(path)

	status := s.newImageDiskStatus("vm/install@aaaaaaaaaaaa", sourcePath, hypervisor.VirtualMachineDiskFromImageSpec{
		Library: diskStatusLibrary,
		File:    "image.raw",
		Digest:  digest.FromString("talos").String(),
	})

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, diskStatusVM)
	spec.TypedSpec().DomainXML = `<domain><name>vm</name><vcpu>1</vcpu></domain>`
	spec.TypedSpec().PowerState = "running"
	spec.TypedSpec().Disks = []string{status.Metadata().ID()}
	s.Create(spec)
	s.start()

	s.assertDomain(diskStatusVM, spec.TypedSpec().DomainXML, true)
	s.assertDiskHeld(status.Metadata().ID(), true)

	// Stopping gives the hold back, which is the moment the file stops being pinned.
	ctest.UpdateWithConflicts(s, spec, func(res *hypervisor.VirtualMachineDomainSpec) error {
		res.TypedSpec().PowerState = "stopped"

		return nil
	})

	s.assertDomain(diskStatusVM, "", false)
	s.assertDiskHeld(status.Metadata().ID(), false)

	// Replaced while nothing held it, exactly as an overwrite of a stopped guest's image would.
	s.Require().NoError(os.WriteFile(sourcePath, []byte("not talos"), 0o600))

	ctest.UpdateWithConflicts(s, spec, func(res *hypervisor.VirtualMachineDomainSpec) error {
		res.TypedSpec().PowerState = "running"

		return nil
	})

	s.assertNeverStarted()
}

// A hold taken for a domain which then turns out not to be startable is one nothing gives back
// while it waits, and one which refuses to let the file behind it be replaced for just as long. So
// every disk is looked at before any of them is held.
func (s *VirtualMachineDomainSuite) TestTakesNoHoldWhenALaterDiskIsNotThere() {
	status := s.newDiskStatus("vm/install@aaaaaaaaaaaa")

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, diskStatusVM)
	spec.TypedSpec().DomainXML = `<domain><name>vm</name><vcpu>1</vcpu></domain>`
	spec.TypedSpec().PowerState = "running"
	spec.TypedSpec().Disks = []string{status.Metadata().ID(), "vm/data@bbbbbbbbbbbb"}
	s.Create(spec)
	s.start()

	s.assertNeverStarted()
	s.assertDiskHeld(status.Metadata().ID(), false)
}
