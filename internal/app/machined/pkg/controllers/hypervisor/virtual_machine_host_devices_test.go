// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func (suite *VirtualMachineSpecSuite) TestPCIDevicesAttachUnmanaged() {
	doc := newVirtualMachine("pci-devices")
	doc.HostDevicesConfig.PCIConfig = []hypervisorcfg.VirtualMachinePCIDevice{
		{PCIAddress: "0000:41:00.0", PCIBootOrder: 1},
		{PCIAddress: "0000:41:00.1"},
		// Written in uppercase: the projection carries the canonical sysfs form.
		{PCIAddress: "0001:AF:1F.7"},
	}
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))

	suite.assertDomain(doc.Name(), "pci-devices")
}

func (suite *VirtualMachineProjectionSuite) TestProjectsPCIDevices() {
	doc := newVirtualMachine("pci-intent")
	doc.HostDevicesConfig.PCIConfig = []hypervisorcfg.VirtualMachinePCIDevice{
		{PCIAddress: "0000:AF:00.0", PCIBootOrder: 2},
		{PCIAddress: "0000:41:00.1"},
	}
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))

	ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
		asrt.Equal([]hypervisor.VirtualMachinePCIDeviceSpec{
			{Address: "0000:af:00.0", BootOrder: 2},
			{Address: "0000:41:00.1"},
		}, res.TypedSpec().PCIDevices)
	})
}

func TestValidateVirtualMachinePCIDevices(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		disks   []hypervisor.VirtualMachineDiskSpec
		devices []hypervisor.VirtualMachinePCIDeviceSpec

		expectedError string
	}{
		{
			name: "valid",
			disks: []hypervisor.VirtualMachineDiskSpec{
				{Name: "system", BootOrder: 1},
			},
			devices: []hypervisor.VirtualMachinePCIDeviceSpec{
				{Address: "0000:41:00.0", BootOrder: 2},
				{Address: "0000:41:00.1"},
			},
		},
		{
			name: "invalid address",
			devices: []hypervisor.VirtualMachinePCIDeviceSpec{
				{Address: "41:00.0"},
			},
			expectedError: `virtual machine "vm": PCI address "41:00.0" must be in the form dddd:bb:ss.f, e.g. 0000:41:00.0`,
		},
		{
			name: "duplicate address",
			devices: []hypervisor.VirtualMachinePCIDeviceSpec{
				{Address: "0000:af:00.0"},
				// The same device in uppercase still collides.
				{Address: "0000:AF:00.0"},
			},
			expectedError: `virtual machine "vm": duplicate PCI device "0000:af:00.0"`,
		},
		{
			name: "boot order collides with a disk",
			disks: []hypervisor.VirtualMachineDiskSpec{
				{Name: "system", BootOrder: 1},
			},
			devices: []hypervisor.VirtualMachinePCIDeviceSpec{
				{Address: "0000:41:00.0", BootOrder: 1},
			},
			expectedError: `virtual machine "vm": PCI device "0000:41:00.0": duplicate boot order 1`,
		},
		{
			name: "boot order collides with another device",
			devices: []hypervisor.VirtualMachinePCIDeviceSpec{
				{Address: "0000:41:00.0", BootOrder: 3},
				{Address: "0000:42:00.0", BootOrder: 3},
			},
			expectedError: `virtual machine "vm": PCI device "0000:42:00.0": duplicate boot order 3`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := hypervisorctrl.ValidateVirtualMachinePCIDevices("vm", &hypervisor.VirtualMachineSpecSpec{
				Disks:      test.disks,
				PCIDevices: test.devices,
			})
			if test.expectedError == "" {
				require.NoError(t, err)

				return
			}

			require.EqualError(t, err, test.expectedError)
		})
	}
}

// hostdevAddress is the host PCI device hostdevDomainXML passes through.
const hostdevAddress = "0000:41:00.0"

// fakePCIDevice lays out the sysfs entries the host device check reads for hostdevAddress, bound to
// the given driver (none when empty).
func fakePCIDevice(t *testing.T, devices, driver string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Join(devices, hostdevAddress), 0o755))

	if driver != "" {
		bindFakePCIDevice(t, devices, driver)
	}
}

