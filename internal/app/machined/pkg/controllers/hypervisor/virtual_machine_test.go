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
	"sync/atomic"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	storagectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/storage"
	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	libvirtstorage "github.com/siderolabs/talos/internal/pkg/libvirt/storage"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
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
	guestInterfaces []libvirtdomain.GuestInterface
	opens           int
	closes          int
	removeErr       error
	startErr        error
	listErr         error
	changed         chan struct{}
	attempted       chan struct{}
	attemptedRemove chan struct{}
	listed          chan struct{}
	checkStart      func() error
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

func (c *domainClient) GuestInterfaces(domain libvirtdomain.Domain) ([]libvirtdomain.GuestInterface, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.domains[domain.Name]; !ok || existing.UUID != domain.UUID {
		return nil, fmt.Errorf("domain %q disappeared", domain.Name)
	}

	return c.guestInterfaces, nil
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

func (c *domainClient) Start(domain libvirtdomain.Domain, text string, opts ...libvirtdomain.StartOption) error {
	c.mu.Lock()

	if c.startErr != nil {
		err := c.startErr
		c.mu.Unlock()

		return err
	}

	_, exists := c.domains[domain.Name]
	unchanged := exists && c.texts[domain.Name] == text
	c.mu.Unlock()

	if unchanged {
		return nil
	}

	if err := libvirtdomain.Admit(opts...); err != nil {
		return err
	}

	if c.checkStart != nil {
		if err := c.checkStart(); err != nil {
			return err
		}
	}

	c.mu.Lock()
	c.starts[domain.Name]++
	c.mu.Unlock()

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
	client := s.client

	s.Require().Eventually(func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()

		domain, ok := client.domains[name]
		if !present {
			return !ok
		}

		return ok && domain.UUID == libvirtdomain.UUID(uuid.MustParse(machineUUID), name) && client.texts[name] == text
	}, 5*time.Second, 10*time.Millisecond)
}

func (s *VirtualMachineDomainSuite) assertFinalizer(name string, present bool) {
	ctx, st := s.Ctx(), s.State()

	s.Require().Eventually(func() bool {
		spec, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](ctx, st, name)

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

	client := s.client

	s.Require().Never(func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()

		return client.starts["log-gated"] != 0
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

	client := s.client

	s.Require().Never(func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()

		_, running := client.domains["log-teardown"]

		return client.starts["waiting"] != 0 || !running
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
			client := s.client

			s.Require().Never(func() bool {
				client.mu.Lock()
				defer client.mu.Unlock()

				return client.starts[name] != 0
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

	client := s.client

	s.Require().Never(func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()

		return client.opens != 0
	}, 100*time.Millisecond, 10*time.Millisecond)

	starting := v1alpha1.NewService(virtqemudServiceID)
	starting.TypedSpec().Unknown = true
	s.Create(starting)
	s.Require().Never(func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()

		return client.opens != 0
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
		client.mu.Lock()
		defer client.mu.Unlock()

		return client.opens != opensWhileReady || client.texts["gated"] == updated.TypedSpec().DomainXML
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

	client := s.client

	s.Require().Eventually(func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()

		_, exists := client.domains["restart"]

		return exists && client.starts["restart"] >= 2
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

	client := s.client

	s.Require().Eventually(func() bool {
		select {
		case <-client.attemptedRemove:
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

// outageStorage provides real COSI controllers with a deterministic daemon boundary.
type outageStorage struct {
	libvirtstorage.Client
	mu              sync.Mutex
	failed          bool
	path            string
	pools           map[string]libvirtstorage.Pool
	poolOpenEntered chan struct{}
	poolOpenRelease chan struct{}
}

func (c *outageStorage) open(context.Context) (libvirtstorage.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.failed {
		return nil, errors.New("storage daemon unavailable")
	}

	return c, nil
}

func (c *outageStorage) openPool(ctx context.Context) (libvirtstorage.Client, error) {
	c.mu.Lock()
	entered, release := c.poolOpenEntered, c.poolOpenRelease
	c.mu.Unlock()

	if release != nil {
		select {
		case entered <- struct{}{}:
		default:
		}

		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return c.open(ctx)
}

func (c *outageStorage) setFailed(failed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.failed = failed
}

func (c *outageStorage) Pools() ([]libvirtstorage.Pool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	pools := make([]libvirtstorage.Pool, 0, len(c.pools))
	for _, pool := range c.pools {
		pools = append(pools, pool)
	}

	return pools, nil
}

func (c *outageStorage) Ensure(pool libvirtstorage.Pool, target string, prepare func() error) error {
	_, err := c.EnsureOperation(pool, target, prepare)

	return err
}

func (c *outageStorage) EnsureOperation(pool libvirtstorage.Pool, target string, prepare func() error) (libvirtstorage.OperationOutcome, error) {
	if err := prepare(); err != nil {
		return libvirtstorage.NoMutationSubmitted, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pools == nil {
		c.pools = make(map[string]libvirtstorage.Pool)
	}

	pool.Target = target
	pool.Type = "dir"
	pool.Active = true
	pool.Persistent = true
	c.pools[pool.Name] = pool

	return libvirtstorage.Finished, nil
}

func (c *outageStorage) RemoveOperation(pool libvirtstorage.Pool) (libvirtstorage.OperationOutcome, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.pools, pool.Name)

	return libvirtstorage.Finished, nil
}

func (c *outageStorage) StopOperation(pool libvirtstorage.Pool) (libvirtstorage.OperationOutcome, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if current, ok := c.pools[pool.Name]; ok {
		current.Active = false
		c.pools[pool.Name] = current
	}

	return libvirtstorage.Finished, nil
}

func (*outageStorage) ResizeVolumeOperation(libvirtstorage.Pool, string, uint64) (libvirtstorage.OperationOutcome, error) {
	return libvirtstorage.NoMutationSubmitted, errors.New("resize unavailable in outage fixture")
}

func (c *outageStorage) Volume(_ libvirtstorage.Pool, name string) (libvirtstorage.Volume, bool, error) {
	return libvirtstorage.Volume{Name: name, Path: c.path, Format: "raw", Capacity: 1 << 20}, true, nil
}

func (c *outageStorage) Close() {}

// deadlineVolumeStorage keeps the handshake available but loses the read reply.
type deadlineVolumeStorage struct {
	*outageStorage
	readUnavailable atomic.Bool
}

func (c *deadlineVolumeStorage) Volume(pool libvirtstorage.Pool, name string) (libvirtstorage.Volume, bool, error) {
	if c.readUnavailable.Load() {
		return libvirtstorage.Volume{}, false, context.DeadlineExceeded
	}

	return c.outageStorage.Volume(pool, name)
}

func (s *VirtualMachineDomainSuite) TestPostHandshakeVolumeReadOutageRetainsRunningGuest() {
	store := &deadlineVolumeStorage{outageStorage: &outageStorage{path: filepath.Join(s.T().TempDir(), "vm__system.raw")}}
	s.Require().NoError(os.WriteFile(store.path, make([]byte, 1<<20), 0o600))

	pool := storage.NewStoragePoolSpec(storage.NamespaceName, "vms")
	pool.TypedSpec().VolumeID = "u-vms"
	s.Create(pool)

	mount := block.NewVolumeMountStatus(block.NamespaceName, "u-vms")
	mount.TypedSpec().VolumeID = "u-vms"
	mount.TypedSpec().Target = s.T().TempDir()
	s.Create(mount)

	s.Require().NoError(s.Runtime().RegisterController(&storagectrl.StoragePoolController{Open: store.openPool}))
	s.Require().NoError(s.Runtime().RegisterController(&storagectrl.StoragePoolVolumeController{Open: func(context.Context) (libvirtstorage.Client, error) {
		return store, nil
	}}))
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDiskController{}))
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainSpecController{}))
	s.start()
	s.Create(newOutageVM("vm"))
	s.Require().Eventually(func() bool {
		s.client.mu.Lock()
		defer s.client.mu.Unlock()

		_, exists := s.client.domains["vm"]

		return exists
	}, 5*time.Second, 10*time.Millisecond)

	definition, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), "vm")
	s.Require().NoError(err)

	original := definition.TypedSpec().DomainXML
	diskID := hypervisor.VirtualMachineDiskStatusID("vm", newOutageVM("vm").TypedSpec().Disks[0])

	s.client.mu.Lock()
	starts := s.client.starts["vm"]
	s.client.mu.Unlock()

	store.readUnavailable.Store(true)
	s.Create(newOutageVM("wake"))
	s.Require().Eventually(func() bool {
		volume, getErr := safe.StateGetByID[*storage.StoragePoolVolumeStatus](s.Ctx(), s.State(), "vms/vm__system.raw")

		return getErr == nil && volume.TypedSpec().Error == context.DeadlineExceeded.Error()
	}, 5*time.Second, 10*time.Millisecond)
	volume, err := safe.StateGetByID[*storage.StoragePoolVolumeStatus](s.Ctx(), s.State(), "vms/vm__system.raw")
	s.Require().NoError(err)
	s.Require().Equal(storage.StoragePoolVolumePhaseObservationUnavailable, volume.TypedSpec().Phase)
	s.Require().Equal(store.path, volume.TypedSpec().Path)
	s.Require().Equal("raw", volume.TypedSpec().Format)
	s.Require().Equal(uint64(1<<20), volume.TypedSpec().Capacity)
	s.assertStorageOutageGuestRetained(diskID, original, starts)
	s.Require().Never(func() bool {
		s.client.mu.Lock()
		defer s.client.mu.Unlock()

		_, started := s.client.domains["wake"]

		return started
	}, 200*time.Millisecond, 10*time.Millisecond)

	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm")
	_, err = s.State().Teardown(s.Ctx(), vm.Metadata())
	s.Require().NoError(err)
	s.assertDomain("vm", "", false)
}

func newOutageVM(name string) *hypervisor.VirtualMachineSpec {
	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, name)
	vm.TypedSpec().CPU.Count = 1
	vm.TypedSpec().Memory.Size = 512 << 20
	vm.TypedSpec().Firmware.Type = "bios"
	vm.TypedSpec().PowerState = "running"
	vm.TypedSpec().Disks = []hypervisor.VirtualMachineDiskSpec{
		{
			Name:   "system",
			Pool:   "vms",
			Size:   1 << 20,
			Format: "raw",
			Bus:    "virtio",
			Type:   "disk",
			Provision: hypervisor.VirtualMachineDiskProvisionSpec{
				Blank: true,
			},
		},
	}

	return vm
}

// TestVolumeHeldBeforeStartUntilConfirmedStop checks the attachment hold at the
// Start boundary and retains it until the daemon confirms removal.
func (s *VirtualMachineDomainSuite) TestVolumeHeldBeforeStartUntilConfirmedStop() {
	ctx := s.Ctx()
	st := s.State()
	client := s.client
	disk := newOutageVM("vm").TypedSpec().Disks[0]
	hold := "hypervisor.VirtualMachineController/" + hypervisor.VirtualMachineDiskStatusID("vm", disk)
	client.checkStart = func() error {
		status, err := safe.StateGetByID[*storage.StoragePoolVolumeStatus](ctx, st, "vms/vm__system.raw")
		if err != nil {
			return err
		}

		if !status.Metadata().Finalizers().Has(hold) {
			return errors.New("Start called without consumer volume hold")
		}

		return nil
	}

	s.startStorageOutageVM()
	domain, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), "vm")
	s.Require().NoError(err)
	s.Require().Len(domain.TypedSpec().Disks, 1)
	diskID := domain.TypedSpec().Disks[0]

	client.mu.Lock()
	client.removeErr = errors.New("stop blocked")
	client.mu.Unlock()

	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm")
	ctest.UpdateWithConflicts(s, vm, func(current *hypervisor.VirtualMachineSpec) error {
		current.TypedSpec().PowerState = "stopped"

		return nil
	})
	s.waitResizeSignal(client.attemptedRemove)

	status, err := safe.StateGetByID[*storage.StoragePoolVolumeStatus](ctx, st, "vms/vm__system.raw")
	s.Require().NoError(err)
	s.True(status.Metadata().Finalizers().Has(hold), "desired stop is not confirmed stop")
	s.assertDiskHeld(diskID, true)
	client.mu.Lock()
	client.removeErr = nil
	client.mu.Unlock()
	s.assertDomain("vm", "", false)
	ctest.AssertResource(s, "vms/vm__system.raw", func(current *storage.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.False(current.Metadata().Finalizers().Has(hold))
	})
}

func (s *VirtualMachineDomainSuite) TestAttachedDisksKeepSeparateVolumeHolds() {
	path := filepath.Join(s.T().TempDir(), "shared.raw")
	s.Require().NoError(os.WriteFile(path, make([]byte, 1<<20), 0o600))

	pool := storage.NewStoragePoolStatus(storage.NamespaceName, "vms")
	pool.TypedSpec().Phase = storage.StoragePoolPhaseReady
	s.Create(pool)

	volume := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, "vms/shared.raw")
	*volume.TypedSpec() = storage.StoragePoolVolumeStatusSpec{
		Pool:     "vms",
		Name:     "shared.raw",
		Path:     path,
		Format:   "raw",
		Capacity: 1 << 20,
		Phase:    storage.StoragePoolVolumePhaseReady,
	}
	s.Create(volume)
	s.AddFinalizer(volume.Metadata(), "test.OtherConsumer")

	ids := []string{"vm/first", "vm/second"}
	for _, id := range ids {
		disk := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, id)
		*disk.TypedSpec() = hypervisor.VirtualMachineDiskStatusSpec{
			VirtualMachine: "vm",
			Name:           id,
			Blank:          true,
			Pool:           "vms",
			Volume:         "shared.raw",
			SourcePath:     path,
			Format:         "raw",
			Size:           1 << 20,
			Phase:          hypervisor.VirtualMachineDiskPhaseReady,
		}
		s.Create(disk)
	}

	domain := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "vm")
	domain.TypedSpec().PowerState = "running"
	domain.TypedSpec().DomainXML = `<domain><name>vm</name><vcpu>1</vcpu></domain>`
	domain.TypedSpec().Disks = ids
	s.Create(domain)
	s.start()
	s.assertDomain("vm", domain.TypedSpec().DomainXML, true)
	ctest.AssertResource(s, volume.Metadata().ID(), func(current *storage.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.True(current.Metadata().Finalizers().Has("hypervisor.VirtualMachineController/" + ids[0]))
		asrt.True(current.Metadata().Finalizers().Has("hypervisor.VirtualMachineController/" + ids[1]))
	})
	ctest.UpdateWithConflicts(s, domain, func(current *hypervisor.VirtualMachineDomainSpec) error {
		current.TypedSpec().DomainXML = `<domain><name>vm</name><vcpu>2</vcpu></domain>`
		current.TypedSpec().Disks = ids[1:]

		return nil
	})
	s.assertDomain("vm", `<domain><name>vm</name><vcpu>2</vcpu></domain>`, true)
	ctest.AssertResource(s, volume.Metadata().ID(), func(current *storage.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.False(current.Metadata().Finalizers().Has("hypervisor.VirtualMachineController/" + ids[0]))
		asrt.True(current.Metadata().Finalizers().Has("hypervisor.VirtualMachineController/" + ids[1]))
		asrt.True(current.Metadata().Finalizers().Has("test.OtherConsumer"))
	})
}

