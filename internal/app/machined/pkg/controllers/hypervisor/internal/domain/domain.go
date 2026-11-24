// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"libvirt.org/go/libvirtxml"

	diskhelpers "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/disks"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// ErrDiskNotReady denotes a disk whose resolved backing is unavailable.
var ErrDiskNotReady = errors.New("not ready")

// Inputs contains only resolved inputs, without a controller reader or COSI state.
//
//nolint:gocyclo
type Inputs struct {
	Name     string
	Spec     *hypervisor.VirtualMachineSpecSpec
	Profile  *hypervisor.VirtualMachineProfileSpecSpec
	Links    []string
	Disks    map[resource.ID]hypervisor.VirtualMachineDiskStatusSpec
	SeedPath string
	// Platform is the architecture of the host the domain runs on.
	Platform Platform
}

// Build constructs a domain without serializing it, so optional seed media can be attached first.
//
//nolint:gocyclo
func Build(input Inputs) (*libvirtxml.Domain, []string, error) {
	name, spec, profile := input.Name, input.Spec, input.Profile

	platform := input.Platform

	interfaces := renderVirtualMachineInterfaces(name, spec.Interfaces, input.Links)

	domain := libvirtxml.Domain{
		Type: "kvm",
		Name: name,
		VCPU: &libvirtxml.DomainVCPU{
			Value: uint(spec.CPU.Count),
		},
		Memory: &libvirtxml.DomainMemory{
			Value: uint(spec.Memory.Size),
			Unit:  "bytes",
		},
		// Talos creates this partition at boot and weights it against the other roots; without an
		// explicit partition libvirt would place the domain in its own /machine default, outside
		// the tree Talos tracks and kills on shutdown.
		Resource: &libvirtxml.DomainResource{
			Partition: "/" + constants.CgroupVirtualMachines,
		},
		OS: &libvirtxml.DomainOS{
			Type: &libvirtxml.DomainOSType{
				Type:    "hvm",
				Arch:    platform.QEMUArch(),
				Machine: profile.Machine,
			},
		},
		// Omitting the balloon would let libvirt add one by default.
		Devices: &libvirtxml.DomainDeviceList{
			MemBalloon: &libvirtxml.DomainMemBalloon{
				Model: "none",
			},
			Interfaces: interfaces,
		},
	}

	domain.CPU = &libvirtxml.DomainCPU{
		Mode: profile.CPU.Mode.String(),
	}

	if profile.CPU.Mode == hypervisorhelpers.VirtualMachineProfileCPUModeCustom {
		fallback := profile.CPU.Fallback
		if fallback == "" {
			fallback = "forbid"
		}

		domain.CPU.Match = "exact"
		domain.CPU.Model = &libvirtxml.DomainCPUModel{
			Value:    profile.CPU.Model,
			Fallback: fallback,
		}

		for _, feature := range profile.CPU.Features {
			domain.CPU.Features = append(domain.CPU.Features, libvirtxml.DomainCPUFeature{
				Name:   feature.Name,
				Policy: feature.Policy.String(),
			})
		}
	}

	domain.Clock = platform.Clock(profile.Timers)

	if topology := spec.CPU.Topology; topology != nil {
		domain.CPU.Topology = &libvirtxml.DomainCPUTopology{
			Sockets: int(topology.Sockets),
			Cores:   int(topology.Cores),
			Threads: int(topology.Threads),
		}
	}

	if spec.Memory.Ballooning.Enabled {
		domain.Devices.MemBalloon.Model = "virtio"
	}

	cputune, err := renderVirtualMachineCPUTune(name, spec.CPU)
	if err != nil {
		return nil, nil, err
	}

	domain.CPUTune = cputune

	if spec.Memory.NUMA != nil {
		nodes, err := canonicalHostIDList(name, "NUMA nodes", spec.Memory.NUMA.Nodes, hypervisorhelpers.MaxHostNUMANodeID)
		if err != nil {
			return nil, nil, err
		}

		// The mode is always written, even the default: libvirt's own default is also strict, but
		// the rendered definition should not depend on that.
		domain.NUMATune = &libvirtxml.DomainNUMATune{
			Memory: &libvirtxml.DomainNUMATuneMemory{
				Mode:    spec.Memory.NUMA.Mode,
				Nodeset: nodes,
			},
		}
	}

	renderVirtualMachineFirmware(&domain, spec.Firmware)

	renderVirtualMachineConsole(&domain, platform, spec.Console, spec.Video)

	renderVirtualMachineGuest(&domain, spec.Guest)

	attachedDisks, err := renderVirtualMachineDisks(&domain, platform, name, spec.Disks, input.Disks)
	if err != nil {
		return nil, nil, err
	}

	if input.SeedPath != "" {
		if err := attachSeed(&domain, platform, name, input.SeedPath); err != nil {
			return nil, nil, err
		}
	}

	return &domain, attachedDisks, nil
}