// bindFakePCIDevice points the driver link of the fake hostdevAddress at the given driver, as a
// rebind does.
func bindFakePCIDevice(t *testing.T, devices, driver string) {
	t.Helper()

	target := filepath.Join(devices, "drivers", driver)
	require.NoError(t, os.MkdirAll(target, 0o755))

	link := filepath.Join(devices, hostdevAddress, "driver")
	require.NoError(t, os.RemoveAll(link))
	require.NoError(t, os.Symlink(target, link))
}

// newFakeSysfs creates a fake sysfs with no PCI devices, and returns its root and its PCI devices
// directory.
func newFakeSysfs(t *testing.T) (sysfs, devices string) {
	t.Helper()

	sysfs = filepath.Join(t.TempDir(), "sys")
	devices = filepath.Join(sysfs, "bus", "pci", "devices")
	require.NoError(t, os.MkdirAll(devices, 0o755))

	return sysfs, devices
}

// hostdevDomainXML is a minimal definition passing hostdevAddress through.
func hostdevDomainXML(name string) string {
	return `<domain type="kvm"><name>` + name + `</name><vcpu>1</vcpu><devices>` +
		`<hostdev mode="subsystem" type="pci" managed="no"><driver name="vfio"></driver>` +
		`<source><address domain="0x0000" bus="0x41" slot="0x00" function="0x0"></address></source></hostdev>` +
		`</devices></domain>`
}

func TestCheckDomainHostDevices(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		driver string
		absent bool

		expectedError string
	}{
		{
			name:   "bound to vfio-pci",
			driver: "vfio-pci",
		},
		{
			name:          "bound to host driver",
			driver:        "nvidia",
			expectedError: `virtual machine "vm": PCI device "0000:41:00.0": host device is not ready: device is bound to "nvidia", add a PCIDriverRebindConfig targeting vfio-pci`,
		},
		{
			name:          "bound to a VFIO variant driver",
			driver:        "mlx5_vfio_pci",
			expectedError: `virtual machine "vm": PCI device "0000:41:00.0": host device is not ready: device is bound to "mlx5_vfio_pci", add a PCIDriverRebindConfig targeting vfio-pci`,
		},
		{
			name:          "no driver",
			expectedError: `virtual machine "vm": PCI device "0000:41:00.0": host device is not ready: device has no driver, ensure the vfio_pci kernel module is loaded`,
		},
		{
			name:          "absent",
			absent:        true,
			expectedError: `virtual machine "vm": PCI device "0000:41:00.0": host device is not ready: not present on the host`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			sysfs, devices := newFakeSysfs(t)

			if !test.absent {
				fakePCIDevice(t, devices, test.driver)
			}

			err := hypervisorctrl.CheckDomainHostDevices(sysfs, "vm", hostdevDomainXML("vm"))
			if test.expectedError == "" {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, hypervisorctrl.ErrHostDeviceNotReady)
			require.EqualError(t, err, test.expectedError)
		})
	}
}

func TestCheckDomainHostDevicesWithoutDevices(t *testing.T) {
	t.Parallel()

	// Nothing is read from the host when the definition passes no devices through.
	require.NoError(t, hypervisorctrl.CheckDomainHostDevices(filepath.Join(t.TempDir(), "missing"), "vm",
		`<domain type="kvm"><name>vm</name><devices></devices></domain>`))
}

// startWithSysfs runs the status controller, with the controllers it reads, against a fake sysfs.
func (s *VirtualMachineStatusSuite) startWithSysfs(sysfs string) {
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainSpecController{}))
	s.startObserver()
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineStatusController{SysfsPath: sysfs}))
}