func (s *VirtualMachineDomainSuite) TestVolumeWithdrawalBetweenReadAndHold() {
	s.assertVolumeWithdrawal(false)
}

func (s *VirtualMachineDomainSuite) TestRetriesRejectedVolumeHoldCleanup() {
	s.assertVolumeWithdrawal(true)
}

func (s *VirtualMachineDomainSuite) assertVolumeWithdrawal(failRemoval bool) {
	path := filepath.Join(s.T().TempDir(), "shared.raw")
	s.Require().NoError(os.WriteFile(path, make([]byte, 1<<20), 0o600))

	pool := storage.NewStoragePoolStatus(storage.NamespaceName, "vms")
	pool.TypedSpec().Phase = storage.StoragePoolPhaseReady
	s.Create(pool)

	volume := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, "vms/shared.raw")
	*volume.TypedSpec() = storage.StoragePoolVolumeStatusSpec{
		Pool:     "vms",
		Name:     "shared.raw",
		Path:     path,
		Format:   "raw",
		Capacity: 1 << 20,
		Phase:    storage.StoragePoolVolumePhaseReady,
	}
	s.Create(volume, state.WithCreateOwner("storage.StoragePoolVolumeController"))
	s.AddFinalizer(volume.Metadata(), "test.OtherConsumer")

	ids := []string{"vm/first"}
	for _, id := range ids {
		disk := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, id)
		*disk.TypedSpec() = hypervisor.VirtualMachineDiskStatusSpec{
			VirtualMachine: "vm",
			Name:           id,
			Blank:          true,
			Pool:           "vms",
			Volume:         "shared.raw",
			SourcePath:     path,
			Format:         "raw",
			Size:           1 << 20,
			Phase:          hypervisor.VirtualMachineDiskPhaseReady,
		}
		s.Create(disk)
	}

	domain := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "vm")
	domain.TypedSpec().PowerState = "running"
	domain.TypedSpec().DomainXML = `<domain><name>vm</name><vcpu>1</vcpu></domain>`
	domain.TypedSpec().Disks = ids
	s.Create(domain)

	ctx := s.Ctx()
	st := s.State()

	var once sync.Once

	settled := make(chan struct{}, 1)
	failed := false

	s.Require().NoError(s.Runtime().RegisterController(&observedController{
		Controller: &hypervisorctrl.VirtualMachineController{Open: s.client.open},
		settled:    settled,
		beforeDiskHold: func() {
			once.Do(func() {
				_, err := st.Teardown(ctx, volume.Metadata(), state.WithTeardownOwner("storage.StoragePoolVolumeController"))
				s.NoError(err)
			})
		},
		beforeVolumeRelease: func() error {
			if failRemoval && !failed {
				failed = true

				return errors.New("transient rejected-hold cleanup failure")
			}

			return nil
		},
	}))
	s.waitResizeSignal(settled)
	s.client.mu.Lock()
	starts := s.client.starts["vm"]
	s.client.mu.Unlock()
	s.Require().Zero(starts, "a withdrawn volume must not admit actual Start")
	ctest.AssertResource(s, volume.Metadata().ID(), func(current *storage.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, current.Metadata().Phase())
		asrt.False(current.Metadata().Finalizers().Has("hypervisor.VirtualMachineController/"+ids[0]), "failed acquisition must release its new hold")
		asrt.True(current.Metadata().Finalizers().Has("test.OtherConsumer"))
	})
	s.RemoveFinalizer(volume.Metadata(), "test.OtherConsumer")
	s.Destroy(volume, state.WithDestroyOwner("storage.StoragePoolVolumeController"))

	replacement := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, volume.Metadata().ID())
	*replacement.TypedSpec() = *volume.TypedSpec()
	s.Create(replacement, state.WithCreateOwner("storage.StoragePoolVolumeController"))
	s.assertDomain("vm", domain.TypedSpec().DomainXML, true)
	ctest.AssertResource(s, replacement.Metadata().ID(), func(current *storage.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.True(current.Metadata().Finalizers().Has("hypervisor.VirtualMachineController/" + ids[0]))
	})
}