func renderVirtualMachineConsole(domain *libvirtxml.Domain, platform Platform, console hypervisor.VirtualMachineConsoleSpec, video hypervisor.VirtualMachineVideoSpec) {
	if console.Serial {
		// libvirt allocates the PTY; no host device path is prescribed.
		// The definition adapter adds capture after assigning the host-specific UUID.
		domain.Devices.Serials = []libvirtxml.DomainSerial{
			{
				Source: &libvirtxml.DomainChardevSource{
					Pty: &libvirtxml.DomainChardevSourcePty{},
				},
			},
		}
	}

	if console.VNC {
		// libvirt allocates a Unix socket; never open a host TCP listener or prescribe a path.
		domain.Devices.Graphics = []libvirtxml.DomainGraphic{
			{
				VNC: &libvirtxml.DomainGraphicVNC{
					Listeners: []libvirtxml.DomainGraphicListener{{Socket: &libvirtxml.DomainGraphicListenerSocket{}}},
				},
			},
		}

		platform.Video(domain, video)
	}
}

func renderVirtualMachineGuest(domain *libvirtxml.Domain, guest hypervisor.VirtualMachineGuestSpec) {
	if !guest.Agent.Enabled {
		return
	}

	// mode="bind" is required: without it libvirt renders the channel but does not open a host
	// endpoint, and the guest agent appears disconnected.
	domain.Devices.Channels = append(domain.Devices.Channels, libvirtxml.DomainChannel{
		Source: &libvirtxml.DomainChardevSource{
			UNIX: &libvirtxml.DomainChardevSourceUNIX{
				Mode: "bind",
			},
		},
		Target: &libvirtxml.DomainChannelTarget{
			VirtIO: &libvirtxml.DomainChannelTargetVirtIO{
				Name: "org.qemu.guest_agent.0",
			},
		},
	})
}

func renderVirtualMachineFirmware(domain *libvirtxml.Domain, firmware hypervisor.VirtualMachineFirmwareSpec) {
	// Every domain declares ACPI: it carries a power button press into the guest, and QEMU rejects
	// UEFI domains without it on supported architectures.
	domain.Features = &libvirtxml.DomainFeatureList{
		ACPI: &libvirtxml.DomainFeature{},
	}

	if firmware.Type != "uefi" {
		// Omit firmware autoselection: the libvirt extension runs legacy BIOS
		// domains without a firmware descriptor.
		return
	}

	domain.OS.Firmware = "efi"

	enabled := libvirtYesNo(firmware.SecureBoot)

	domain.OS.FirmwareInfo = &libvirtxml.DomainOSFirmwareInfo{
		Features: []libvirtxml.DomainOSFirmwareFeature{
			{Name: "secure-boot", Enabled: enabled},
			{Name: "enrolled-keys", Enabled: enabled},
		},
	}
}