// A device not yet handed to vfio-pci is pending and names the driver holding it, as is one the host
// does not have; a list the renderer rejects is an error. A rebind is what the first waits for.
func (s *VirtualMachineStatusSuite) TestHostDeviceStatus() {
	sysfs, devices := newFakeSysfs(s.T())
	fakePCIDevice(s.T(), devices, "nvme")

	name := "vm1"

	bound := newRenderableSpec(name, "running")
	bound.TypedSpec().PCIDevices = []hypervisor.VirtualMachinePCIDeviceSpec{{Address: hostdevAddress}}
	s.Create(bound)

	absent := newRenderableSpec("vm2", "running")
	absent.TypedSpec().PCIDevices = []hypervisor.VirtualMachinePCIDeviceSpec{{Address: "0000:42:00.0"}}
	s.Create(absent)

	duplicate := newRenderableSpec("vm3", "running")
	duplicate.TypedSpec().PCIDevices = []hypervisor.VirtualMachinePCIDeviceSpec{{Address: "0000:43:00.0"}, {Address: "0000:43:00.0"}}
	s.Create(duplicate)

	// A virtual machine meant to be stopped is not held back by its devices.
	stopped := newRenderableSpec("vm4", "stopped")
	stopped.TypedSpec().PCIDevices = []hypervisor.VirtualMachinePCIDeviceSpec{{Address: "0000:44:00.0"}}
	s.Create(stopped)

	s.startWithSysfs(sysfs)

	s.assertStatus(name, "unknown", hypervisorhelpers.VirtualMachineStagePending,
		`virtual machine "vm1": PCI device "0000:41:00.0": host device is not ready: `+
			`device is bound to "nvme", add a PCIDriverRebindConfig targeting vfio-pci`)
	s.assertStatus("vm2", "unknown", hypervisorhelpers.VirtualMachineStagePending,
		`virtual machine "vm2": PCI device "0000:42:00.0": host device is not ready: not present on the host`)
	s.assertStatus("vm3", "unknown", hypervisorhelpers.VirtualMachineStageError,
		`virtual machine "vm3": duplicate PCI device "0000:43:00.0"`)
	s.assertStatus("vm4", "stopped", hypervisorhelpers.VirtualMachineStageReady, "")

	bindFakePCIDevice(s.T(), devices, hypervisorhelpers.VFIOPCIDriver)
	s.Create(hardware.NewPCIDriverRebindStatus(hostdevAddress))

	s.assertStatus(name, "unknown", hypervisorhelpers.VirtualMachineStageUnknown, "domain has not been observed")
}

type hostDeviceFixture struct {
	VirtualMachineDomainSuite

	sysfs   string
	devices string
}

func TestVirtualMachineHostDeviceAdmission(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(*hostDeviceFixture)
	}{
		{"waits for vfio-pci", (*hostDeviceFixture).startWaitsForVFIOPCI},
		{"waits for the device", (*hostDeviceFixture).absentDeviceIsNotStarted},
	} {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := &hostDeviceFixture{}
				s.SetT(t)
				s.SetupTest()
				t.Cleanup(s.TearDownTest)

				s.sysfs, s.devices = newFakeSysfs(t)

				test.run(s)
			})
		})
	}
}

func (s *hostDeviceFixture) start() {
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineController{
		Open:      s.client.open,
		SysfsPath: s.sysfs,
	}))
}

func (s *hostDeviceFixture) newRunningDomainSpec(name string) string {
	domainXML := hostdevDomainXML(name)

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, name)
	spec.TypedSpec().DomainXML = domainXML
	spec.TypedSpec().PowerState = "running"
	s.Create(spec)

	return domainXML
}

func (s *hostDeviceFixture) assertNotStarted(name string) {
	client := s.client

	synctest.Wait()
	client.mu.Lock()
	defer client.mu.Unlock()

	_, started := client.domains[name]
	s.Require().False(started)
	s.Require().Zero(client.starts[name])
}

// The domain is held back, not failed, while the device is on its host driver, and started once a
// rebind hands it to vfio-pci; a domain without devices is not held back meanwhile.
func (s *hostDeviceFixture) startWaitsForVFIOPCI() {
	fakePCIDevice(s.T(), s.devices, "nvme")

	domainXML := s.newRunningDomainSpec("pci")

	plain := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "plain")
	plain.TypedSpec().DomainXML = `<domain><name>plain</name><vcpu>1</vcpu></domain>`
	plain.TypedSpec().PowerState = "running"
	s.Create(plain)

	s.start()

	s.assertDomain("plain", plain.TypedSpec().DomainXML, true)
	s.assertNotStarted("pci")

	bindFakePCIDevice(s.T(), s.devices, hypervisorhelpers.VFIOPCIDriver)
	s.Create(hardware.NewPCIDriverRebindStatus(hostdevAddress))

	s.assertDomain("pci", domainXML, true)
}

// A device the host does not have is never handed to libvirt, and is not a controller failure; the
// start goes ahead once the device shows up and its rebind is reported.
func (s *hostDeviceFixture) absentDeviceIsNotStarted() {
	domainXML := s.newRunningDomainSpec("pci")
	s.start()

	s.assertNotStarted("pci")

	fakePCIDevice(s.T(), s.devices, hypervisorhelpers.VFIOPCIDriver)
	s.Create(hardware.NewPCIDriverRebindStatus(hostdevAddress))

	s.assertDomain("pci", domainXML, true)
}
