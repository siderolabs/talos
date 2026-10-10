// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"libvirt.org/go/libvirtxml"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// VirtualMachineDomainSpecController renders backend-neutral specs into libvirt XML.
// It reads no machine configuration and performs no libvirt operations; host links are
// observed only to resolve interface link names and aliases.
type VirtualMachineDomainSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *VirtualMachineDomainSpecController) Name() string {
	return "hypervisor.VirtualMachineDomainSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *VirtualMachineDomainSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineDiskStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.CloudInitSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.CloudInitStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.ContentLibraryStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineDomainSpecType,
			Kind:      controller.InputDestroyReady,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.LinkStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *VirtualMachineDomainSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hypervisor.VirtualMachineDomainSpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *VirtualMachineDomainSpecController) Run(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-runtime.EventCh():
		}

		if err := ctrl.reconcile(ctx, runtime, logger); err != nil {
			return err
		}

		runtime.ResetRestartBackoff()
	}
}

func (ctrl *VirtualMachineDomainSpecController) reconcile(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger) error {
	specs, err := safe.ReaderListAll[*hypervisor.VirtualMachineSpec](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list virtual machine specs: %w", err)
	}

	linkStatuses, err := safe.ReaderListAll[*network.LinkStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list link statuses: %w", err)
	}

	links := newHostLinks(linkStatuses)

	resolvedDisks, err := listResolvedDisks(ctx, r)
	if err != nil {
		return err
	}

	desired := make(map[resource.ID]struct{}, specs.Len())

	var errs []error

	for vm := range specs.All() {
		name := vm.Metadata().ID()
		desired[name] = struct{}{}

		// COSI tracks attempted modifications even when the callback fails. Keep
		// validation inside the callback so an invalid update preserves the last
		// good domain without preventing unrelated renders and output cleanup.
		if err := safe.WriterModify(
			ctx, r,
			hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, name),
			func(res *hypervisor.VirtualMachineDomainSpec) error {
				domainXML, attachedDisks, seedID, renderErr := renderVirtualMachineDomainWithSeed(ctx, r, name, vm.TypedSpec(), links, resolvedDisks)
				if renderErr != nil {
					if res.TypedSpec().DomainXML == "" {
						// Nothing was ever defined, so there is nothing to stop.
						return renderErr
					}

					// The config validated but cannot be applied; the disks stay as they are,
					// held for the definition which may still be running.
					res.TypedSpec().PowerState = hypervisorhelpers.PowerStateStopped.String()

					logger.Error("stopping virtual machine: spec cannot be rendered",
						zap.String("virtual_machine", name), zap.Error(renderErr))

					return nil
				}

				*res.TypedSpec() = hypervisor.VirtualMachineDomainSpecSpec{
					DomainXML:  domainXML,
					PowerState: vm.TypedSpec().PowerState,
					Disks:      attachedDisks,
					CloudInit:  seedID,
				}

				return nil
			},
		); err != nil {
			if isHeldBack(err) {
				logger.Info("virtual machine is held back by its host resources", zap.Error(err))

				continue
			}

			errs = append(errs, fmt.Errorf("failed to write virtual machine domain spec %q: %w", name, err))
		}
	}

	// VirtualMachineController holds a finalizer on every domain spec it has claimed, so an unwanted
	// spec is torn down to ask for that hold back, and destroyed only once it comes.
	return errors.Join(append(errs,
		cleanupOutputs[*hypervisor.VirtualMachineDomainSpec](ctx, r, "virtual machine domain spec", desired))...)
}