// renderVirtualMachineCPUTune builds the one cputune element the quota and the pins share, or nil
// when none of them is set.
func renderVirtualMachineCPUTune(name string, cpu hypervisor.VirtualMachineCPUSpec) (*libvirtxml.DomainCPUTune, error) {
	var cputune libvirtxml.DomainCPUTune

	if cpu.Limit > 0 {
		// One core is the whole period, so the quota is the period scaled by cores. The limit was
		// validated against hypervisorhelpers.MaxCPULimitMillicores, which keeps the product within the
		// schema's cpuquota range and therefore within int64.
		quota := cpu.Limit * hypervisorhelpers.CPUQuotaPeriod / 1000

		// global_quota bounds the whole domain (vCPUs and emulator together), unlike quota which
		// is enforced per vCPU thread.
		cputune.GlobalPeriod = &libvirtxml.DomainCPUTunePeriod{Value: hypervisorhelpers.CPUQuotaPeriod}
		cputune.GlobalQuota = &libvirtxml.DomainCPUTuneQuota{Value: int64(quota)}
	}

	for _, pin := range cpu.Pins {
		cpus, err := canonicalHostIDList(name, fmt.Sprintf("vCPU %d pin", pin.VCPU), pin.CPUs, hypervisorhelpers.MaxHostCPUID)
		if err != nil {
			return nil, err
		}

		cputune.VCPUPin = append(cputune.VCPUPin, libvirtxml.DomainCPUTuneVCPUPin{
			VCPU:   uint(pin.VCPU),
			CPUSet: cpus,
		})
	}

	if cpu.EmulatorPin != "" {
		cpus, err := canonicalHostIDList(name, "emulator pin", cpu.EmulatorPin, hypervisorhelpers.MaxHostCPUID)
		if err != nil {
			return nil, err
		}

		cputune.EmulatorPin = &libvirtxml.DomainCPUTuneEmulatorPin{CPUSet: cpus}
	}

	if cputune.GlobalQuota == nil && len(cputune.VCPUPin) == 0 && cputune.EmulatorPin == nil {
		return nil, nil
	}

	return &cputune, nil
}

func renderVirtualMachineInterfaces(name string, interfaces []hypervisor.VirtualMachineInterfaceSpec, resolved []string) []libvirtxml.DomainInterface {
	result := make([]libvirtxml.DomainInterface, 0, len(interfaces))

	for i, iface := range interfaces {
		domainInterface := libvirtxml.DomainInterface{
			MAC: &libvirtxml.DomainInterfaceMAC{
				Address: interfaceHardwareAddr(name, iface),
			},
			Source: renderInterfaceSource(iface, resolved[i]),
			Model: &libvirtxml.DomainInterfaceModel{
				Type: "virtio",
			},
			// The user alias carries the configured interface name onto the device, so it can be
			// addressed without depending on the generated macvtap name.
			Alias: &libvirtxml.DomainAlias{
				Name: interfaceAliasPrefix + iface.Name,
			},
		}

		// libvirt applies the guest's receive filter updates to the macvtap: the multicast list
		// (needed for NDP and DHCPv6), the promiscuous and all-multicast flags, which propagate to
		// the lower link, and the MAC address, which the guest can set to anything.
		if iface.TrustGuestRxFilters {
			domainInterface.TrustGuestRXFilters = "yes"
		}

		result = append(result, domainInterface)
	}

	return result
}

// renderInterfaceSource attaches an interface to its resolved host link.
func renderInterfaceSource(iface hypervisor.VirtualMachineInterfaceSpec, link string) *libvirtxml.DomainInterfaceSource {
	if iface.Attachment == hypervisorhelpers.VirtualMachineInterfaceAttachmentBridge.String() {
		// libvirt creates a tap device and adds it to the bridge as a port.
		return &libvirtxml.DomainInterfaceSource{
			Bridge: &libvirtxml.DomainInterfaceSourceBridge{
				Bridge: link,
			},
		}
	}

	// A macvtap in bridge mode lets guests on the same lower link reach each other, and
	// a host macvlan in bridge mode on the same parent reach the guest. The lower link's
	// own address cannot: macvtap traffic never loops back to its parent.
	return &libvirtxml.DomainInterfaceSource{
		Direct: &libvirtxml.DomainInterfaceSourceDirect{
			Dev:  link,
			Mode: "bridge",
		},
	}
}

// interfaceAliasPrefix marks a device alias as user-assigned; libvirt ignores other aliases.
const interfaceAliasPrefix = "ua-"

// interfaceHardwareAddr is the address the guest is given: the configured one, or a derived one
// when the configuration names none.
func interfaceHardwareAddr(vmName string, iface hypervisor.VirtualMachineInterfaceSpec) string {
	if iface.HardwareAddr != "" {
		return iface.HardwareAddr
	}

	return interfaceMAC(vmName, iface.Name)
}