func (s *VirtualMachineDomainSuite) TestVolumeRetainedDuringDomainInventoryFailure() {
	s.startStorageOutageVM()
	domain, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), "vm")
	s.Require().NoError(err)
	s.Require().Len(domain.TypedSpec().Disks, 1)
	diskID := domain.TypedSpec().Disks[0]
	client := s.client
	client.mu.Lock()

	client.listErr = errors.New("QEMU inventory unavailable")
	select {
	case <-client.listed:
	default:
	}
	client.mu.Unlock()

	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm")
	_, err = s.State().Teardown(s.Ctx(), vm.Metadata())
	s.Require().NoError(err)
	s.waitResizeSignal(client.listed)
	s.assertDiskHeld(diskID, true)
	volume, err := safe.StateGetByID[*storage.StoragePoolVolumeStatus](s.Ctx(), s.State(), "vms/vm__system.raw")
	s.Require().NoError(err)
	s.True(volume.Metadata().Finalizers().Has("hypervisor.VirtualMachineController/" + diskID))
	client.mu.Lock()
	_, exists := client.domains["vm"]
	client.listErr = nil
	client.mu.Unlock()
	s.True(exists, "failed inventory is not proof of detach")
	s.assertDomain("vm", "", false)
	ctest.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](s, diskID)
	ctest.AssertNoResource[*storage.StoragePoolVolumeStatus](s, "vms/vm__system.raw")
}

func (s *VirtualMachineDomainSuite) TestStorageDaemonOutage() {
	store, pool, diskID, original, starts := s.startStorageOutageVM()
	s.assertStorageOutageAndRecovery(store, pool, diskID, original, starts, false)
}

func (s *VirtualMachineDomainSuite) TestStorageDaemonOutageVolumeFirst() {
	store, pool, diskID, original, starts := s.startStorageOutageVM()
	s.assertStorageOutageAndRecovery(store, pool, diskID, original, starts, true)
}

func (s *VirtualMachineDomainSuite) TestStorageDaemonOutageMountTeardown() {
	store, pool, diskID, _, _ := s.startStorageOutageVM()
	store.setFailed(true)

	// A failed runtime removal leaves the guest using the backing mount.
	s.client.mu.Lock()
	s.client.removeErr = errors.New("guest removal blocked")
	s.client.mu.Unlock()

	mount := block.NewVolumeMountStatus(block.NamespaceName, pool.TypedSpec().VolumeID)
	ready, err := s.State().Teardown(s.Ctx(), mount.Metadata())
	s.Require().NoError(err)
	s.Require().False(ready)
	s.Require().Eventually(func() bool {
		status, getErr := safe.StateGetByID[*hypervisor.VirtualMachineDiskStatus](s.Ctx(), s.State(), diskID)

		return getErr == nil && status.TypedSpec().Phase == hypervisor.VirtualMachineDiskPhaseNotReady
	}, 5*time.Second, 10*time.Millisecond)
	s.Require().Eventually(func() bool {
		domain, getErr := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), "vm")

		return getErr == nil && domain.TypedSpec().PowerState == "stopped"
	}, 5*time.Second, 10*time.Millisecond, "intentional backing withdrawal must stop the retained definition")
	ctx := s.Ctx()
	st := s.State()
	s.Require().Never(func() bool {
		current, getErr := safe.StateGetByID[*block.VolumeMountStatus](ctx, st, pool.TypedSpec().VolumeID)

		return getErr != nil || !current.Metadata().Finalizers().Has("storage.StoragePoolController")
	}, 300*time.Millisecond, 10*time.Millisecond, "mount must remain held while guest removal is blocked")
	s.assertDiskHeld(diskID, true)
	currentPool, err := safe.StateGetByID[*storage.StoragePoolStatus](s.Ctx(), s.State(), "vms")
	s.Require().NoError(err)
	s.Require().True(currentPool.Metadata().Finalizers().Has("storage.StoragePoolVolumeController"))
	s.client.mu.Lock()
	_, exists := s.client.domains["vm"]
	s.client.mu.Unlock()
	s.Require().True(exists)

	s.client.mu.Lock()
	s.client.removeErr = nil
	s.client.mu.Unlock()
	s.assertDomain("vm", "", false)
	s.assertDiskHeld(diskID, false)
	s.Require().Eventually(func() bool {
		current, getErr := safe.StateGetByID[*storage.StoragePoolStatus](s.Ctx(), s.State(), "vms")

		return getErr == nil && !current.Metadata().Finalizers().Has("storage.StoragePoolVolumeController")
	}, 5*time.Second, 10*time.Millisecond)
	s.Require().Eventually(func() bool {
		current, getErr := safe.StateGetByID[*block.VolumeMountStatus](s.Ctx(), s.State(), pool.TypedSpec().VolumeID)

		return getErr == nil && current.Metadata().Finalizers().Empty()
	}, 10*time.Second, 10*time.Millisecond)
}

func (s *VirtualMachineDomainSuite) TestStorageDaemonOutageChangedVMIntent() {
	store, _, diskID := s.startStorageOutageBacking()
	s.observeVolumeFirstOutage(store, diskID)

	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm")
	ctest.UpdateWithConflicts(s, vm, func(current *hypervisor.VirtualMachineSpec) error {
		current.TypedSpec().CPU.Count = 2

		return nil
	})
	s.assertDomain("vm", "", false)
	s.assertDiskHeld(diskID, false)
}

func (s *VirtualMachineDomainSuite) TestStorageDaemonOutageVMTeardown() {
	store, _, diskID := s.startStorageOutageBacking()
	s.observeVolumeFirstOutage(store, diskID)

	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm")
	_, err := s.State().Teardown(s.Ctx(), vm.Metadata())
	s.Require().NoError(err)
	s.assertDomain("vm", "", false)
	ctest.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](s, diskID)
}

func (s *VirtualMachineDomainSuite) TestStorageDaemonOutagePoolRetarget() {
	store, pool, diskID, _, _ := s.startStorageOutageVM()
	s.observeVolumeFirstOutage(store, diskID)
	ctest.UpdateWithConflicts(s, pool, func(current *storage.StoragePoolSpec) error {
		current.TypedSpec().VolumeID = "u-replacement"

		return nil
	})
	s.assertDomain("vm", "", false)
	s.assertDiskHeld(diskID, false)
}

func (s *VirtualMachineDomainSuite) observeVolumeFirstOutage(store *outageStorage, diskID string) {
	store.setFailed(true)
	s.Create(newOutageVM("volume-first"))
	s.Require().Eventually(func() bool {
		status, err := safe.StateGetByID[*hypervisor.VirtualMachineDiskStatus](s.Ctx(), s.State(), diskID)

		return err == nil && status.TypedSpec().Phase == hypervisor.VirtualMachineDiskPhaseObservationUnavailable
	}, 5*time.Second, 10*time.Millisecond)
	client := s.client
	s.Require().Never(func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()

		_, exists := client.domains["vm"]

		return !exists
	}, 200*time.Millisecond, 10*time.Millisecond)
}

func (s *VirtualMachineDomainSuite) startStorageOutageBacking() (*outageStorage, *storage.StoragePoolSpec, string) {
	store, pool, diskID, _, _ := s.startStorageOutageVM()

	return store, pool, diskID
}

