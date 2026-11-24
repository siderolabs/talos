// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"context"
	"io"
	"net"
	"os"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/perf"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// BootSuite boots a Talos ISO and talks to the guest.
type BootSuite struct{ LibvirtSuite }

// SuiteName implements base.NamedSuite.
func (suite *BootSuite) SuiteName() string { return "hypervisor.BootSuite" }

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
func (suite *BootSuite) TestBootsAndServesItsAPI() {
	node, nodeCtx, doc, guestIP := suite.prepareTalosGuest()
	name := doc.Name()

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
	suite.assertRunningTransientDomainWithDevices(node, name, guestCPUCount, 2, 1)

	suite.T().Logf("waiting for the restarted guest to serve its API on %s", guestIP)

	restarted := suite.awaitGuestAPI(guestIP)
	defer restarted.Close() //nolint:errcheck

	// Removing the document alone has to stop the running domain and take every resource derived
	// from it with it: no explicit stop precedes this one.
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, hypervisorcfg.VirtualMachineConfigKind, name)
	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](nodeCtx, suite.T(), suite.Client.State(), name)
	rtestutils.AssertNoResource[*hypervisor.VirtualMachineStatus](nodeCtx, suite.T(), suite.Client.State(), name)
	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](nodeCtx, suite.T(), suite.Client.State(),
		diskStatusID(name, doc.DisksConfig[0]))
	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](nodeCtx, suite.T(), suite.Client.State(),
		diskStatusID(name, doc.DisksConfig[1]))

	// No status is left to read once the document is gone; ask libvirt whether the domain outlived it.
	suite.assertNoDomain(node, name)
}

// TestCopySurvivesSourceDeletionAndHostReboot requires a cold guest boot from the existing pool copy
// after both the library alias and immutable object are gone.
func (suite *BootSuite) TestCopySurvivesSourceDeletionAndHostReboot() {
	if !suite.hostRebootSupported() {
		suite.T().Skip("skipping as host reboots are only supported on qemu")
	}

	node, nodeCtx, doc, guestIP := suite.prepareTalosGuest()
	name := doc.Name()
	disk := doc.DisksConfig[0]
	image := disk.ProvisionConfig.FromImageConfig

	guestMAC := doc.NetworkingConfig.InterfacesConfig[0].HardwareAddressConfig
	beforeBootID := suite.guestBootID(guestIP, guestMAC)

	var copyPath string

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), diskStatusID(name, disk),
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, status.TypedSpec().Phase, "error: %q", status.TypedSpec().Error)
			asrt.True(status.TypedSpec().ReadOnly)
			copyPath = status.TypedSpec().SourcePath
			asrt.NotEmpty(copyPath)
			asrt.NotContains(copyPath, "/objects/")
		},
	)

	// Removing only the alias would leave the ISO object available for a fresh copy.
	_, err := suite.Client.ContentLibraryService().Delete(nodeCtx, &machine.ContentLibraryServiceDeleteRequest{
		LibraryId: image.ImageLibrary,
		Target:    &machine.ContentLibraryServiceDeleteRequest_Name{Name: image.ImageFile},
	})
	suite.Require().NoError(err)

	_, err = suite.Client.ContentLibraryService().Delete(nodeCtx, &machine.ContentLibraryServiceDeleteRequest{
		LibraryId: image.ImageLibrary,
		Target:    &machine.ContentLibraryServiceDeleteRequest_Sha256{Sha256: image.ImageDigest},
	})
	suite.Require().NoError(err)

	assertSourceAbsent := func() {
		suite.T().Helper()

		stream, listErr := suite.Client.ContentLibraryService().List(nodeCtx, &machine.ContentLibraryServiceListRequest{
			LibraryId: image.ImageLibrary,
		})
		suite.Require().NoError(listErr)

		_, listErr = stream.Recv()
		suite.Require().ErrorIs(listErr, io.EOF, "the dedicated library must contain no ISO objects")
	}
	assertSourceAbsent()

	// This is a host reboot, not a VM reboot or daemon restart. No configuration
	// is reapplied afterwards: recovery must adopt the on-disk copy on its own.
	suite.AssertRebootedNoChecks(suite.ctx, node, func(ctx context.Context) error {
		return base.IgnoreGRPCUnavailable(suite.Client.Reboot(ctx))
	}, 5*time.Minute)
	suite.WaitForBootDone(suite.ctx)
	suite.AssertServicesRunning(suite.ctx, node, map[string]string{
		"ext-virtqemud":    "Running",
		"ext-virtstoraged": "Running",
	})

	assertSourceAbsent()
	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), diskStatusID(name, disk),
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, status.TypedSpec().Phase, "error: %q", status.TypedSpec().Error)
			asrt.Equal(copyPath, status.TypedSpec().SourcePath, "recovery must reuse the existing pool copy")
			asrt.True(status.TypedSpec().ReadOnly)
		},
	)
	suite.assertRunningTransientDomainWithDevices(node, name, guestCPUCount, 2, 1)
	suite.Require().Contains(suite.runVirsh(node, "domblklist", name, "--details"), copyPath)

	// A fresh boot ID proves the guest booted again. A libvirt running state alone could also
	// describe a QEMU process stuck at firmware or a boot error.
	suite.Require().NotEqual(beforeBootID, suite.guestBootID(guestIP, guestMAC), "the guest must cold-boot after the host reboot")

	// A graceful stop has no forced fallback, so the domain going away proves the guest obeyed the
	// power button.
	_, err = suite.Client.HypervisorService().Stop(nodeCtx, &machine.VirtualMachineStopRequest{Name: name})
	suite.Require().NoError(err)
	suite.assertStoppedDomain(nodeCtx, node, name)
	suite.assertStopMode(nodeCtx, name, hypervisorhelpers.StopModeGraceful)
}