// interfaceMAC derives a stable MAC address for an interface under the QEMU OUI.
func interfaceMAC(vmName, ifaceName string) string {
	sum := sha256.Sum256([]byte(vmName + "\x00" + ifaceName))

	return fmt.Sprintf("52:54:00:%02x:%02x:%02x", sum[0], sum[1], sum[2])
}

// canonicalHostIDList checks that a host ID list is well-formed, bounded and nonempty, and returns
// it in canonical form: that is what the schema's cpuset pattern accepts, whatever the spec wrote.
func canonicalHostIDList(name, what, list string, maxID int) (string, error) {
	set, err := hypervisorhelpers.ParseHostIDList(list, maxID)
	if err != nil {
		return "", fmt.Errorf("virtual machine %q: %s %q: %w", name, what, list, err)
	}

	if set.IsEmpty() {
		return "", fmt.Errorf("virtual machine %q: %s must name at least one host ID", name, what)
	}

	return set.String(), nil
}

// renderVirtualMachineDisks attaches every disk of the virtual machine, in configuration order.
// Reports the IDs of the disk statuses it attached, in the same order, so the definition carries
// what it was rendered from.
func renderVirtualMachineDisks(
	domain *libvirtxml.Domain,
	platform Platform,
	name string,
	disks []hypervisor.VirtualMachineDiskSpec,
	resolvedDisks map[resource.ID]hypervisor.VirtualMachineDiskStatusSpec,
) ([]string, error) {
	devs := targetDevAllocator{}
	attached := make([]string, 0, len(disks))

	for _, disk := range disks {
		// Graded before the disk status is consulted, because every status failure reads as waiting:
		// a disk this slice does not provision would otherwise leave the machine pending forever.
		if err := diskhelpers.CheckSupported(disk); err != nil {
			return nil, fmt.Errorf("virtual machine %q: disk %q: %w", name, disk.Name, err)
		}

		// The ID covers what the disk is provisioned from, so a status left over from another
		// image is simply not found here.
		id := hypervisor.VirtualMachineDiskStatusID(name, disk)

		resolved, found := resolvedDisks[id]

		switch {
		case !found:
			return nil, fmt.Errorf("virtual machine %q: disk %q is %w: no disk status yet", name, disk.Name, ErrDiskNotReady)
		case resolved.Phase != hypervisor.VirtualMachineDiskPhaseReady:
			return nil, fmt.Errorf("virtual machine %q: disk %q is %w: %s", name, disk.Name, ErrDiskNotReady, resolved.Error)
		case resolved.SourcePath == "" && !disk.IsEmptyDrive():
			return nil, fmt.Errorf("virtual machine %q: disk %q is %w: ready without a source path", name, disk.Name, ErrDiskNotReady)
		}

		attached = append(attached, id)

		dev, err := devs.allocate(disk.Bus)
		if err != nil {
			return nil, fmt.Errorf("virtual machine %q: disk %q: %w", name, disk.Name, err)
		}

		appendVirtualMachineDisk(domain, platform, disk, resolved, dev)
	}

	return attached, nil
}

func appendVirtualMachineDisk(
	domain *libvirtxml.Domain,
	platform Platform,
	disk hypervisor.VirtualMachineDiskSpec,
	resolved hypervisor.VirtualMachineDiskStatusSpec,
	dev string,
) {
	device := "disk"
	if disk.Type == hypervisorhelpers.VirtualMachineDiskTypeCDROM.String() {
		device = "cdrom"
	}

	rendered := libvirtxml.DomainDisk{
		Device: device,
		Driver: &libvirtxml.DomainDiskDriver{
			Name: "qemu",
			Type: resolved.Format,
		},
		Target: &libvirtxml.DomainDiskTarget{
			Dev: dev,
			Bus: disk.Bus,
		},
	}

	if !disk.IsEmptyDrive() {
		rendered.Source = &libvirtxml.DomainDiskSource{
			File: &libvirtxml.DomainDiskSourceFile{
				File: resolved.SourcePath,
			},
		}
	}

	if resolved.ReadOnly {
		rendered.ReadOnly = &libvirtxml.DomainDiskReadOnly{}
	} else {
		// A writable disk is sparse in a pool nothing accounts for, so it can hit ENOSPC with
		// the pool's filesystem full. Pausing the guest is recoverable; letting it see a failed
		// write is how a qcow2 loses the L2 table it was growing.
		rendered.Driver.ErrorPolicy = "stop"
		rendered.Driver.RErrorPolicy = "stop"
	}

	// Talos renders no <os><boot dev>, so per-device boot elements are free to use; libvirt
	// rejects a domain that mixes the two.
	if disk.BootOrder > 0 {
		rendered.Boot = &libvirtxml.DomainDeviceBoot{Order: uint(disk.BootOrder)}
	}

	platform.DiskController(domain, disk.Bus)

	domain.Devices.Disks = append(domain.Devices.Disks, rendered)
}

