// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	talosruntime "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
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

// fakePCIDevice lays out the sysfs entries host device admission reads for one device.
func fakePCIDevice(t *testing.T, root, address, driver string, iommuGroup bool) {
	t.Helper()

	device := filepath.Join(root, address)
	require.NoError(t, os.MkdirAll(device, 0o755))

	if iommuGroup {
		group := filepath.Join(t.TempDir(), "iommu_groups", "7")
		require.NoError(t, os.MkdirAll(group, 0o755))
		require.NoError(t, os.Symlink(group, filepath.Join(device, "iommu_group")))
	}

	if driver != "" {
		bindFakePCIDevice(t, root, address, driver)
	}
}

// bindFakePCIDevice points a fake device's driver link at the given driver, as a rebind does.
func bindFakePCIDevice(t *testing.T, root, address, driver string) {
	t.Helper()

	target := filepath.Join(root, "drivers", driver)
	require.NoError(t, os.MkdirAll(target, 0o755))

	link := filepath.Join(root, address, "driver")
	require.NoError(t, os.RemoveAll(link))
	require.NoError(t, os.Symlink(target, link))
}

// newFakeSysfs creates a fake sysfs with no PCI devices, and returns its root and its PCI devices
// directory. The VFIO IOMMU backend is reported loaded when requested.
func newFakeSysfs(t *testing.T, iommuBackend bool) (sysfs, devices string) {
	t.Helper()

	sysfs = filepath.Join(t.TempDir(), "sys")
	devices = filepath.Join(sysfs, "bus", "pci", "devices")
	require.NoError(t, os.MkdirAll(devices, 0o755))

	if iommuBackend {
		require.NoError(t, os.MkdirAll(filepath.Join(sysfs, "module", "vfio_iommu_type1"), 0o755))
	}

	return sysfs, devices
}

// hostdevDomainXML is a minimal definition passing the device at 0000:41:00.0 through.
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

		driver       string
		iommuGroup   bool
		absent       bool
		noIOMMUModel bool

		expectedErr   error
		expectedError string
	}{
		{
			name:       "bound to vfio-pci",
			driver:     "vfio-pci",
			iommuGroup: true,
		},
		{
			name:          "bound to host driver",
			driver:        "nvidia",
			iommuGroup:    true,
			expectedErr:   hypervisorctrl.ErrHostDevicePending,
			expectedError: `virtual machine "vm": PCI device "0000:41:00.0": host device is not bound to vfio-pci: device is bound to "nvidia", add a PCIDriverRebindConfig targeting vfio-pci`,
		},
		{
			name:          "no driver",
			iommuGroup:    true,
			expectedErr:   hypervisorctrl.ErrHostDevicePending,
			expectedError: `virtual machine "vm": PCI device "0000:41:00.0": host device is not bound to vfio-pci: device has no driver, ensure the vfio_pci kernel module is loaded`,
		},
		{
			name:          "IOMMU disabled",
			driver:        "vfio-pci",
			expectedErr:   hypervisorctrl.ErrHostDeviceUnavailable,
			expectedError: `virtual machine "vm": PCI device "0000:41:00.0": host device is unavailable: device has no IOMMU group, ensure the IOMMU is enabled on the host`,
		},
		{
			name:          "absent",
			absent:        true,
			expectedErr:   hypervisorctrl.ErrHostDeviceUnavailable,
			expectedError: `virtual machine "vm": PCI device "0000:41:00.0": host device is unavailable: not present on the host`,
		},
		{
			name:         "IOMMU backend not loaded",
			driver:       "vfio-pci",
			iommuGroup:   true,
			noIOMMUModel: true,
			expectedErr:  hypervisorctrl.ErrVFIOBackendPending,
			expectedError: `virtual machine "vm": VFIO IOMMU backend is not available: ` +
				`ensure the vfio_iommu_type1 kernel module is loaded`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			sysfs, devices := newFakeSysfs(t, !test.noIOMMUModel)

			if !test.absent {
				fakePCIDevice(t, devices, "0000:41:00.0", test.driver, test.iommuGroup)
			}

			err := hypervisorctrl.CheckDomainHostDevices(sysfs, "vm", hostdevDomainXML("vm"), nil)
			if test.expectedErr == nil {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, test.expectedErr)
			require.EqualError(t, err, test.expectedError)
		})
	}
}