func (s *VirtualMachineDomainSuite) startStorageOutageVM() (*outageStorage, *storage.StoragePoolSpec, string, string, int) {
	store := &outageStorage{path: filepath.Join(s.T().TempDir(), "vm__system.raw")}
	s.Require().NoError(os.WriteFile(store.path, make([]byte, 1<<20), 0o600))

	pool := storage.NewStoragePoolSpec(storage.NamespaceName, "vms")
	pool.TypedSpec().VolumeID = "u-vms"
	s.Create(pool)

	mount := block.NewVolumeMountStatus(block.NamespaceName, pool.TypedSpec().VolumeID)
	mount.TypedSpec().VolumeID = pool.TypedSpec().VolumeID
	mount.TypedSpec().Target = s.T().TempDir()
	s.Create(mount)

	s.Require().NoError(s.Runtime().RegisterController(&storagectrl.StoragePoolController{Open: store.openPool}))
	s.Require().NoError(s.Runtime().RegisterController(&storagectrl.StoragePoolVolumeController{Open: store.open}))
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDiskController{}))
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainSpecController{}))
	s.start()

	vm := newOutageVM("vm")
	s.Create(vm)

	s.Require().Eventually(func() bool {
		status, err := safe.StateGetByID[*storage.StoragePoolVolumeStatus](s.Ctx(), s.State(), "vms/vm__system.raw")

		return err == nil && status.TypedSpec().Phase == storage.StoragePoolVolumePhaseReady
	}, 5*time.Second, 50*time.Millisecond)

	diskID := hypervisor.VirtualMachineDiskStatusID("vm", vm.TypedSpec().Disks[0])

	s.Require().Eventually(func() bool {
		status, err := safe.StateGetByID[*hypervisor.VirtualMachineDiskStatus](s.Ctx(), s.State(), diskID)

		return err == nil && status.TypedSpec().Phase == hypervisor.VirtualMachineDiskPhaseReady
	}, 5*time.Second, 50*time.Millisecond)
	s.Require().Eventually(func() bool {
		s.client.mu.Lock()
		defer s.client.mu.Unlock()

		_, ok := s.client.domains["vm"]

		return ok
	}, 5*time.Second, 10*time.Millisecond)

	s.assertDiskHeld(diskID, true)
	domain, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), "vm")
	s.Require().NoError(err)

	original := domain.TypedSpec().DomainXML

	s.client.mu.Lock()
	starts := s.client.starts["vm"]
	s.client.mu.Unlock()
	s.Require().Positive(starts)

	return store, pool, diskID, original, starts
}

func (s *VirtualMachineDomainSuite) assertVolumeFirstStorageOutage(starts int) {
	// Publishing a volume wakes its controller while the pool still reports Ready.
	s.Create(newOutageVM("volume-first"))
	s.Require().Eventually(func() bool {
		status, err := safe.StateGetByID[*storage.StoragePoolVolumeStatus](s.Ctx(), s.State(), "vms/vm__system.raw")

		return err == nil && status.TypedSpec().Phase == storage.StoragePoolVolumePhaseObservationUnavailable
	}, 5*time.Second, 10*time.Millisecond)
	status, err := safe.StateGetByID[*storage.StoragePoolStatus](s.Ctx(), s.State(), "vms")
	s.Require().NoError(err)
	s.Require().Equal(storage.StoragePoolPhaseReady, status.TypedSpec().Phase)
	client := s.client
	s.Require().Never(func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()

		_, exists := client.domains["vm"]
		_, started := client.domains["volume-first"]

		return !exists || started || client.starts["vm"] != starts
	}, time.Second, 10*time.Millisecond, "volume-first observation loss must retain the guest and prevent new starts")
	volume, err := safe.StateGetByID[*storage.StoragePoolVolumeStatus](s.Ctx(), s.State(), "vms/vm__system.raw")
	s.Require().NoError(err)
	s.Require().Equal(storage.StoragePoolVolumePhaseObservationUnavailable, volume.TypedSpec().Phase)
}

func (s *VirtualMachineDomainSuite) assertStorageOutageGuestRetained(diskID, original string, starts int) {
	ctx := s.Ctx()
	st := s.State()
	client := s.client
	s.Require().Never(func() bool {
		client.mu.Lock()
		_, exists := client.domains["vm"]
		client.mu.Unlock()

		definition, getErr := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](ctx, st, "vm")

		return !exists || getErr != nil || definition.TypedSpec().PowerState != "running" || definition.TypedSpec().DomainXML != original
	}, 300*time.Millisecond, 10*time.Millisecond)
	s.assertDiskHeld(diskID, true)
	s.client.mu.Lock()
	s.Equal(starts, s.client.starts["vm"], "outage must not restart a running guest")
	s.client.mu.Unlock()
}

func (s *VirtualMachineDomainSuite) assertStorageOutageAndRecovery(store *outageStorage, pool *storage.StoragePoolSpec,
	diskID, original string, starts int, volumeFirst bool,
) {
	wakeup := storage.NewStoragePoolSpec(storage.NamespaceName, "wakeup")
	wakeup.TypedSpec().VolumeID = "u-vms"

	if volumeFirst {
		// Park the real pool controller at its daemon boundary before publishing
		// the volume loss. Volume-status updates also wake the pool controller.
		entered := make(chan struct{}, 1)
		release := make(chan struct{})
		releasePool := sync.OnceFunc(func() { close(release) })
		s.T().Cleanup(releasePool)
		store.mu.Lock()
		store.poolOpenEntered = entered
		store.poolOpenRelease = release
		store.mu.Unlock()

		s.Create(wakeup)

		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			s.FailNow("pool controller did not reach the daemon boundary")
		}

		store.setFailed(true)
		s.assertVolumeFirstStorageOutage(starts)
		releasePool()
	} else {
		store.setFailed(true)
		// A distinct pool event wakes the controller after the transport fails.
		s.Create(wakeup)
	}

	s.Require().Eventually(func() bool {
		status, getErr := safe.StateGetByID[*storage.StoragePoolStatus](s.Ctx(), s.State(), "vms")

		return getErr == nil && status.TypedSpec().Phase == storage.StoragePoolPhaseObservationUnavailable
	}, 5*time.Second, 10*time.Millisecond)
	s.Require().Eventually(func() bool {
		status, getErr := safe.StateGetByID[*hypervisor.VirtualMachineDiskStatus](s.Ctx(), s.State(), diskID)

		return getErr == nil && status.TypedSpec().Phase == hypervisor.VirtualMachineDiskPhaseObservationUnavailable
	}, 5*time.Second, 10*time.Millisecond)

	s.assertStorageOutageGuestRetained(diskID, original, starts)

	s.Create(newOutageVM("new-vm"))
	client := s.client
	s.Require().Never(func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()

		_, exists := client.domains["new-vm"]

		return exists
	}, 200*time.Millisecond, 10*time.Millisecond)

	store.setFailed(false)
	s.Destroy(wakeup)
	s.Require().Eventually(func() bool {
		status, getErr := safe.StateGetByID[*storage.StoragePoolStatus](s.Ctx(), s.State(), "vms")

		return getErr == nil && status.TypedSpec().Phase == storage.StoragePoolPhaseReady
	}, 5*time.Second, 10*time.Millisecond)
	s.assertDomain("vm", original, true)
	s.client.mu.Lock()
	s.Equal(starts, s.client.starts["vm"], "recovery must not restart a running guest")
	s.client.mu.Unlock()

	_, err := s.State().Teardown(s.Ctx(), pool.Metadata())
	s.Require().NoError(err)
	s.assertDomain("vm", "", false)
	s.assertDiskHeld(diskID, false)
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
	status.TypedSpec().Phase = hypervisor.VirtualMachineDiskPhaseReady
	s.Create(status)

	return status
}

// newImageDiskStatus publishes a disk status resolved against a content library file.
func (s *VirtualMachineDomainSuite) newImageDiskStatus(id, sourcePath string, image hypervisor.VirtualMachineDiskFromImageSpec) *hypervisor.VirtualMachineDiskStatus {
	status := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, id)
	status.TypedSpec().VirtualMachine = diskStatusVM
	status.TypedSpec().Name = "install"
	status.TypedSpec().Phase = hypervisor.VirtualMachineDiskPhaseReady
	status.TypedSpec().SourcePath = sourcePath
	status.TypedSpec().Image = image
	s.Create(status)

	return status
}

// newContentLibraryStatus publishes a ready library a disk status can name.
func (s *VirtualMachineDomainSuite) newContentLibraryStatus(path string) *hypervisor.ContentLibraryStatus {
	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, diskStatusLibrary)
	library.TypedSpec().VolumeID = "u-" + diskStatusLibrary
	library.TypedSpec().Phase = hypervisor.ContentLibraryPhaseReady
	library.TypedSpec().Path = path
	s.Create(library)

	return library
}

