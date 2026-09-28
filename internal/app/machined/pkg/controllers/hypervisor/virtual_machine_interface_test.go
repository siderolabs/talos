// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"regexp"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func (suite *VirtualMachineSpecSuite) assertWaitingForLink(reason string) {
	suite.T().Helper()
	suite.Require().Eventually(func() bool {
		return suite.logs.Filter(func(entry observer.LoggedEntry) bool {
			err, ok := entry.ContextMap()["error"].(string)

			return ok && entry.Level < zap.ErrorLevel && strings.Contains(err, reason)
		}).Len() > 0
	}, 30*time.Second, time.Millisecond)

	suite.Assert().Empty(suite.logs.FilterLevelExact(zap.ErrorLevel).All())
}

func (suite *VirtualMachineSpecSuite) createLink(id, alias string) {
	suite.createLinkOfType(id, alias, nethelpers.LinkEther)
}

func (suite *VirtualMachineSpecSuite) createLinkOfType(id, alias string, linkType nethelpers.LinkType) {
	link := network.NewLinkStatus(network.NamespaceName, id)
	link.TypedSpec().Type = linkType
	link.TypedSpec().Alias = alias
	suite.Create(link)
}

func (suite *VirtualMachineSpecSuite) TestInterfacesAttachToResolvedLinks() {
	suite.createLink("eth0", "")
	suite.createLink("macvlan0", "uplink")

	doc := newVirtualMachine("interfaces")
	doc.NetworkingConfig.InterfacesConfig = []hypervisorcfg.VirtualMachineInterface{
		{InterfaceName: "net0", InterfaceLink: "eth0"},
		{InterfaceName: "net1", InterfaceLink: "uplink"},
	}
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))

	suite.assertDomain(doc.Name(), "interfaces")
}

// The fixture pins MACs derived from the names; this checks they also differ per interface and per
// virtual machine.
func (suite *VirtualMachineSpecSuite) TestInterfaceMACsAreDistinct() {
	suite.createLink("eth0", "")

	macs := map[string]struct{}{}

	for _, name := range []string{"mac-one", "mac-two"} {
		spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, name)
		*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 1 << 30},
			PowerState: "running",
			Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
			Interfaces: []hypervisor.VirtualMachineInterfaceSpec{
				{Name: "net0", Link: "eth0"},
				{Name: "net1", Link: "eth0"},
			},
		}
		suite.Create(spec)

		ctest.AssertResource(suite, name, func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
			found := regexp.MustCompile(`<mac address="([^"]+)">`).FindAllStringSubmatch(res.TypedSpec().DomainXML, -1)
			asrt.Len(found, 2)

			for _, match := range found {
				asrt.Regexp(`^52:54:00:[0-9a-f]{2}:[0-9a-f]{2}:[0-9a-f]{2}$`, match[1])
				macs[match[1]] = struct{}{}
			}
		})
	}

	suite.Assert().Len(macs, 4)
}

func (suite *VirtualMachineSpecSuite) TestMissingLinkBlocksRenderUntilItAppears() {
	suite.logs.TakeAll()

	doc := newVirtualMachine("missing-link")
	doc.NetworkingConfig.InterfacesConfig = []hypervisorcfg.VirtualMachineInterface{
		{InterfaceName: "net0", InterfaceLink: "uplink"},
	}
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))

	suite.assertWaitingForLink(`virtual machine "missing-link": interface "net0": host link not found: "uplink"`)
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, doc.Name())

	suite.createLink("macvlan0", "uplink")

	ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Contains(res.TypedSpec().DomainXML, `<source dev="macvlan0" mode="bridge">`)
	})
}

// A link that goes away after the domain was rendered costs the spec its power intent, so
// VirtualMachineController stops the guest instead of leaving it on a definition that can no
// longer be reproduced. The last good definition stays, and the intent returns with the link.
func (suite *VirtualMachineSpecSuite) TestVanishedLinkStopsVirtualMachine() {
	suite.createLink("macvlan0", "uplink")

	doc := newVirtualMachine("vanished-link")
	doc.NetworkingConfig.InterfacesConfig = []hypervisorcfg.VirtualMachineInterface{
		{InterfaceName: "net0", InterfaceLink: "uplink"},
	}
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))

	ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal("running", res.TypedSpec().PowerState)
	})

	link, err := safe.StateGetByID[*network.LinkStatus](suite.Ctx(), suite.State(), "macvlan0")
	suite.Require().NoError(err)
	suite.Destroy(link)

	ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal("stopped", res.TypedSpec().PowerState)
		asrt.Contains(res.TypedSpec().DomainXML, `<source dev="macvlan0" mode="bridge">`)
	})

	suite.createLink("macvlan0", "uplink")

	ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal("running", res.TypedSpec().PowerState)
	})
}

// A macvtap needs an Ethernet lower link. A link of any other type resolves but cannot carry one,
// so it is rejected here rather than at domain start.
func (suite *VirtualMachineSpecSuite) TestNonEthernetLinkIsRejected() {
	suite.logs.TakeAll()
	suite.createLinkOfType("lo", "", nethelpers.LinkLoopbck)

	doc := newVirtualMachine("non-ethernet-link")
	doc.NetworkingConfig.InterfacesConfig = []hypervisorcfg.VirtualMachineInterface{
		{InterfaceName: "net0", InterfaceLink: "lo"},
	}
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))

	suite.assertWaitingForLink(`virtual machine "non-ethernet-link": interface "net0": host link is not an Ethernet link: "lo"`)
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, doc.Name())
}