func TestCheckDomainHostDevicesWithoutDevices(t *testing.T) {
	t.Parallel()

	// Nothing is read from the host when the definition passes no devices through.
	require.NoError(t, hypervisorctrl.CheckDomainHostDevices(filepath.Join(t.TempDir(), "missing"), "vm",
		`<domain type="kvm"><name>vm</name><devices></devices></domain>`, nil))
}

// fakeIOMMUGroup lays out a fake IOMMU group holding the given devices, each bound to the given
// driver (none when empty), and links every device to it.
func fakeIOMMUGroup(t *testing.T, sysfs, group string, members map[string]string) {
	t.Helper()

	groupPath := filepath.Join(sysfs, "kernel", "iommu_groups", group)
	require.NoError(t, os.MkdirAll(filepath.Join(groupPath, "devices"), 0o755))

	devices := filepath.Join(sysfs, "bus", "pci", "devices")

	for address, driver := range members {
		fakePCIDevice(t, devices, address, driver, false)
		require.NoError(t, os.Symlink(groupPath, filepath.Join(devices, address, "iommu_group")))
		require.NoError(t, os.Symlink(filepath.Join(devices, address), filepath.Join(groupPath, "devices", address)))
	}
}

// VFIO only opens an IOMMU group whose every device is on vfio-pci, pci-stub or no driver; a group
// with a device still on a host driver is held, naming it, rather than failing as QEMU starts.
func TestCheckDomainHostDevicesIOMMUGroup(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		sibling string

		expectedError string
	}{
		{sibling: ""},
		{sibling: "vfio-pci"},
		{sibling: "pci-stub"},
		{
			sibling: "mlx5_core",
			expectedError: `virtual machine "vm": PCI device "0000:41:00.0": host device is not bound to vfio-pci: ` +
				`device "0000:41:00.1" in the same IOMMU group 7 is bound to "mlx5_core"; ` +
				`every device in the group needs vfio-pci, pci-stub or no driver`,
		},
	} {
		t.Run("sibling on "+test.sibling, func(t *testing.T) {
			t.Parallel()

			sysfs, _ := newFakeSysfs(t, true)
			fakeIOMMUGroup(t, sysfs, "7", map[string]string{"0000:41:00.0": "vfio-pci", "0000:41:00.1": test.sibling})

			err := hypervisorctrl.CheckDomainHostDevices(sysfs, "vm", hostdevDomainXML("vm"), nil)
			if test.expectedError == "" {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, hypervisorctrl.ErrHostDevicePending)
			require.EqualError(t, err, test.expectedError)
		})
	}
}

// A device, or another device of its IOMMU group, which another virtual machine passes through is
// held: a running virtual machine keeps it, and between two which are not running the one whose
// name sorts first gets it.
func TestCheckPCIClaims(t *testing.T) {
	t.Parallel()

	sysfs, devices := newFakeSysfs(t, true)
	fakeIOMMUGroup(t, sysfs, "7", map[string]string{"0000:41:00.0": "vfio-pci", "0000:41:00.1": "vfio-pci"})
	fakeIOMMUGroup(t, sysfs, "8", map[string]string{"0000:42:00.0": "vfio-pci"})

	for _, test := range []struct {
		name  string
		claim hypervisorctrl.PCIClaim

		expectedError string
	}{
		{
			name:          "same device, first by name",
			claim:         hypervisorctrl.NewPCIClaimForTest("a-vm", false, "0000:41:00.0"),
			expectedError: `PCI device "0000:41:00.0": host device is claimed by another virtual machine "a-vm"`,
		},
		{
			name:  "same device, later by name",
			claim: hypervisorctrl.NewPCIClaimForTest("z-vm", false, "0000:41:00.0"),
		},
		{
			name:          "same device, running",
			claim:         hypervisorctrl.NewPCIClaimForTest("z-vm", true, "0000:41:00.0"),
			expectedError: `PCI device "0000:41:00.0": host device is claimed by another virtual machine "z-vm"`,
		},
		{
			name:  "same IOMMU group",
			claim: hypervisorctrl.NewPCIClaimForTest("a-vm", false, "0000:41:00.1"),
			expectedError: `PCI device "0000:41:00.0": host device is claimed by another virtual machine "a-vm", ` +
				`which passes through "0000:41:00.1" in the same IOMMU group 7`,
		},
		{
			name:  "another IOMMU group",
			claim: hypervisorctrl.NewPCIClaimForTest("a-vm", true, "0000:42:00.0"),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := hypervisorctrl.CheckPCIClaims(devices, "vm", []string{"0000:41:00.0"}, []hypervisorctrl.PCIClaim{test.claim})
			if test.expectedError == "" {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, hypervisorctrl.ErrHostDeviceClaimed)
			require.EqualError(t, err, test.expectedError)
		})
	}
}