func (s *VirtualMachineDomainSuite) assertNeverStarted() {
	client := s.client

	s.Require().Never(func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()

		_, started := client.domains[diskStatusVM]

		return started
	}, 200*time.Millisecond, 10*time.Millisecond)
}

func (s *VirtualMachineDomainSuite) assertDiskHeld(id string, held bool) {
	ctx, st := s.Ctx(), s.State()

	s.Require().Eventually(func() bool {
		status, err := safe.StateGetByID[*hypervisor.VirtualMachineDiskStatus](ctx, st, id)

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

	client := s.client

	s.Require().Never(func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()

		_, started := client.domains["vm"]

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

	ctx, st := s.Ctx(), s.State()

	s.Require().Never(func() bool {
		held, err := safe.StateGetByID[*hypervisor.VirtualMachineDiskStatus](ctx, st, status.Metadata().ID())

		return err != nil || !held.Metadata().Finalizers().Has("hypervisor.VirtualMachineController")
	}, 200*time.Millisecond, 10*time.Millisecond)
}

// The start-time intent check reads the same projected seed as the domain renderer. A ready
// status alone does not grant the starter access to that projection in COSI.
func (s *VirtualMachineDomainSuite) TestStartsMatchingProjectedCloudInitSeed() {
	path := s.T().TempDir()
	diskPath := filepath.Join(path, "vm__system.raw")
	s.Require().NoError(os.WriteFile(diskPath, make([]byte, 1<<20), 0o600))

	disk := hypervisor.VirtualMachineDiskSpec{
		Name:   "system",
		Pool:   "vms",
		Type:   "disk",
		Bus:    "virtio",
		Format: "raw",
		Size:   1 << 20,
		Provision: hypervisor.VirtualMachineDiskProvisionSpec{
			Blank: true,
		},
	}

	poolSpec := storage.NewStoragePoolSpec(storage.NamespaceName, disk.Pool)
	poolSpec.TypedSpec().VolumeID = "u-vms"
	s.Create(poolSpec)

	pool := storage.NewStoragePoolStatus(storage.NamespaceName, disk.Pool)
	pool.TypedSpec().VolumeID = poolSpec.TypedSpec().VolumeID
	pool.TypedSpec().Phase = storage.StoragePoolPhaseReady
	s.Create(pool)

	mount := block.NewVolumeMountStatus(block.NamespaceName, pool.TypedSpec().VolumeID)
	mount.TypedSpec().Target = path
	s.Create(mount)

	diskStatus := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, hypervisor.VirtualMachineDiskStatusID(diskStatusVM, disk))
	*diskStatus.TypedSpec() = hypervisor.VirtualMachineDiskStatusSpec{
		VirtualMachine: diskStatusVM, Name: disk.Name, Blank: true, Pool: disk.Pool, Volume: "vm__system.raw",
		SourcePath: diskPath,
		Format:     "raw",
		Size:       disk.Size,
		Phase:      hypervisor.VirtualMachineDiskPhaseReady,
	}
	s.Create(diskStatus)

	volume := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, "vms/vm__system.raw")
	*volume.TypedSpec() = storage.StoragePoolVolumeStatusSpec{
		Pool:     "vms",
		Name:     "vm__system.raw",
		Path:     diskPath,
		Format:   "raw",
		Capacity: disk.Size,
		Phase:    storage.StoragePoolVolumePhaseReady,
	}
	s.Create(volume)

	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, diskStatusVM)
	vm.TypedSpec().CPU.Count = 1
	vm.TypedSpec().Memory.Size = 512 << 20
	vm.TypedSpec().Firmware.Type = "bios"
	vm.TypedSpec().PowerState = "running"
	vm.TypedSpec().Disks = []hypervisor.VirtualMachineDiskSpec{disk}
	vm.TypedSpec().CloudInit = &hypervisor.VirtualMachineCloudInitSpec{
		Library:  diskStatusLibrary,
		MetaData: "instance-id: vm\n",
		UserData: "guest-data",
	}
	s.Create(vm)

	projection := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, diskStatusVM)
	projection.TypedSpec().Library = vm.TypedSpec().CloudInit.Library
	projection.TypedSpec().MetaData = vm.TypedSpec().CloudInit.MetaData
	projection.TypedSpec().UserData = vm.TypedSpec().CloudInit.UserData
	s.Create(projection)

	currentProjection, err := safe.StateGetByID[*hypervisor.CloudInitSpec](s.Ctx(), s.State(), diskStatusVM)
	s.Require().NoError(err)

	library := s.newContentLibraryStatus(path)
	seedID := hypervisor.CloudInitStatusID(diskStatusVM, *currentProjection.TypedSpec())
	seedName := "cloud-init-test.iso"
	seedContents := []byte("seed-content")
	s.Require().NoError(os.WriteFile(filepath.Join(path, seedName), seedContents, 0o600))

	seed := hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, seedID)
	seed.TypedSpec().VirtualMachine = diskStatusVM
	seed.TypedSpec().Library = diskStatusLibrary
	seed.TypedSpec().Name = seedName
	seed.TypedSpec().Path = path
	seed.TypedSpec().VolumeID = library.TypedSpec().VolumeID
	seed.TypedSpec().Digest = digest.FromBytes(seedContents).String()
	seed.TypedSpec().SizeBytes = uint64(len(seedContents))
	seed.TypedSpec().InputDigest = currentProjection.TypedSpec().InputDigest()
	seed.TypedSpec().ObservedGeneration = currentProjection.Metadata().Version().String()
	seed.TypedSpec().Phase = hypervisor.CloudInitPhaseReady
	s.Create(seed)

	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainSpecController{}))
	s.Require().Eventually(func() bool {
		domain, getErr := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), diskStatusVM)

		return getErr == nil && domain.TypedSpec().PowerState == "running" && domain.TypedSpec().CloudInit == seedID &&
			len(domain.TypedSpec().DomainXML) > 0
	}, 5*time.Second, 10*time.Millisecond)

	domain, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), diskStatusVM)
	s.Require().NoError(err)
	s.Require().Contains(domain.TypedSpec().DomainXML, filepath.Join(path, seedName))
	s.start()
	s.assertDomain(diskStatusVM, domain.TypedSpec().DomainXML, true)
	s.assertSeedHeld(seedID, true)

	s.client.mu.Lock()
	starts := s.client.starts[diskStatusVM]
	s.client.mu.Unlock()
	s.Require().Equal(1, starts, "matching current VM intent must reach client.Start")

	ctest.UpdateWithConflicts(s, diskStatus, func(current *hypervisor.VirtualMachineDiskStatus) error {
		current.TypedSpec().Phase = hypervisor.VirtualMachineDiskPhaseObservationUnavailable
		current.TypedSpec().Error = "storage observation unavailable"

		return nil
	})
	ctx := s.Ctx()
	st := s.State()
	s.Require().Never(func() bool {
		current, getErr := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](ctx, st, diskStatusVM)

		return getErr != nil || current.TypedSpec().PowerState != "running" || current.TypedSpec().CloudInit != seedID
	}, 200*time.Millisecond, 10*time.Millisecond)
	s.assertDomain(diskStatusVM, domain.TypedSpec().DomainXML, true)
	s.assertSeedHeld(seedID, true)

	// The outage does not excuse invalid seed backing.
	ctest.UpdateWithConflicts(s, library, func(current *hypervisor.ContentLibraryStatus) error {
		current.TypedSpec().Phase = hypervisor.ContentLibraryPhaseNotReady

		return nil
	})
	s.assertDomain(diskStatusVM, "", false)

	// Recover both dependencies before testing a different seed identity change.
	ctest.UpdateWithConflicts(s, library, func(current *hypervisor.ContentLibraryStatus) error {
		current.TypedSpec().Phase = hypervisor.ContentLibraryPhaseReady

		return nil
	})
	ctest.UpdateWithConflicts(s, diskStatus, func(current *hypervisor.VirtualMachineDiskStatus) error {
		current.TypedSpec().Phase = hypervisor.VirtualMachineDiskPhaseReady
		current.TypedSpec().Error = ""

		return nil
	})
	s.assertDomain(diskStatusVM, domain.TypedSpec().DomainXML, true)

	ctest.UpdateWithConflicts(s, diskStatus, func(current *hypervisor.VirtualMachineDiskStatus) error {
		current.TypedSpec().Phase = hypervisor.VirtualMachineDiskPhaseObservationUnavailable

		return nil
	})
	ctest.UpdateWithConflicts(s, seed, func(current *hypervisor.CloudInitStatus) error {
		current.TypedSpec().Name = "different-seed.iso"

		return nil
	})
	s.assertDomain(diskStatusVM, "", false)
}

