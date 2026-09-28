// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"
	"libvirt.org/go/libvirtxml"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// VirtualMachineDomainSpecController renders backend-neutral specs into libvirt XML.
// It reads no machine configuration and performs no libvirt operations.
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
			Type:      hypervisor.VirtualMachineDomainSpecType,
			Kind:      controller.InputDestroyReady,
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
func (ctrl *VirtualMachineDomainSpecController) Run(ctx context.Context, runtime controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-runtime.EventCh():
		}

		if err := ctrl.reconcile(ctx, runtime); err != nil {
			return err
		}

		runtime.ResetRestartBackoff()
	}
}

func (ctrl *VirtualMachineDomainSpecController) reconcile(ctx context.Context, runtime controller.Runtime) error {
	specs, err := safe.ReaderListAll[*hypervisor.VirtualMachineSpec](ctx, runtime)
	if err != nil {
		return fmt.Errorf("failed to list virtual machine specs: %w", err)
	}

	desired := make(map[string]struct{}, specs.Len())

	var errs []error

	for vm := range specs.All() {
		name := vm.Metadata().ID()
		desired[name] = struct{}{}

		// COSI tracks attempted modifications even when the callback fails. Keep
		// validation inside the callback so an invalid update preserves the last
		// good domain without preventing unrelated renders and output cleanup.
		if err := safe.WriterModify(ctx, runtime,
			hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, name),
			func(res *hypervisor.VirtualMachineDomainSpec) error {
				domainXML, renderErr := renderVirtualMachineDomain(name, vm.TypedSpec())
				if renderErr != nil {
					return renderErr
				}

				*res.TypedSpec() = hypervisor.VirtualMachineDomainSpecSpec{
					DomainXML:  domainXML,
					PowerState: vm.TypedSpec().PowerState,
				}

				return nil
			},
		); err != nil {
			errs = append(errs, fmt.Errorf("failed to write virtual machine domain spec %q: %w", name, err))
		}
	}

	return errors.Join(append(errs, ctrl.cleanupDomains(ctx, runtime, desired))...)
}

func (ctrl *VirtualMachineDomainSpecController) cleanupDomains(ctx context.Context, runtime controller.Runtime, desired map[string]struct{}) error {
	domains, err := safe.ReaderListAll[*hypervisor.VirtualMachineDomainSpec](ctx, runtime)
	if err != nil {
		return fmt.Errorf("failed to list virtual machine domain specs: %w", err)
	}

	for domain := range domains.All() {
		if _, wanted := desired[domain.Metadata().ID()]; wanted && domain.Metadata().Phase() == resource.PhaseRunning {
			continue
		}

		ready, teardownErr := runtime.Teardown(ctx, domain.Metadata())
		if teardownErr != nil {
			return fmt.Errorf("failed to tear down virtual machine domain spec %q: %w", domain.Metadata().ID(), teardownErr)
		}

		if !ready {
			continue
		}

		if destroyErr := runtime.Destroy(ctx, domain.Metadata()); destroyErr != nil {
			return fmt.Errorf("failed to destroy virtual machine domain spec %q: %w", domain.Metadata().ID(), destroyErr)
		}
	}

	return nil
}

func renderVirtualMachineDomain(name string, spec *hypervisor.VirtualMachineSpecSpec) (string, error) {
	if err := validateVirtualMachineDomainSpec(name, spec); err != nil {
		return "", err
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
		},
	}

	if topology := spec.CPU.Topology; topology != nil {
		domain.CPU = &libvirtxml.DomainCPU{
			Topology: &libvirtxml.DomainCPUTopology{
				Sockets: int(topology.Sockets),
				Cores:   int(topology.Cores),
				Threads: int(topology.Threads),
			},
		}
	}

	if spec.Memory.Ballooning.Enabled {
		domain.Devices.MemBalloon.Model = "virtio"
	}

	cputune, err := renderVirtualMachineCPUTune(name, spec.CPU)
	if err != nil {
		return "", err
	}

	domain.CPUTune = cputune

	if spec.Memory.NUMA != nil {
		nodes, err := canonicalHostIDList(name, "NUMA nodes", spec.Memory.NUMA.Nodes, hypervisorhelpers.MaxHostNUMANodeID)
		if err != nil {
			return "", err
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

	if spec.Console.Serial {
		// libvirt allocates the PTY; no host device path is prescribed.
		domain.Devices.Serials = []libvirtxml.DomainSerial{
			{
				Source: &libvirtxml.DomainChardevSource{
					Pty: &libvirtxml.DomainChardevSourcePty{},
				},
			},
		}
	}

	if spec.Console.VNC {
		// Bind locally; a remote graphics endpoint requires a separate access policy.
		domain.Devices.Graphics = []libvirtxml.DomainGraphic{
			{
				VNC: &libvirtxml.DomainGraphicVNC{
					AutoPort: "yes",
					Port:     -1,
					Listen:   "127.0.0.1",
				},
			},
		}
	}

	domainXML, err := domain.Marshal()
	if err != nil {
		return "", fmt.Errorf("failed to marshal virtual machine %q: %w", name, err)
	}

	return domainXML, nil
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

	// Pool and library references are logical names, not resolved volume paths.
	// Rendering fewer disks than requested would silently alter the VM definition.
	if len(spec.Disks) != 0 {
		return fmt.Errorf("virtual machine %q: unresolved disks require provisioned volume sources", name)
	}

	return nil
}