// startWithSysfs runs the status controller, with the controllers it reads, against a fake sysfs.
func (s *VirtualMachineStatusSuite) startWithSysfs(sysfs string) {
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainSpecController{}))
	s.startObserver()
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineStatusController{SysfsPath: sysfs}))
}

// Loading the VFIO IOMMU backend is no rebind: the held start is woken by the module's status.
func (s *VirtualMachineStatusSuite) TestHostDeviceStatusWaitsForIOMMUBackend() {
	sysfs, devices := newFakeSysfs(s.T(), false)
	fakePCIDevice(s.T(), devices, "0000:41:00.0", hypervisorhelpers.VFIOPCIDriver, true)

	name := "vm1"
	s.client.domains[name] = libvirtdomain.Domain{Name: name, UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), name)}

	spec := newRenderableSpec(name, "running")
	spec.TypedSpec().PCIDevices = []hypervisor.VirtualMachinePCIDeviceSpec{{Address: "0000:41:00.0"}}
	s.Create(spec)

	s.startWithSysfs(sysfs)

	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStagePending,
		`virtual machine "vm1": VFIO IOMMU backend is not available: ensure the vfio_iommu_type1 kernel module is loaded`)

	s.Require().NoError(os.MkdirAll(filepath.Join(sysfs, "module", "vfio_iommu_type1"), 0o755))
	s.Create(talosruntime.NewKernelModuleStatus(talosruntime.NamespaceName, "vfio_iommu_type1"))

	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStageReady, "")
}

// Two virtual machines which pass the same device through: the one running keeps it, even against a
// name which sorts first, and the other is held and names it.
func (s *VirtualMachineStatusSuite) TestHostDeviceStatusClaimedDevice() {
	sysfs, devices := newFakeSysfs(s.T(), true)
	fakePCIDevice(s.T(), devices, "0000:41:00.0", hypervisorhelpers.VFIOPCIDriver, true)

	// Only vm2 runs.
	s.client.domains["vm2"] = libvirtdomain.Domain{Name: "vm2", UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), "vm2")}

	for _, name := range []string{"vm1", "vm2"} {
		spec := newRenderableSpec(name, "running")
		spec.TypedSpec().PCIDevices = []hypervisor.VirtualMachinePCIDeviceSpec{{Address: "0000:41:00.0"}}
		s.Create(spec)
	}

	s.startWithSysfs(sysfs)

	s.assertStatus("vm2", "running", hypervisorhelpers.VirtualMachineStageReady, "")
	s.assertStatus("vm1", "unknown", hypervisorhelpers.VirtualMachineStagePending,
		`virtual machine "vm1": PCI device "0000:41:00.0": host device is claimed by another virtual machine "vm2"`)
}

// A virtual machine stopped through a power override does not claim its devices, even against a
// name which sorts after it.
func (s *VirtualMachineStatusSuite) TestHostDeviceStatusStoppedOverrideReleasesClaim() {
	sysfs, devices := newFakeSysfs(s.T(), true)
	fakePCIDevice(s.T(), devices, "0000:41:00.0", hypervisorhelpers.VFIOPCIDriver, true)

	for _, name := range []string{"vm1", "vm2"} {
		spec := newRenderableSpec(name, "running")
		spec.TypedSpec().PCIDevices = []hypervisor.VirtualMachinePCIDeviceSpec{{Address: "0000:41:00.0"}}
		s.Create(spec)
	}

	override := hypervisor.NewVirtualMachinePowerOverride(hypervisor.NamespaceName, "vm1")
	override.TypedSpec().PowerState = "stopped"
	override.TypedSpec().StopRequest = "graceful/request-1"
	s.Create(override)

	s.startWithSysfs(sysfs)

	s.assertStatus("vm2", "unknown", hypervisorhelpers.VirtualMachineStageUnknown, "domain has not been observed")
}