// renderVirtualMachineDomainWithSeed appends the projected seed to a valid base domain.
func renderVirtualMachineDomainWithSeed(
	ctx context.Context, r controller.Reader, name string, spec *hypervisor.VirtualMachineSpecSpec,
	links hostLinks, resolvedDisks map[resource.ID]hypervisor.VirtualMachineDiskStatusSpec,
) (string, []string, string, error) {
	domainXML, attachedDisks, err := renderVirtualMachineDomain(name, spec, links, resolvedDisks)
	if err != nil || spec.CloudInit == nil {
		return domainXML, attachedDisks, "", err
	}

	seedID, err := attachCloudInit(ctx, r, name, spec.CloudInit, &domainXML)

	return domainXML, attachedDisks, seedID, err
}

// attachCloudInit requires the exact projected seed and its current library backing before rendering.
func attachCloudInit(ctx context.Context, r controller.Reader, name string, desired *hypervisor.VirtualMachineCloudInitSpec, domainXML *string) (string, error) {
	seed := hypervisor.CloudInitSpecSpec{
		Library:       desired.Library,
		MetaData:      desired.MetaData,
		UserData:      desired.UserData,
		NetworkConfig: desired.NetworkConfig,
	}

	if seed.Library == "" || strings.ContainsAny(seed.Library, "/\\\x00") {
		return "", fmt.Errorf("virtual machine %q: invalid cloud-init library", name)
	}

	projected, status, library, id, err := loadCloudInitBacking(ctx, r, name, seed)
	if err != nil {
		return "", err
	}

	asset := status.TypedSpec()
	if !cloudInitProjectionReady(projected, seed) ||
		!cloudInitAssetCurrent(status, seed, projected.Metadata().Version().String(), name) ||
		!cloudInitLibraryMatches(library, asset) || !cloudInitAssetNameValid(asset) {
		return "", fmt.Errorf("virtual machine %q: cloud-init seed is %w", name, errDiskNotReady)
	}

	var domain libvirtxml.Domain
	if err := domain.Unmarshal(*domainXML); err != nil {
		return "", fmt.Errorf("decode domain: %w", err)
	}

	dev := freeCloudInitSATATarget(&domain)
	if dev == "" {
		return "", fmt.Errorf("virtual machine %q: cloud-init seed: %w: no free SATA target", name, errDiskUnsupported)
	}

	appendCloudInitCDROM(&domain, asset, dev)

	*domainXML, err = domain.Marshal()

	return id, err
}

// loadCloudInitBacking reads the projection, its seed status, and the library in dependency order.
func loadCloudInitBacking(
	ctx context.Context, r controller.Reader, name string, seed hypervisor.CloudInitSpecSpec,
) (*hypervisor.CloudInitSpec, *hypervisor.CloudInitStatus, *hypervisor.ContentLibraryStatus, string, error) {
	pending := func() error {
		return fmt.Errorf("virtual machine %q: cloud-init seed is %w", name, errDiskNotReady)
	}

	projected, err := safe.ReaderGetByID[*hypervisor.CloudInitSpec](ctx, r, name)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil, nil, "", pending()
		}

		return nil, nil, nil, "", err
	}

	id := hypervisor.CloudInitStatusID(name, seed)

	status, err := safe.ReaderGetByID[*hypervisor.CloudInitStatus](ctx, r, id)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil, nil, "", pending()
		}

		return nil, nil, nil, "", err
	}

	library, err := safe.ReaderGetByID[*hypervisor.ContentLibraryStatus](ctx, r, seed.Library)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil, nil, "", pending()
		}

		return nil, nil, nil, "", err
	}

	return projected, status, library, id, nil
}

func cloudInitProjectionReady(projected *hypervisor.CloudInitSpec, seed hypervisor.CloudInitSpecSpec) bool {
	return projected.Metadata().Phase() == resource.PhaseRunning && *projected.TypedSpec() == seed
}

func cloudInitAssetCurrent(status *hypervisor.CloudInitStatus, seed hypervisor.CloudInitSpecSpec, generation, name string) bool {
	asset := status.TypedSpec()

	return status.Metadata().Phase() == resource.PhaseRunning && asset.Ready &&
		asset.InputDigest == seed.InputDigest() && asset.ObservedGeneration == generation &&
		asset.VirtualMachine == name && asset.Library == seed.Library
}