// targetDevAllocator hands out the guest device names libvirt requires to be unique.
//
// Buses sharing a driver share a sequence: scsi and sata both present sd*, so a domain with one of
// each must not name both sda.
type targetDevAllocator map[string]int

func (a targetDevAllocator) allocate(bus string) (string, error) {
	var prefix string

	switch bus {
	case hypervisorhelpers.VirtualMachineDiskBusVirtio.String():
		prefix = "vd"
	case hypervisorhelpers.VirtualMachineDiskBusSCSI.String(), hypervisorhelpers.VirtualMachineDiskBusSATA.String():
		prefix = "sd"
	default:
		// libvirt has no nvme disk bus: an NVMe namespace is a source type or a controller, not a
		// <target bus>. Refuse rather than render a domain libvirt will not accept.
		return "", fmt.Errorf("unsupported bus %q", bus)
	}

	index := a[prefix]
	if index >= 26 {
		// Past sdz libvirt continues sdaa, sdab and so on. A guest with 27 disks on one driver is
		// not a case this supports yet.
		return "", fmt.Errorf("too many disks on the %q bus", bus)
	}

	a[prefix] = index + 1

	return prefix + string(rune('a'+index)), nil
}

// libvirtYesNo formats a bool as libvirt's yes/no attribute value.
func libvirtYesNo(value bool) string {
	if value {
		return "yes"
	}

	return "no"
}

// SATA and SCSI share sd* targets; choose a free target without changing user boot order.
func freeCloudInitSATATarget(domain *libvirtxml.Domain) string {
	used := make(map[string]struct{}, len(domain.Devices.Disks))

	for _, disk := range domain.Devices.Disks {
		if disk.Target != nil {
			used[disk.Target.Dev] = struct{}{}
		}
	}

	for letter := 'a'; letter <= 'z'; letter++ {
		candidate := "sd" + string(letter)
		if _, exists := used[candidate]; !exists {
			return candidate
		}
	}

	return ""
}

func appendCloudInitCDROM(domain *libvirtxml.Domain, platform Platform, path, dev string) {
	bus := platform.SeedBus(domain)
	domain.Devices.Disks = append(domain.Devices.Disks, libvirtxml.DomainDisk{
		Device: "cdrom",
		Driver: &libvirtxml.DomainDiskDriver{
			Name: "qemu",
			Type: hypervisorhelpers.VirtualMachineDiskFormatRaw.String(),
		},
		Source: &libvirtxml.DomainDiskSource{
			File: &libvirtxml.DomainDiskSourceFile{
				File: path,
			},
		},
		Target: &libvirtxml.DomainDiskTarget{
			Dev: dev,
			Bus: bus,
		},
		ReadOnly: &libvirtxml.DomainDiskReadOnly{},
	})
}

// attachSeed adds a readonly CD-ROM without a boot order to the constructed domain.
func attachSeed(domain *libvirtxml.Domain, platform Platform, name, path string) error {
	dev := freeCloudInitSATATarget(domain)
	if dev == "" {
		return fmt.Errorf("virtual machine %q: cloud-init seed: %w: no free CD-ROM target", name, diskhelpers.ErrDiskUnsupported)
	}

	appendCloudInitCDROM(domain, platform, path, dev)

	return nil
}