// A device not yet handed to vfio-pci is pending and names the driver holding it; one the host does
// not have, or a list the renderer rejects, is an error. A rebind is what the pending one waits for.
func (s *VirtualMachineStatusSuite) TestHostDeviceStatus() {
	sysfs, devices := newFakeSysfs(s.T(), true)
	fakePCIDevice(s.T(), devices, "0000:41:00.0", "nvme", true)

	name := "vm1"
	s.client.domains[name] = libvirtdomain.Domain{Name: name, UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), name)}

	bound := newRenderableSpec(name, "running")
	bound.TypedSpec().PCIDevices = []hypervisor.VirtualMachinePCIDeviceSpec{{Address: "0000:41:00.0"}}
	s.Create(bound)

	absent := newRenderableSpec("vm2", "running")
	absent.TypedSpec().PCIDevices = []hypervisor.VirtualMachinePCIDeviceSpec{{Address: "0000:42:00.0"}}
	s.Create(absent)

	duplicate := newRenderableSpec("vm3", "running")
	duplicate.TypedSpec().PCIDevices = []hypervisor.VirtualMachinePCIDeviceSpec{{Address: "0000:43:00.0"}, {Address: "0000:43:00.0"}}
	s.Create(duplicate)

	s.startWithSysfs(sysfs)

	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStagePending,
		`virtual machine "vm1": PCI device "0000:41:00.0": host device is not bound to vfio-pci: `+
			`device is bound to "nvme", add a PCIDriverRebindConfig targeting vfio-pci`)
	s.assertStatus("vm2", "unknown", hypervisorhelpers.VirtualMachineStageError,
		`virtual machine "vm2": PCI device "0000:42:00.0": host device is unavailable: not present on the host`)
	s.assertStatus("vm3", "unknown", hypervisorhelpers.VirtualMachineStageError,
		`virtual machine "vm3": duplicate PCI device "0000:43:00.0"`)

	bindFakePCIDevice(s.T(), devices, "0000:41:00.0", hypervisorhelpers.VFIOPCIDriver)
	s.Create(hardware.NewPCIDriverRebindStatus("0000:41:00.0"))

	s.assertStatus(name, "running", hypervisorhelpers.VirtualMachineStageReady, "")
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
		{"does not start without the device", (*hostDeviceFixture).absentDeviceIsNotStarted},
	} {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := &hostDeviceFixture{}
				s.SetT(t)
				s.SetupTest()
				t.Cleanup(s.TearDownTest)

				s.sysfs, s.devices = newFakeSysfs(t, true)

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

// The domain is held back while the device is on its host driver, and started once a rebind hands
// it to vfio-pci; a domain without devices is not held back meanwhile.
func (s *hostDeviceFixture) startWaitsForVFIOPCI() {
	fakePCIDevice(s.T(), s.devices, "0000:41:00.0", "nvme", true)

	domainXML := s.newRunningDomainSpec("pci")

	plain := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "plain")
	plain.TypedSpec().DomainXML = `<domain><name>plain</name><vcpu>1</vcpu></domain>`
	plain.TypedSpec().PowerState = "running"
	s.Create(plain)

	s.start()

	s.assertDomain("plain", plain.TypedSpec().DomainXML, true)
	s.assertNotStarted("pci")

	bindFakePCIDevice(s.T(), s.devices, "0000:41:00.0", hypervisorhelpers.VFIOPCIDriver)
	s.Create(hardware.NewPCIDriverRebindStatus("0000:41:00.0"))

	s.assertDomain("pci", domainXML, true)
}

// A device the host does not have is never handed to libvirt, and is not a controller failure.
func (s *hostDeviceFixture) absentDeviceIsNotStarted() {
	s.newRunningDomainSpec("pci")
	s.start()

	s.assertNotStarted("pci")
}