func cloudInitLibraryMatches(library *hypervisor.ContentLibraryStatus, asset *hypervisor.CloudInitStatusSpec) bool {
	return library.Metadata().Phase() == resource.PhaseRunning && library.TypedSpec().Ready &&
		asset.Path != "" && asset.VolumeID != "" &&
		asset.Path == library.TypedSpec().Path && asset.VolumeID == library.TypedSpec().VolumeID
}

func cloudInitAssetNameValid(asset *hypervisor.CloudInitStatusSpec) bool {
	return asset.Name != "" && filepath.Base(asset.Name) == asset.Name && asset.Digest != ""
}

// SATA shares sd* targets with SCSI; choose a free target without changing user boot order.
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

func appendCloudInitCDROM(domain *libvirtxml.Domain, asset *hypervisor.CloudInitStatusSpec, dev string) {
	domain.Devices.Disks = append(domain.Devices.Disks, libvirtxml.DomainDisk{
		Device: "cdrom",
		Driver: &libvirtxml.DomainDiskDriver{
			Name: "qemu",
			Type: "raw",
		},
		Source: &libvirtxml.DomainDiskSource{
			File: &libvirtxml.DomainDiskSourceFile{
				File: filepath.Join(asset.Path, asset.Name),
			},
		},
		Target: &libvirtxml.DomainDiskTarget{
			Dev: dev,
			Bus: "sata",
		},
		ReadOnly: &libvirtxml.DomainDiskReadOnly{},
	})
}

// listResolvedDisks indexes the published disk statuses by their resource ID, the key
// renderVirtualMachineDisks looks a disk up under.
func listResolvedDisks(ctx context.Context, reader controller.Reader) (map[resource.ID]hypervisor.VirtualMachineDiskStatusSpec, error) {
	diskStatuses, err := safe.ReaderListAll[*hypervisor.VirtualMachineDiskStatus](ctx, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to list virtual machine disk statuses: %w", err)
	}

	resolvedDisks := make(map[resource.ID]hypervisor.VirtualMachineDiskStatusSpec, diskStatuses.Len())

	for status := range diskStatuses.All() {
		// A status which is tearing down is one a domain has not let go of yet: downstream reads it as absent.
		if status.Metadata().Phase() != resource.PhaseRunning {
			continue
		}

		resolvedDisks[status.Metadata().ID()] = *status.TypedSpec()
	}

	return resolvedDisks, nil
}

//nolint:gocyclo
func renderVirtualMachineDomain(
	name string,
	spec *hypervisor.VirtualMachineSpecSpec,
	links hostLinks,
	resolvedDisks map[resource.ID]hypervisor.VirtualMachineDiskStatusSpec,
) (string, []string, error) {
	if err := validateVirtualMachineDomainSpec(name, spec); err != nil {
		return "", nil, err
	}

	interfaces, err := renderVirtualMachineInterfaces(name, spec.Interfaces, links)
	if err != nil {
		return "", nil, err
	}

	// KVM guests use the host architecture. Leave architecture and machine
	// selection to libvirt rather than hard-coding an x86 machine on arm64.
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
				Type: "hvm",
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
		Mode: "host-model",
	}

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
		return "", nil, err
	}

	domain.CPUTune = cputune

	if spec.Memory.NUMA != nil {
		nodes, err := canonicalHostIDList(name, "NUMA nodes", spec.Memory.NUMA.Nodes, hypervisorhelpers.MaxHostNUMANodeID)
		if err != nil {
			return "", nil, err
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

	renderVirtualMachineConsole(&domain, spec.Console, spec.Video)

	renderVirtualMachineGuest(&domain, spec.Guest)

	attachedDisks, err := renderVirtualMachineDisks(&domain, name, spec.Disks, resolvedDisks)
	if err != nil {
		return "", nil, err
	}

	domainXML, err := domain.Marshal()
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal virtual machine %q: %w", name, err)
	}

	return domainXML, attachedDisks, nil
}