// guestBootID waits for the guest to serve its API and returns its kernel boot ID. The guest must
// own mac: another machine answering on the address would also report a different boot ID.
func (suite *LibvirtSuite) guestBootID(ip string, mac nethelpers.HardwareAddr) string {
	suite.T().Helper()

	guest := suite.awaitGuestAPI(ip)
	defer guest.Close() //nolint:errcheck

	ctx, cancel := context.WithTimeout(suite.ctx, time.Minute)
	defer cancel()

	links, err := safe.StateListAll[*network.LinkStatus](ctx, guest.State())
	suite.Require().NoError(err)

	_, owned := links.Find(func(link *network.LinkStatus) bool {
		return slices.Equal(link.TypedSpec().HardwareAddr, mac)
	})
	suite.Require().True(owned, "the guest answering on %s must own %s", ip, net.HardwareAddr(mac))

	bootID, err := safe.StateGetByID[*runtime.BootID](ctx, guest.State(), runtime.BootIDID)
	suite.Require().NoError(err)
	suite.Require().NotEmpty(bootID.TypedSpec().BootID)

	return bootID.TypedSpec().BootID
}

// prepareTalosGuest boots the Talos ISO from a content library of its own on a random node, on the
// address of a reserved DHCP lease, and returns the guest once its domain runs. It skips the test
// unless the cluster can serve the guest that lease.
func (suite *LibvirtSuite) prepareTalosGuest() (string, context.Context, *hypervisorcfg.VirtualMachineConfigV1Alpha1, string) {
	suite.T().Helper()
	suite.requireContentLibrarySupport()

	if suite.HypervisorTalosISOPath == "" {
		suite.T().Skip("skipping as -talos.hypervisor.talos-iso is not set")
	}

	// The lease has to come from the cluster's own DHCP server, which serves reservations only.
	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping as the cluster is not provisioned by qemu")
	}

	ourRecord, ok := suite.LeaseDHCPRecord(suite.T())
	if !ok {
		suite.T().Skip("skipping as the cluster has no free DHCP reservations")
	}

	hardwareAddr, err := net.ParseMAC(ourRecord.MAC)
	suite.T().Logf("retrieved DHCP record %s", hardwareAddr)

	suite.Require().NoError(err)

	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	library, _ := provisionContentLibrary(&suite.APISuite, nodeCtx, node)
	pool, _ := suite.provisionContentLibraryPool(nodeCtx, node, library)

	dgst := suite.digestOfISO()

	iso, err := os.Open(suite.HypervisorTalosISOPath)
	suite.Require().NoError(err)

	defer iso.Close() //nolint:errcheck

	suite.T().Logf("uploading %s (%s) into content library %q", suite.HypervisorTalosISOPath, dgst, library)

	_, err = suite.Client.ContentLibraryUpload(nodeCtx, library, isoName, dgst.String(), iso)
	suite.Require().NoError(err)

	name := "vm-boot-" + uuid.NewString()

	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	doc.FirmwareConfig.FirmwareType = suite.guestArch(node).firmware
	doc.CPUConfig.CPUCount = guestCPUCount
	doc.MemoryConfig.MemorySize = meta.MustByteSize(guestMemory)
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{
		{
			DiskName:      "install",
			DiskPool:      pool,
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
		// An empty drive beside the ISO takes the next target and must not get in the boot's way.
		emptyCDROMDisk("spare"),
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
		// The DHCP lease is released after this; the guest must not still hold the address.
		suite.assertNoDomainWithContext(ctx, node, name)
	})

	suite.PatchMachineConfig(nodeCtx, doc)

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.State(),
		[]string{diskStatusID(name, doc.DisksConfig[0]), diskStatusID(name, doc.DisksConfig[1])},
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Phase == hypervisor.VirtualMachineDiskPhaseReady, "error: %q", status.TypedSpec().Error)
		},
	)

	// The address only reaches the guest because the domain carries the reserved one.
	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), name,
		func(spec *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
			asrt.Contains(spec.TypedSpec().DomainXML, `<mac address="`+ourRecord.MAC+`">`)
		},
	)

	suite.assertRunningTransientDomainWithDevices(node, name, guestCPUCount, 2, 1)

	return node, nodeCtx, doc, ourRecord.IP.Addr().String()
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

	cores, err := safe.StateListAll[*hardware.CPUCore](ctx, guest.State())
	suite.Require().NoError(err)
	suite.Assert().Equal(guestCPUCount, cores.Len(), "the guest must see the cores it was given")

	// Compared with a tolerance against what the kernel reports: the firmware and the kernel itself
	// reserve some of what the domain was given, so the guest never sees quite all of it.
	memory, err := safe.StateGetByID[*perf.Memory](ctx, guest.State(), perf.MemoryID)
	suite.Require().NoError(err)

	wanted := meta.MustByteSize(guestMemory)
	total := memory.TypedSpec().MemTotal * 1024

	suite.Assert().InEpsilon(wanted.Value(), total, 0.1,
		"the guest must see close to the %s it was given, saw %d bytes", guestMemory, total)
}

func init() {
	allSuites = append(allSuites, new(BootSuite))
}
