// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
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

const machineUUID = "c737f778-82a1-48dd-990b-67901031bcc5"

type domainClient struct {
	mu              sync.Mutex
	domains         map[string]libvirtdomain.Domain
	texts           map[string]string
	starts          map[string]int
	removeErr       error
	changed         chan struct{}
	attempted       chan struct{}
	attemptedRemove chan struct{}
	listed          chan struct{}
}

func (c *domainClient) open(context.Context) (libvirtdomain.Client, error) { return c, nil }
func (*domainClient) Close()                                               {}

func (c *domainClient) Domains() ([]libvirtdomain.Domain, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case c.listed <- struct{}{}:
	default:
	}

	return slices.Collect(maps.Values(c.domains)), nil
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
		Open:              s.client.open,
		ReconcileInterval: 20 * time.Millisecond,
	}))

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "restart")
	spec.TypedSpec().DomainXML = `<domain><name>restart</name></domain>`
	spec.TypedSpec().PowerState = "running"
	s.Create(spec)
	s.assertDomain("restart", spec.TypedSpec().DomainXML, true)
	s.assertFinalizer("restart", true)

	s.client.mu.Lock()
	delete(s.client.domains, "restart")
	delete(s.client.texts, "restart")
	s.client.mu.Unlock()

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
