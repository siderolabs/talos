// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"context"
	"net"
	"os"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/perf"
)

const (
	// guestCPUCount and guestMemory are what the guest is asked for, and what it is then checked to
	// have: a Talos guest needs enough of both to reach its API at all.
	guestCPUCount = 2
	guestMemory   = "2GiB"

	// guestBootTimeout bounds a cold boot of a Talos ISO in a nested guest: firmware, the kernel,
	// unpacking the initramfs, then DHCP and apid coming up.
	guestBootTimeout = 5 * time.Minute
)

// TestBootsAndServesItsAPI boots a real Talos ISO in a virtual machine and talks to the guest.
//
// Everything before this asserts what the host was told to do. This asserts what the guest did: it
// booted far enough to bring up networking, took the DHCP lease reserved for the address the
// virtual machine was configured with, and answers its own Talos API describing the hardware it was
// given.
func (suite *LibvirtSuite) TestBootsAndServesItsAPI() {
	suite.requireContentLibrarySupport()

	if suite.HypervisorTalosISOPath == "" {
		suite.T().Skip("skipping as -talos.hypervisor.talos-iso is not set")
	}

	// The lease has to come from the cluster's own DHCP server, which serves reservations only.
	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping as the cluster is not provisioned by qemu")
	}

	extraDHCPRecords := suite.Cluster.Info().Network.ExtraDHCPRecords
	if len(extraDHCPRecords) == 0 {
		suite.T().Skip("skipping as the cluster has no spare DHCP reservations")
	}

	ourRecord := extraDHCPRecords[len(extraDHCPRecords)-1]
	hardwareAddr, err := net.ParseMAC(ourRecord.MAC)
	suite.T().Logf("retrieved DHCP record %s", hardwareAddr)

	suite.Require().NoError(err)

	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	library, _ := provisionContentLibrary(&suite.APISuite, nodeCtx, node)

	dgst := suite.digestOfISO()

	iso, err := os.Open(suite.HypervisorTalosISOPath)
	suite.Require().NoError(err)

	defer iso.Close() //nolint:errcheck

	suite.T().Logf("uploading %s (%s) into content library %q", suite.HypervisorTalosISOPath, dgst, library)

	_, err = suite.Client.ContentLibraryUpload(nodeCtx, library, isoName, false, dgst.String(), iso)
	suite.Require().NoError(err)

	name := "vm-boot-" + uuid.NewString()

	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	doc.CPUConfig.CPUCount = guestCPUCount
	doc.MemoryConfig.MemorySize = meta.MustByteSize(guestMemory)
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{
		{
			DiskName:      "install",
			DiskType:      hypervisorhelpers.VirtualMachineDiskTypeCDROM,
			DiskBootOrder: 1,
			ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
				FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
					ImageLibrary: library,
					ImageFile:    isoName,
					ImageDigest:  dgst.String(),
				},
			},
		},
	}
	doc.NetworkingConfig.InterfacesConfig = []hypervisorcfg.VirtualMachineInterface{
		{
			InterfaceName: "nic0",
			InterfaceLink: "net0",
			// DHCPD operates with reserved hw addresses, we take one for the VM's interface to get a predictable IP assigned.
			HardwareAddressConfig: nethelpers.HardwareAddr(hardwareAddr),
		},
	}

	suite.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		suite.RemoveMachineConfigDocumentsByName(client.WithNode(ctx, node), hypervisorcfg.VirtualMachineConfigKind, name)
	})

	suite.PatchMachineConfig(nodeCtx, doc)

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI, []string{diskStatusID(name, doc.DisksConfig[0])},
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Phase == hypervisor.VirtualMachineDiskPhaseReady, "error: %q", status.TypedSpec().Error)
		},
	)

	// The address only reaches the guest because the domain carries the reserved one.
	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, name,
		func(spec *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
			asrt.Contains(spec.TypedSpec().DomainXML, `<mac address="`+ourRecord.MAC+`">`)
		},
	)

	suite.assertRunningTransientDomainWithDevices(node, name, guestCPUCount, 1, 1)

	guestIP := ourRecord.IP.Addr().String()
	suite.T().Logf("waiting for the guest to take %s and serve its API", guestIP)

	guest := suite.awaitGuestAPI(guestIP)
	defer guest.Close() //nolint:errcheck

	suite.assertGuestHardware(guest)

	// Stop the guest first.
	doc.PowerStateConfig = hypervisorhelpers.PowerStateStopped
	suite.PatchMachineConfig(nodeCtx, doc)
	suite.assertStoppedDomain(nodeCtx, node, name)

	// Start it once more. The disk outlived the stop, so the guest boots the same image again and
	// takes the same lease, which is what makes the removal below a removal of a live guest.
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	suite.PatchMachineConfig(nodeCtx, doc)
	suite.assertRunningTransientDomainWithDevices(node, name, guestCPUCount, 1, 1)

	suite.T().Logf("waiting for the restarted guest to serve its API on %s", guestIP)

	restarted := suite.awaitGuestAPI(guestIP)
	defer restarted.Close() //nolint:errcheck

	// Removing the document alone has to stop the running domain and take every resource derived
	// from it with it: no explicit stop precedes this one.
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, hypervisorcfg.VirtualMachineConfigKind, name)
	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](nodeCtx, suite.T(), suite.Client.COSI, name)
	rtestutils.AssertNoResource[*hypervisor.VirtualMachineStatus](nodeCtx, suite.T(), suite.Client.COSI, name)
	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](nodeCtx, suite.T(), suite.Client.COSI,
		diskStatusID(name, doc.DisksConfig[0]))

	// No status is left to read once the document is gone; ask libvirt whether the domain outlived it.
	suite.assertNoDomain(node, name)
}