func (s *VirtualMachineDomainSuite) TestCloudInitSeedHeldUntilDomainRemoval() {
	path := s.T().TempDir()
	seedName := "cloud-init-test.iso"
	seedPath := filepath.Join(path, seedName)
	s.Require().NoError(os.WriteFile(seedPath, []byte("seed-content"), 0o600))
	library := s.newContentLibraryStatus(path)
	seed := hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, "vm@seed")
	seed.TypedSpec().VirtualMachine = diskStatusVM
	seed.TypedSpec().Library = diskStatusLibrary
	seed.TypedSpec().Name = seedName
	seed.TypedSpec().Path = path
	seed.TypedSpec().VolumeID = library.TypedSpec().VolumeID
	seed.TypedSpec().Digest = digest.FromString("seed-content").String()
	seed.TypedSpec().SizeBytes = uint64(len("seed-content"))
	seed.TypedSpec().InputDigest = "sha256:input"
	seed.TypedSpec().Phase = hypervisor.CloudInitPhaseReady
	s.Create(seed)

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, diskStatusVM)
	spec.TypedSpec().PowerState = "running"
	spec.TypedSpec().CloudInit = seed.Metadata().ID()
	spec.TypedSpec().DomainXML = `<domain><name>vm</name><devices><disk type="file" device="cdrom"><source file="` + seedPath + `"/><target dev="hdz" bus="ide"/><readonly/></disk></devices></domain>`
	s.Create(spec)
	s.client.mu.Lock()
	s.client.startErr = errors.New("daemon unavailable")
	s.client.mu.Unlock()
	s.start()
	s.Require().Eventually(func() bool {
		current, err := safe.StateGetByID[*hypervisor.CloudInitStatus](s.Ctx(), s.State(), seed.Metadata().ID())

		return err == nil && current.Metadata().Finalizers().Has("hypervisor.VirtualMachineController")
	}, 5*time.Second, 10*time.Millisecond)
	s.client.mu.Lock()
	s.client.startErr = nil
	s.client.mu.Unlock()
	ctest.UpdateWithConflicts(s, spec, func(current *hypervisor.VirtualMachineDomainSpec) error {
		current.TypedSpec().DomainXML = spec.TypedSpec().DomainXML

		return nil
	})
	s.assertDomain(diskStatusVM, spec.TypedSpec().DomainXML, true)
	s.Require().Eventually(func() bool {
		current, err := safe.StateGetByID[*hypervisor.CloudInitStatus](s.Ctx(), s.State(), seed.Metadata().ID())

		return err == nil && current.Metadata().Finalizers().Has("hypervisor.VirtualMachineController")
	}, 5*time.Second, 10*time.Millisecond)
	ready, err := s.State().Teardown(s.Ctx(), seed.Metadata())
	s.Require().NoError(err)
	s.Require().False(ready, "running domain must hold its seed")
	ctest.UpdateWithConflicts(s, spec, func(current *hypervisor.VirtualMachineDomainSpec) error {
		current.TypedSpec().PowerState = "stopped"

		return nil
	})
	s.assertDomain(diskStatusVM, "", false)
	s.Require().Eventually(func() bool {
		current, getErr := safe.StateGetByID[*hypervisor.CloudInitStatus](s.Ctx(), s.State(), seed.Metadata().ID())

		return getErr == nil && current.Metadata().Finalizers().Empty()
	}, 5*time.Second, 10*time.Millisecond)
}

// A newly updated VM may still have the old runnable domain definition while the renderer
// waits for seed B. The starter must not hand seed A to libvirt in that interval.
func (s *VirtualMachineDomainSuite) TestDoesNotStartStaleSeedAfterVMIntentChanges() {
	path := s.T().TempDir()
	seedName := "seed-a.iso"
	seedPath := filepath.Join(path, seedName)
	s.Require().NoError(os.WriteFile(seedPath, []byte("seed-a"), 0o600))
	library := s.newContentLibraryStatus(path)

	seed := hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, "vm@seed-a")
	seed.TypedSpec().VirtualMachine = diskStatusVM
	seed.TypedSpec().Library = diskStatusLibrary
	seed.TypedSpec().Name = seedName
	seed.TypedSpec().Path = path
	seed.TypedSpec().VolumeID = library.TypedSpec().VolumeID
	seed.TypedSpec().Digest = digest.FromString("seed-a").String()
	seed.TypedSpec().SizeBytes = uint64(len("seed-a"))
	seed.TypedSpec().InputDigest = "sha256:old"
	seed.TypedSpec().Phase = hypervisor.CloudInitPhaseReady
	s.Create(seed)

	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, diskStatusVM)
	vm.TypedSpec().PowerState = "running"
	vm.TypedSpec().CloudInit = &hypervisor.VirtualMachineCloudInitSpec{
		Library:  diskStatusLibrary,
		UserData: "old",
	}
	s.Create(vm)

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, diskStatusVM)
	spec.TypedSpec().PowerState = "running"
	spec.TypedSpec().CloudInit = seed.Metadata().ID()
	spec.TypedSpec().DomainXML = `<domain><name>vm</name><devices><disk type="file" device="cdrom"><source file="` +
		seedPath + `"/><target dev="hdz" bus="ide"/><readonly/></disk></devices></domain>`
	s.Create(spec)

	ctest.UpdateWithConflicts(s, vm, func(current *hypervisor.VirtualMachineSpec) error {
		current.TypedSpec().CloudInit.UserData = "new"

		return nil
	})
	s.start()

	s.Require().Eventually(func() bool {
		s.client.mu.Lock()
		defer s.client.mu.Unlock()

		return s.client.opens > 0
	}, 5*time.Second, 10*time.Millisecond)
	s.assertNeverStarted()
}

// Two failed starts without any domain must not pin the first, obsolete seed indefinitely.
func (s *VirtualMachineDomainSuite) TestReleasesObsoleteFailedStartSeedWithoutDomain() {
	path := s.T().TempDir()
	library := s.newContentLibraryStatus(path)

	newSeed := func(id, content string) (*hypervisor.CloudInitStatus, string) {
		s.T().Helper()

		name := id + ".iso"
		seedPath := filepath.Join(path, name)
		s.Require().NoError(os.WriteFile(seedPath, []byte(content), 0o600))

		seed := hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, id)
		seed.TypedSpec().VirtualMachine = diskStatusVM
		seed.TypedSpec().Library = diskStatusLibrary
		seed.TypedSpec().Name = name
		seed.TypedSpec().Path = path
		seed.TypedSpec().VolumeID = library.TypedSpec().VolumeID
		seed.TypedSpec().Digest = digest.FromString(content).String()
		seed.TypedSpec().SizeBytes = uint64(len(content))
		seed.TypedSpec().InputDigest = "sha256:" + id
		seed.TypedSpec().Phase = hypervisor.CloudInitPhaseReady
		s.Create(seed)

		return seed, `<domain><name>vm</name><devices><disk type="file" device="cdrom"><source file="` +
			seedPath + `"/><target dev="hdz" bus="ide"/><readonly/></disk></devices></domain>`
	}

	seedA, xmlA := newSeed("vm@a", "seed-a")
	seedB, xmlB := newSeed("vm@b", "seed-b")

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, diskStatusVM)
	spec.TypedSpec().PowerState = "running"
	spec.TypedSpec().CloudInit = seedA.Metadata().ID()
	spec.TypedSpec().DomainXML = xmlA
	s.Create(spec)

	s.client.mu.Lock()
	s.client.startErr = errors.New("start refused")
	s.client.mu.Unlock()
	s.start()
	s.assertSeedHeld(seedA.Metadata().ID(), true)
	s.assertFinalizer(diskStatusVM, true)

	ctest.UpdateWithConflicts(s, spec, func(current *hypervisor.VirtualMachineDomainSpec) error {
		current.TypedSpec().CloudInit = seedB.Metadata().ID()
		current.TypedSpec().DomainXML = xmlB

		return nil
	})
	s.assertSeedHeld(seedB.Metadata().ID(), true)
	s.assertSeedHeld(seedA.Metadata().ID(), false)
	s.assertNeverStarted()
}