func renderVirtualMachineConsole(domain *libvirtxml.Domain, console hypervisor.VirtualMachineConsoleSpec, video hypervisor.VirtualMachineVideoSpec) {
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

		if runtime.GOARCH == "amd64" {
			// Standard VGA and USB HID work before guest drivers are installed.
			// Leave other architectures' device defaults to libvirt.
			videoModel := video.Model
			if videoModel == "" {
				videoModel = "vga"
			}

			videoDev := libvirtxml.DomainVideo{Model: libvirtxml.DomainVideoModel{Type: videoModel}}
			if video.VRAMMiB > 0 {
				// libvirt's vram attribute is in KiB.
				videoDev.Model.VRam = uint(video.VRAMMiB) * 1024
			}

			domain.Devices.Videos = []libvirtxml.DomainVideo{videoDev}
			domain.Devices.Controllers = append(domain.Devices.Controllers, libvirtxml.DomainController{
				Type: "usb", Model: "qemu-xhci", USB: &libvirtxml.DomainControllerUSB{},
			})
			domain.Devices.Inputs = []libvirtxml.DomainInput{
				{Type: "tablet", Bus: "usb"},
				{Type: "keyboard", Bus: "usb"},
			}
		}
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
	if firmware.Type != "uefi" {
		// Omit firmware autoselection: the libvirt extension runs legacy BIOS
		// domains without a firmware descriptor.
		return
	}

	domain.OS.Firmware = "efi"
	// QEMU rejects UEFI domains without ACPI on supported architectures.
	domain.Features = &libvirtxml.DomainFeatureList{
		ACPI: &libvirtxml.DomainFeature{},
	}

	enabled := "no"
	if firmware.SecureBoot {
		enabled = "yes"
	}

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

// errLinkNotFound marks a render waiting on a host link that has not appeared yet, as opposed
// to one that failed. See the call site in reconcile.
var errLinkNotFound = errors.New("host link not found")

// errLinkNotEthernet marks a host link that exists but cannot carry a macvtap.
var errLinkNotEthernet = errors.New("host link is not an Ethernet link")

// errDiskNotReady marks a render waiting on a disk that has not been resolved or provisioned yet.
var errDiskNotReady = errors.New("not ready")

// errDiskUnsupported marks a disk nothing on the host will ever make attachable. Unlike
// errDiskNotReady, waiting does not help: the configuration has to change.
var errDiskUnsupported = errors.New("unsupported disk")

// isHeldBack reports whether a render failed on something this controller should wait out rather
// than fail on. It is wider than what renderStage grades as pending: a configuration the host
// cannot satisfy is a permanent obstacle to report, but still no reason to restart the controller.
func isHeldBack(err error) bool {
	return errors.Is(err, errLinkNotFound) || errors.Is(err, errLinkNotEthernet) ||
		errors.Is(err, errDiskNotReady) || errors.Is(err, errDiskUnsupported)
}

// hostLinks carries what an interface needs to know about the host's links: how to resolve a
// name or alias, and whether what it resolves to can carry a macvtap.
type hostLinks struct {
	resolver *network.LinkResolver
	types    map[string]nethelpers.LinkType
}

// newHostLinks indexes the observed host links for interface resolution.
func newHostLinks(links safe.List[*network.LinkStatus]) hostLinks {
	types := make(map[string]nethelpers.LinkType, links.Len())

	for link := range links.All() {
		types[link.Metadata().ID()] = link.TypedSpec().Type
	}

	return hostLinks{
		resolver: network.NewLinkResolver(links.All),
		types:    types,
	}
}

// resolveInterfaceLinks resolves the host link of each interface, in order. It fails on the
// first link the host does not have, or has but cannot attach a macvtap to: rendering fewer
// interfaces than requested would silently alter the VM definition, so the whole render fails
// and VirtualMachineDomainSpecController withdraws the power intent.
//
// VirtualMachineStatusController renders the same spec to report the obstacle.
func resolveInterfaceLinks(name string, interfaces []hypervisor.VirtualMachineInterfaceSpec, links hostLinks) ([]string, error) {
	resolved := make([]string, 0, len(interfaces))

	for _, iface := range interfaces {
		link, ok := links.resolver.ResolveChecked(iface.Link)
		if !ok {
			return nil, fmt.Errorf("virtual machine %q: interface %q: %w: %q", name, iface.Name, errLinkNotFound, iface.Link)
		}

		// Only the type is checked, not the kind: a bond or a VLAN is as good a macvtap lower
		// link as a physical interface, while a loopback or an L3 tunnel is not one at all.
		if links.types[link] != nethelpers.LinkEther {
			return nil, fmt.Errorf("virtual machine %q: interface %q: %w: %q", name, iface.Name, errLinkNotEthernet, iface.Link)
		}

		resolved = append(resolved, link)
	}

	return resolved, nil
}

func renderVirtualMachineInterfaces(name string, interfaces []hypervisor.VirtualMachineInterfaceSpec, links hostLinks) ([]libvirtxml.DomainInterface, error) {
	resolved, err := resolveInterfaceLinks(name, interfaces, links)
	if err != nil {
		return nil, err
	}

	var result []libvirtxml.DomainInterface

	for i, iface := range interfaces {
		result = append(result, libvirtxml.DomainInterface{
			MAC: &libvirtxml.DomainInterfaceMAC{
				Address: interfaceHardwareAddr(name, iface),
			},
			// A macvtap in bridge mode lets guests on the same lower link reach each other, and
			// a host macvlan in bridge mode on the same parent reach the guest. The lower link's
			// own address cannot: macvtap traffic never loops back to its parent.
			Source: &libvirtxml.DomainInterfaceSource{
				Direct: &libvirtxml.DomainInterfaceSourceDirect{
					Dev:  resolved[i],
					Mode: "bridge",
				},
			},
			Model: &libvirtxml.DomainInterfaceModel{
				Type: "virtio",
			},
			// The user alias carries the configured interface name onto the device, so it can be
			// addressed without depending on the generated macvtap name.
			Alias: &libvirtxml.DomainAlias{
				Name: interfaceAliasPrefix + iface.Name,
			},
		})
	}

	return result, nil
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

func validateVirtualMachineInterfaces(name string, interfaces []hypervisor.VirtualMachineInterfaceSpec) error {
	names := make(map[string]struct{}, len(interfaces))

	for _, iface := range interfaces {
		if err := hypervisorhelpers.ValidateName(iface.Name); err != nil {
			return fmt.Errorf("virtual machine %q: interface %w", name, err)
		}

		if _, exists := names[iface.Name]; exists {
			return fmt.Errorf("virtual machine %q: duplicate interface name %q", name, iface.Name)
		}

		names[iface.Name] = struct{}{}

		if iface.Link == "" {
			return fmt.Errorf("virtual machine %q: interface %q: link is required", name, iface.Name)
		}

		if iface.HardwareAddr != "" {
			addr, err := net.ParseMAC(iface.HardwareAddr)
			if err != nil {
				return fmt.Errorf("virtual machine %q: interface %q: %w", name, iface.Name, err)
			}

			if len(addr) != 6 || addr[0]&1 != 0 {
				return fmt.Errorf("virtual machine %q: interface %q: hardware address %q must be a 6 byte unicast address",
					name, iface.Name, iface.HardwareAddr)
			}
		}
	}

	return nil
}

func validateVirtualMachineDomainName(name string) error {
	// The domain schema's objectNameWithSlash is [^\n]+; slash is allowed.
	if name == "" || strings.ContainsRune(name, '\n') {
		return fmt.Errorf("virtual machine %q: name must be nonempty and contain no newline", name)
	}

	// libvirtxml replaces invalid UTF-8 and XML 1.0 characters with U+FFFD.
	// Reject those inputs instead of silently changing the resource's identity.
	if !utf8.ValidString(name) {
		return fmt.Errorf("virtual machine %q: name must be valid UTF-8", name)
	}

	for _, char := range name {
		if char != '	' && char != '\r' && (char < 0x20 || char == 0xfffe || char == 0xffff) {
			return fmt.Errorf("virtual machine %q: name contains an invalid XML character", name)
		}
	}

	return nil
}

func validateVirtualMachineFirmware(name string, firmware hypervisor.VirtualMachineFirmwareSpec) error {
	switch firmware.Type {
	case "bios":
		if firmware.SecureBoot {
			return fmt.Errorf("virtual machine %q: secure boot requires UEFI", name)
		}
	case "uefi":
	default:
		return fmt.Errorf("virtual machine %q: unsupported firmware type %q", name, firmware.Type)
	}

	return nil
}

func validateVirtualMachineMemory(name string, memory hypervisor.VirtualMachineMemorySpec) error {
	// On 64-bit libvirt, virDomainParseMemory requires memory below 2^63 bytes.
	// Talos supports only 64-bit targets, so the checked value also fits uint.
	if memory.Size == 0 || memory.Size > 1<<63-1024 {
		return fmt.Errorf("virtual machine %q: memory size must be between 1024 and 9223372036854774784 bytes", name)
	}

	// libvirt stores memory in KiB, even when XML specifies bytes. Reject values
	// that would otherwise be silently rounded up by virDomainParseMemory.
	if memory.Size%1024 != 0 {
		return fmt.Errorf("virtual machine %q: memory size must be a multiple of 1024 bytes", name)
	}

	if memory.NUMA == nil {
		return nil
	}

	switch memory.NUMA.Mode {
	case "strict", "preferred", "interleave":
	default:
		return fmt.Errorf("virtual machine %q: unsupported NUMA mode %q", name, memory.NUMA.Mode)
	}

	return nil
}

func validateVirtualMachineCPU(name string, cpu hypervisor.VirtualMachineCPUSpec) error {
	// The domain schema defines countCPU as a nonzero unsigned short.
	if cpu.Count == 0 || cpu.Count > 65535 {
		return fmt.Errorf("virtual machine %q: CPU count must be between 1 and 65535", name)
	}

	if cpu.Limit != 0 && (cpu.Limit < hypervisorhelpers.MinCPULimitMillicores || cpu.Limit > hypervisorhelpers.MaxCPULimitMillicores) {
		return fmt.Errorf("virtual machine %q: CPU limit must be between %d and %d millicores",
			name, hypervisorhelpers.MinCPULimitMillicores, hypervisorhelpers.MaxCPULimitMillicores)
	}

	return errors.Join(validateVirtualMachineCPUTopology(name, cpu), validateVirtualMachineCPUPins(name, cpu))
}

func validateVirtualMachineCPUTopology(name string, cpu hypervisor.VirtualMachineCPUSpec) error {
	topology := cpu.Topology
	if topology == nil {
		return nil
	}

	if topology.Sockets == 0 || topology.Cores == 0 || topology.Threads == 0 {
		return fmt.Errorf("virtual machine %q: CPU topology sockets, cores and threads must all be positive", name)
	}

	// Check the two-factor product first so multiplying by threads cannot overflow.
	cores := uint64(topology.Sockets) * uint64(topology.Cores)
	if cores > uint64(cpu.Count) || cores*uint64(topology.Threads) != uint64(cpu.Count) {
		return fmt.Errorf("virtual machine %q: CPU topology sockets * cores * threads must equal CPU count %d", name, cpu.Count)
	}

	return nil
}

func validateVirtualMachineCPUPins(name string, cpu hypervisor.VirtualMachineCPUSpec) error {
	pinned := map[uint32]struct{}{}

	for _, pin := range cpu.Pins {
		if pin.VCPU >= cpu.Count {
			return fmt.Errorf("virtual machine %q: pinned vCPU %d must be less than CPU count %d", name, pin.VCPU, cpu.Count)
		}

		if _, exists := pinned[pin.VCPU]; exists {
			return fmt.Errorf("virtual machine %q: vCPU %d is pinned more than once", name, pin.VCPU)
		}

		pinned[pin.VCPU] = struct{}{}
	}

	return nil
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

func validateVirtualMachineDomainSpec(name string, spec *hypervisor.VirtualMachineSpecSpec) error {
	if err := validateVirtualMachineDomainName(name); err != nil {
		return err
	}

	if err := validateVirtualMachineCPU(name, spec.CPU); err != nil {
		return err
	}

	if err := validateVirtualMachineMemory(name, spec.Memory); err != nil {
		return err
	}

	switch spec.PowerState {
	case "running", "stopped", "suspended":
	default:
		return fmt.Errorf("virtual machine %q: unsupported power state %q", name, spec.PowerState)
	}

	if err := validateVirtualMachineFirmware(name, spec.Firmware); err != nil {
		return err
	}

	if err := validateVirtualMachineInterfaces(name, spec.Interfaces); err != nil {
		return err
	}

	return nil
}

// renderVirtualMachineDisks attaches every disk of the virtual machine, in configuration order.
// Reports the IDs of the disk statuses it attached, in the same order, so the definition carries
// what it was rendered from.
func renderVirtualMachineDisks(
	domain *libvirtxml.Domain,
	name string,
	disks []hypervisor.VirtualMachineDiskSpec,
	resolvedDisks map[resource.ID]hypervisor.VirtualMachineDiskStatusSpec,
) ([]string, error) {
	devs := targetDevAllocator{}
	attached := make([]string, 0, len(disks))

	for _, disk := range disks {
		// Graded before the disk status is consulted, because every status failure reads as waiting:
		// a disk this slice does not provision would otherwise leave the machine pending forever.
		if err := checkVirtualMachineDiskSupported(disk); err != nil {
			return nil, fmt.Errorf("virtual machine %q: disk %q: %w", name, disk.Name, err)
		}

		// The ID covers what the disk is provisioned from, so a status left over from another
		// image is simply not found here.
		id := hypervisor.VirtualMachineDiskStatusID(name, disk)

		resolved, found := resolvedDisks[id]

		switch {
		case !found:
			return nil, fmt.Errorf("virtual machine %q: disk %q is %w: no disk status yet", name, disk.Name, errDiskNotReady)
		case !resolved.Ready:
			return nil, fmt.Errorf("virtual machine %q: disk %q is %w: %s", name, disk.Name, errDiskNotReady, resolved.Error)
		}

		attached = append(attached, id)

		dev, err := devs.allocate(disk.Bus)
		if err != nil {
			return nil, fmt.Errorf("virtual machine %q: disk %q: %w", name, disk.Name, err)
		}

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
			Source: &libvirtxml.DomainDiskSource{
				File: &libvirtxml.DomainDiskSourceFile{
					File: resolved.SourcePath,
				},
			},
			Target: &libvirtxml.DomainDiskTarget{
				Dev: dev,
				Bus: disk.Bus,
			},
		}

		if resolved.ReadOnly {
			rendered.ReadOnly = &libvirtxml.DomainDiskReadOnly{}
		}

		// Talos renders no <os><boot dev>, so per-device boot elements are free to use; libvirt
		// rejects a domain that mixes the two.
		if disk.BootOrder > 0 {
			rendered.Boot = &libvirtxml.DomainDeviceBoot{Order: uint(disk.BootOrder)}
		}

		domain.Devices.Disks = append(domain.Devices.Disks, rendered)
	}

	return attached, nil
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