// awaitGuestAPI waits for the guest to answer its maintenance API, which it does only once it has
// booted and taken its lease. The client is insecure: a guest with no machine configuration serves
// a certificate from a CA it generates in RAM on each boot, which nothing can be made to trust.
func (suite *LibvirtSuite) awaitGuestAPI(ip string) *client.Client {
	suite.T().Helper()

	ctx, cancel := context.WithTimeout(suite.ctx, guestBootTimeout)
	defer cancel()

	guest, err := client.New(ctx, client.WithMaintenanceMode(ip, nil))
	suite.Require().NoError(err)

	started := time.Now()

	for {
		reqCtx, reqCancel := context.WithTimeout(ctx, 10*time.Second)
		resp, err := guest.Version(reqCtx)

		reqCancel()

		if err == nil {
			suite.T().Logf("guest answered after %s: %s", time.Since(started).Round(time.Second), resp.Messages[0].Version.Tag)

			return guest
		}

		select {
		case <-ctx.Done():
			guest.Close() //nolint:errcheck

			suite.Require().FailNowf("the guest never answered its API", "waited %s, last error: %s",
				time.Since(started).Round(time.Second), err)

			return nil
		case <-time.After(5 * time.Second):
		}
	}
}

// assertGuestHardware checks the guest was given the machine the virtual machine was configured
// with, rather than merely that something on that address answered.
func (suite *LibvirtSuite) assertGuestHardware(guest *client.Client) {
	suite.T().Helper()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Minute)
	defer cancel()

	cores, err := safe.StateListAll[*hardware.CPUCore](ctx, guest.COSI)
	suite.Require().NoError(err)
	suite.Assert().Equal(guestCPUCount, cores.Len(), "the guest must see the cores it was given")

	// Compared with a tolerance against what the kernel reports: the firmware and the kernel itself
	// reserve some of what the domain was given, so the guest never sees quite all of it.
	memory, err := safe.StateGetByID[*perf.Memory](ctx, guest.COSI, perf.MemoryID)
	suite.Require().NoError(err)

	wanted := meta.MustByteSize(guestMemory)
	total := memory.TypedSpec().MemTotal * 1024

	suite.Assert().InEpsilon(wanted.Value(), total, 0.1,
		"the guest must see close to the %s it was given, saw %d bytes", guestMemory, total)
}