// A domain still present after Remove fails may still be reading its original seed,
// regardless of a newer stopped definition.
func (s *VirtualMachineDomainSuite) TestKeepsSeedWhenDomainRemovalFails() {
	path := s.T().TempDir()
	name := "seed.iso"
	seedPath := filepath.Join(path, name)
	s.Require().NoError(os.WriteFile(seedPath, []byte("seed"), 0o600))
	library := s.newContentLibraryStatus(path)

	seed := hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, "vm@seed")
	seed.TypedSpec().VirtualMachine = diskStatusVM
	seed.TypedSpec().Library = diskStatusLibrary
	seed.TypedSpec().Name = name
	seed.TypedSpec().Path = path
	seed.TypedSpec().VolumeID = library.TypedSpec().VolumeID
	seed.TypedSpec().Digest = digest.FromString("seed").String()
	seed.TypedSpec().SizeBytes = uint64(len("seed"))
	seed.TypedSpec().InputDigest = "sha256:seed"
	seed.TypedSpec().Phase = hypervisor.CloudInitPhaseReady
	s.Create(seed)

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, diskStatusVM)
	spec.TypedSpec().PowerState = "running"
	spec.TypedSpec().CloudInit = seed.Metadata().ID()
	spec.TypedSpec().DomainXML = `<domain><name>vm</name><devices><disk type="file" device="cdrom"><source file="` +
		seedPath + `"/><target dev="hdz" bus="ide"/><readonly/></disk></devices></domain>`
	s.Create(spec)
	s.start()
	s.assertDomain(diskStatusVM, spec.TypedSpec().DomainXML, true)
	s.assertSeedHeld(seed.Metadata().ID(), true)

	s.client.mu.Lock()
	s.client.removeErr = errors.New("removal uncertain")
	s.client.mu.Unlock()
	ctest.UpdateWithConflicts(s, spec, func(current *hypervisor.VirtualMachineDomainSpec) error {
		current.TypedSpec().PowerState = "stopped"
		current.TypedSpec().CloudInit = ""

		return nil
	})

	select {
	case <-s.client.attemptedRemove:
	case <-s.Ctx().Done():
		s.FailNow("controller did not attempt domain removal")
	}

	s.assertSeedHeld(seed.Metadata().ID(), true)
	s.assertDomain(diskStatusVM, spec.TypedSpec().DomainXML, true)
}

func (s *VirtualMachineDomainSuite) assertSeedHeld(id string, held bool) {
	s.T().Helper()

	s.Require().Eventually(func() bool {
		seed, err := safe.StateGetByID[*hypervisor.CloudInitStatus](s.Ctx(), s.State(), id)

		return err == nil && seed.Metadata().Finalizers().Has("hypervisor.VirtualMachineController") == held
	}, 5*time.Second, 10*time.Millisecond)
}

func (s *VirtualMachineDomainSuite) TestCloudInitSeedRejectedWhenMutationPending() {
	path := s.T().TempDir()
	seedName := "cloud-init-test.iso"
	s.Require().NoError(os.WriteFile(filepath.Join(path, seedName), []byte("seed-content"), 0o600))
	library := s.newContentLibraryStatus(path)
	s.AddFinalizer(library.Metadata(), hypervisor.ContentLibraryMutationFinalizer(seedName))

	seed := hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, "vm@seed")
	seed.TypedSpec().VirtualMachine = diskStatusVM
	seed.TypedSpec().Library = diskStatusLibrary
	seed.TypedSpec().Name = seedName
	seed.TypedSpec().Path = path
	seed.TypedSpec().VolumeID = library.TypedSpec().VolumeID
	seed.TypedSpec().Digest = digest.FromString("seed-content").String()
	seed.TypedSpec().SizeBytes = uint64(len("seed-content"))
	seed.TypedSpec().InputDigest = "sha256:input"
	seed.TypedSpec().Phase = hypervisor.CloudInitPhaseReady
	s.Create(seed)

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, diskStatusVM)
	spec.TypedSpec().PowerState = "running"
	spec.TypedSpec().CloudInit = seed.Metadata().ID()
	spec.TypedSpec().DomainXML = `<domain><name>vm</name><devices><disk type="file" device="cdrom"><source file="` +
		filepath.Join(path, seedName) + `"/><target dev="hdz" bus="ide"/><readonly/></disk></devices></domain>`
	s.Create(spec)
	s.start()
	s.assertNeverStarted()
	s.RemoveFinalizer(library.Metadata(), hypervisor.ContentLibraryMutationFinalizer(seedName))
	s.assertDomain(diskStatusVM, spec.TypedSpec().DomainXML, true)
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
	client := s.client

	s.Require().Never(func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()

		return client.texts[diskStatusVM] == swapped
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

// observedController reports completion of an actual reconciliation, including
// admission errors. Absence is asserted at this boundary, never by sleeping.
type observedController struct {
	controller.Controller
	settled             chan struct{}
	beforeMutation      func()
	beforeDiskHold      func()
	beforeVolumeRelease func() error
}

type observedRuntime struct {
	controller.Runtime
	observer *observedController
}

func (r observedRuntime) AddFinalizer(ctx context.Context, pointer resource.Pointer, finalizers ...resource.Finalizer) error {
	if pointer.Type() == storage.StoragePoolVolumeStatusType && r.observer.beforeDiskHold != nil {
		r.observer.beforeDiskHold()
	}

	for _, finalizer := range finalizers {
		if finalizer == storage.StoragePoolVolumeMutationFinalizer("vm__data.raw") && r.observer.beforeMutation != nil {
			r.observer.beforeMutation()
		}
	}

	return r.Runtime.AddFinalizer(ctx, pointer, finalizers...)
}

func (r observedRuntime) RemoveFinalizer(ctx context.Context, pointer resource.Pointer, finalizers ...resource.Finalizer) error {
	if pointer.Type() == storage.StoragePoolVolumeStatusType && r.observer.beforeVolumeRelease != nil {
		if err := r.observer.beforeVolumeRelease(); err != nil {
			return err
		}
	}

	return r.Runtime.RemoveFinalizer(ctx, pointer, finalizers...)
}

func (r observedRuntime) ResetRestartBackoff() {
	r.Runtime.ResetRestartBackoff()
	r.observer.signal()
}

func (c *observedController) signal() {
	select {
	case c.settled <- struct{}{}:
	default:
	}
}

func (c *observedController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	err := c.Controller.Run(ctx, observedRuntime{Runtime: r, observer: c}, logger)
	c.signal()

	return err
}

// resizeGate pauses a controller without holding any lock used by the other
// actor. The channels establish the interleaving; Once also makes failure cleanup safe.
type resizeGate struct {
	entered     chan struct{}
	release     chan struct{}
	enterOnce   sync.Once
	releaseOnce sync.Once
}

func newResizeGate() *resizeGate {
	return &resizeGate{entered: make(chan struct{}), release: make(chan struct{})}
}

func (g *resizeGate) pause(ctx context.Context) {
	g.enterOnce.Do(func() {
		close(g.entered)

		select {
		case <-g.release:
		case <-ctx.Done():
		}
	})
}

func (g *resizeGate) unblock() {
	g.releaseOnce.Do(func() { close(g.release) })
}

// resizeStorage pauses observation (after the old reconciliation census),
// mutation publication (between admission censuses), or the destructive call.
type resizeStorage struct {
	libvirtstorage.Client
	mu             sync.Mutex
	capacity       uint64
	resizes        int
	observe        func()
	resize         func()
	beforeMutation func()
}

func (c *resizeStorage) Volume(_ libvirtstorage.Pool, name string) (libvirtstorage.Volume, bool, error) {
	if c.observe != nil {
		c.observe()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return libvirtstorage.Volume{Name: name, Path: "/disks/vm__data.raw", Format: "raw", Capacity: c.capacity}, true, nil
}

func (c *resizeStorage) ResizeVolume(libvirtstorage.Pool, string, uint64) error {
	if c.resize != nil {
		c.resize()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.resizes++
	c.capacity = 2 << 20

	return nil
}

func (c *resizeStorage) ResizeVolumeOperation(pool libvirtstorage.Pool, name string, capacity uint64) (libvirtstorage.OperationOutcome, error) {
	err := c.ResizeVolume(pool, name, capacity)
	if err != nil {
		return libvirtstorage.Unknown, err
	}

	return libvirtstorage.Finished, nil
}

func (*resizeStorage) Close() {}

func (s *VirtualMachineDomainSuite) resizeFixture(store *resizeStorage, vmResource bool) *hypervisor.VirtualMachineDiskStatus {
	pool := storage.NewStoragePoolStatus(storage.NamespaceName, "vms")
	pool.TypedSpec().Phase = storage.StoragePoolPhaseReady
	s.Create(pool)

	request := storage.NewStoragePoolVolumeSpec(storage.NamespaceName, "vms/vm__data.raw")
	*request.TypedSpec() = storage.StoragePoolVolumeSpecSpec{
		Pool:     "vms",
		Name:     "vm__data.raw",
		Format:   "raw",
		Capacity: 2 << 20,
	}
	s.Create(request)

	volume := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, request.Metadata().ID())
	*volume.TypedSpec() = storage.StoragePoolVolumeStatusSpec{
		Pool:     "vms",
		Name:     "vm__data.raw",
		Phase:    storage.StoragePoolVolumePhaseReady,
		Path:     "/disks/vm__data.raw",
		Format:   "raw",
		Capacity: 1 << 20,
	}
	s.Create(volume, state.WithCreateOwner("storage.StoragePoolVolumeController"))

	diskSpec := blankDiskSpec("data", "vms", "raw", 2<<20)
	disk := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, hypervisor.VirtualMachineDiskStatusID("vm", diskSpec))
	*disk.TypedSpec() = hypervisor.VirtualMachineDiskStatusSpec{
		VirtualMachine: "vm",
		Name:           "data",
		Blank:          true,
		Pool:           "vms",
		Volume:         "vm__data.raw",
		Phase:          hypervisor.VirtualMachineDiskPhaseReady,
		SourcePath:     "/disks/vm__data.raw",
		Format:         "raw",
	}
	s.Create(disk)

	if vmResource {
		vm := newOutageVM("vm")
		vm.TypedSpec().Disks = []hypervisor.VirtualMachineDiskSpec{diskSpec}
		s.Create(vm)
		s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainSpecController{}))
		s.Require().Eventually(func() bool {
			_, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), "vm")

			return err == nil
		}, 5*time.Second, 10*time.Millisecond)
	} else {
		domain := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "vm")
		domain.TypedSpec().PowerState = "running"
		domain.TypedSpec().DomainXML = `<domain><name>vm</name><vcpu>1</vcpu></domain>`
		domain.TypedSpec().Disks = []string{disk.Metadata().ID()}
		s.Create(domain)
	}

	s.Require().NoError(s.Runtime().RegisterController(&observedController{
		Controller: &storagectrl.StoragePoolVolumeController{
			Open: func(context.Context) (libvirtstorage.Client, error) { return store, nil },
		},
		beforeMutation: store.beforeMutation,
	}))

	return disk
}

func (s *VirtualMachineDomainSuite) waitResizeSignal(signal <-chan struct{}) {
	s.T().Helper()

	select {
	case <-signal:
	case <-s.Ctx().Done():
		s.Require().FailNow("controller did not reach the channel boundary")
	}
}

func (s *VirtualMachineDomainSuite) startObservedVM() <-chan struct{} {
	return s.startObservedVMWithHold(nil)
}

func (s *VirtualMachineDomainSuite) startObservedVMWithHold(beforeDiskHold func()) <-chan struct{} {
	settled := make(chan struct{}, 1)
	s.Require().NoError(s.Runtime().RegisterController(&observedController{
		Controller:     &hypervisorctrl.VirtualMachineController{Open: s.client.open},
		settled:        settled,
		beforeDiskHold: beforeDiskHold,
	}))

	return settled
}

func (s *VirtualMachineDomainSuite) assertResizeDeferred(store *resizeStorage) {
	ctx := s.Ctx()
	st := s.State()

	s.Require().Eventually(func() bool {
		status, err := safe.StateGetByID[*storage.StoragePoolVolumeStatus](ctx, st, "vms/vm__data.raw")

		store.mu.Lock()
		resized := store.resizes != 0
		store.mu.Unlock()

		// Wait for deferral or an actual mutation, so an unsafe resize fails the
		// assertion below rather than timing out waiting for deferred capacity.
		return resized || (err == nil && status.TypedSpec().Phase == storage.StoragePoolVolumePhaseReady && status.TypedSpec().PendingCapacity == 2<<20)
	}, 5*time.Second, 10*time.Millisecond)
	store.mu.Lock()
	resizes := store.resizes
	store.mu.Unlock()
	s.Equal(0, resizes, "a VM that started after the earlier census must prevent resize")

	status, err := safe.StateGetByID[*storage.StoragePoolVolumeStatus](s.Ctx(), s.State(), "vms/vm__data.raw")
	s.Require().NoError(err)
	s.Equal(uint64(1<<20), status.TypedSpec().Capacity)
	s.Equal(uint64(2<<20), status.TypedSpec().PendingCapacity)

	pool, err := safe.StateGetByID[*storage.StoragePoolStatus](s.Ctx(), s.State(), "vms")
	s.Require().NoError(err)
	s.False(pool.Metadata().Finalizers().Has(storage.StoragePoolVolumeMutationFinalizer("vm__data.raw")),
		"deferred growth must release exclusion instead of waiting for disk holds")
}

func (s *VirtualMachineDomainSuite) TestResizeCensusAfterConcurrentStart() {
	gate := newResizeGate()
	defer gate.unblock()

	store := &resizeStorage{capacity: 1 << 20, observe: func() { gate.pause(s.Ctx()) }}
	s.assertResizeDeferredAfterConcurrentStart(store, gate)
}

func (s *VirtualMachineDomainSuite) TestStartBetweenResizeCensuses() {
	gate := newResizeGate()
	defer gate.unblock()

	store := &resizeStorage{capacity: 1 << 20, beforeMutation: func() { gate.pause(s.Ctx()) }}
	s.assertResizeDeferredAfterConcurrentStart(store, gate)
}

func (s *VirtualMachineDomainSuite) assertResizeDeferredAfterConcurrentStart(store *resizeStorage, gate *resizeGate) {
	disk := s.resizeFixture(store, false)
	s.waitResizeSignal(gate.entered)
	s.waitResizeSignal(s.startObservedVM())
	s.assertDomain("vm", `<domain><name>vm</name><vcpu>1</vcpu></domain>`, true)
	s.assertDiskHeld(disk.Metadata().ID(), true)
	// Release only after the VM has started, not merely published intent.
	gate.unblock()
	s.assertResizeDeferred(store)
}

func (s *VirtualMachineDomainSuite) assertStartExcludedDuringResize(vmResource bool) {
	gate := newResizeGate()
	defer gate.unblock()

	store := &resizeStorage{capacity: 1 << 20, resize: func() { gate.pause(s.Ctx()) }}
	disk := s.resizeFixture(store, vmResource)
	s.waitResizeSignal(gate.entered)
	s.waitResizeSignal(s.startObservedVM())
	s.assertDiskHeld(disk.Metadata().ID(), true)
	s.client.mu.Lock()
	_, started := s.client.domains["vm"]
	s.client.mu.Unlock()
	s.Require().False(started, "a completed VM reconciliation must not start while resize is in flight")

	gate.unblock()

	domain, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), "vm")
	s.Require().NoError(err)
	s.assertDomain("vm", domain.TypedSpec().DomainXML, true)
}

func (s *VirtualMachineDomainSuite) TestStartExcludedDuringResize() {
	s.assertStartExcludedDuringResize(false)
}

func (s *VirtualMachineDomainSuite) TestMutationBetweenStartCheckAndDiskHold() {
	observeGate := newResizeGate()
	defer observeGate.unblock()

	holdGate := newResizeGate()
	defer holdGate.unblock()

	resizeGate := newResizeGate()
	defer resizeGate.unblock()

	store := &resizeStorage{
		capacity: 1 << 20,
		observe:  func() { observeGate.pause(s.Ctx()) },
		resize:   func() { resizeGate.pause(s.Ctx()) },
	}
	disk := s.resizeFixture(store, false)
	s.waitResizeSignal(observeGate.entered)
	settled := s.startObservedVMWithHold(func() { holdGate.pause(s.Ctx()) })
	s.waitResizeSignal(holdGate.entered)
	// The VM actor is paused just before committing its disk hold. Let storage
	// publish exclusion and enter ResizeVolume before allowing that hold.
	observeGate.unblock()
	s.waitResizeSignal(resizeGate.entered)
	holdGate.unblock()
	s.waitResizeSignal(settled)
	s.assertDiskHeld(disk.Metadata().ID(), true)
	s.client.mu.Lock()
	_, started := s.client.domains["vm"]
	s.client.mu.Unlock()
	s.Require().False(started, "exclusion must be checked after the disk hold is committed")

	resizeGate.unblock()
	s.assertDomain("vm", `<domain><name>vm</name><vcpu>1</vcpu></domain>`, true)
}

func (s *VirtualMachineDomainSuite) TestVMResourceStartExcludedDuringResize() {
	s.assertStartExcludedDuringResize(true)
}

func (s *VirtualMachineDomainSuite) TestBlankStartFailsClosedWithoutPoolStatus() {
	disk := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, "vm/data@missing")
	disk.TypedSpec().VirtualMachine = "vm"
	disk.TypedSpec().Pool = "vms"
	disk.TypedSpec().Volume = "vm__data.raw"
	disk.TypedSpec().Blank = true
	disk.TypedSpec().Phase = hypervisor.VirtualMachineDiskPhaseReady
	s.Create(disk)

	domain := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "vm")
	domain.TypedSpec().PowerState = "running"
	domain.TypedSpec().DomainXML = `<domain><name>vm</name><vcpu>1</vcpu></domain>`
	domain.TypedSpec().Disks = []string{disk.Metadata().ID()}
	s.Create(domain)

	volume := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, "vms/vm__data.raw")
	volume.TypedSpec().Pool = "vms"
	volume.TypedSpec().Name = "vm__data.raw"
	volume.TypedSpec().Phase = storage.StoragePoolVolumePhaseReady
	s.Create(volume)
	s.waitResizeSignal(s.startObservedVM())
	s.client.mu.Lock()
	_, started := s.client.domains["vm"]
	s.client.mu.Unlock()
	s.Require().False(started, "missing authoritative pool status must not admit a blank disk")

	pool := storage.NewStoragePoolStatus(storage.NamespaceName, "vms")
	pool.TypedSpec().Phase = storage.StoragePoolPhaseReady
	s.Create(pool)
	s.assertDomain("vm", domain.TypedSpec().DomainXML, true)
}
