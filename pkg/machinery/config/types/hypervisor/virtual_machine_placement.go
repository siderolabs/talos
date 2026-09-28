// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

//docgen:jsonschema

import (
	"errors"
	"fmt"

	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
)

// Check interfaces.
var (
	_ config.VirtualMachineCPUTopologyConfig = &VirtualMachineCPUTopology{}
	_ yaml.IsZeroer                          = VirtualMachineCPUTopology{}
	_ config.VirtualMachineCPUPinningConfig  = &VirtualMachineCPUPinning{}
	_ config.VirtualMachineVCPUPinConfig     = &VirtualMachineVCPUPin{}
	_ config.VirtualMachineNUMAConfig        = &VirtualMachineNUMA{}
	_ yaml.IsZeroer                          = VirtualMachineCPUPinning{}
)

// VirtualMachineCPUTopology describes guest CPU geometry and independent host CPU pinning.
type VirtualMachineCPUTopology struct {
	//   description: |
	//     Number of guest sockets. If any geometry dimension is set, all three must be
	//     positive and sockets * cores * threads must equal `cpu.count`.
	TopologySockets *uint32 `yaml:"sockets,omitempty"`
	//   description: |
	//     Number of guest cores per socket, not host CPU IDs.
	TopologyCores *uint32 `yaml:"cores,omitempty"`
	//   description: |
	//     Number of guest threads per core, not host CPU IDs.
	TopologyThreads *uint32 `yaml:"threads,omitempty"`
	//   description: |
	//     Host CPUs the guest's threads are pinned to; independent of guest geometry and `cpu.limit`.
	//     Optional; omitting it leaves the virtual machine schedulable on any host CPU.
	PinningConfig VirtualMachineCPUPinning `yaml:"pinning,omitempty"`
}

// IsZero implements yaml.IsZeroer.
func (t VirtualMachineCPUTopology) IsZero() bool {
	return t.TopologySockets == nil && t.TopologyCores == nil && t.TopologyThreads == nil && t.PinningConfig.IsZero()
}

// Sockets implements config.VirtualMachineCPUTopologyConfig interface.
func (t *VirtualMachineCPUTopology) Sockets() uint32 {
	if t.TopologySockets == nil {
		return 0
	}

	return *t.TopologySockets
}

// Cores implements config.VirtualMachineCPUTopologyConfig interface.
func (t *VirtualMachineCPUTopology) Cores() uint32 {
	if t.TopologyCores == nil {
		return 0
	}

	return *t.TopologyCores
}

// Threads implements config.VirtualMachineCPUTopologyConfig interface.
func (t *VirtualMachineCPUTopology) Threads() uint32 {
	if t.TopologyThreads == nil {
		return 0
	}

	return *t.TopologyThreads
}

// Pinning implements config.VirtualMachineCPUTopologyConfig interface.
func (t *VirtualMachineCPUTopology) Pinning() config.VirtualMachineCPUPinningConfig {
	return &t.PinningConfig
}

func (t *VirtualMachineCPUTopology) validate(count uint32) error {
	var geometryError error

	if t.TopologySockets != nil || t.TopologyCores != nil || t.TopologyThreads != nil {
		geometryError = t.validateGeometry(count)
	}

	return errors.Join(geometryError, t.PinningConfig.validate(count))
}

func (t *VirtualMachineCPUTopology) validateGeometry(count uint32) error {
	if t.TopologySockets == nil || t.TopologyCores == nil || t.TopologyThreads == nil {
		return errors.New("cpu.topology: sockets, cores and threads must all be specified")
	}

	if t.Sockets() == 0 || t.Cores() == 0 || t.Threads() == 0 {
		return errors.New("cpu.topology: sockets, cores and threads must all be positive")
	}

	// Bound the two-factor product by count before multiplying by threads to avoid overflow.
	cores := uint64(t.Sockets()) * uint64(t.Cores())
	if cores > uint64(count) || cores*uint64(t.Threads()) != uint64(count) {
		return errors.New("cpu.topology: sockets * cores * threads must equal cpu.count")
	}

	return nil
}

// VirtualMachineCPUPinning describes which host CPUs the guest's threads are pinned to.
//
// Host CPU IDs are the kernel's logical CPU numbers, SMT threads included. Whether a named CPU
// exists and is available is only known on the host, when the virtual machine starts.
type VirtualMachineCPUPinning struct {
	//   description: |
	//     Per-vCPU pins.
	//
	//     A vCPU left out of this list is scheduled on any host CPU.
	//
	//     A configuration patch replaces this list as a whole rather than appending to it.
	VCPUsConfig []VirtualMachineVCPUPin `yaml:"vcpus,omitempty" merge:"replace"`
	//   description: |
	//     Host CPUs the emulator threads (everything of the virtual machine that is not a vCPU)
	//     are pinned to, as a Linux CPU list, e.g. `0-1,4`.
	//
	//     Optional; omitting it leaves the emulator threads unpinned.
	//   examples:
	//     - value: '"0-1"'
	EmulatorConfig string `yaml:"emulator,omitempty"`
}

// VirtualMachineVCPUPin pins one guest vCPU to a set of host CPUs.
type VirtualMachineVCPUPin struct {
	//   description: |
	//     Guest vCPU index, starting at 0 and below `cpu.count`.
	//   examples:
	//     - value: 0
	//   schemaRequired: true
	PinVCPU uint32 `yaml:"vcpu"`
	//   description: |
	//     Host CPUs the vCPU is pinned to, as a Linux CPU list, e.g. `8` or `9-10`.
	//   examples:
	//     - value: '"9-10"'
	//   schemaRequired: true
	PinCPUs string `yaml:"cpus"`
}

// VirtualMachineNUMA describes where the guest memory is placed on the host.
type VirtualMachineNUMA struct {
	//   description: |
	//     How guest memory is bound to the nodes.
	//
	//     `strict` fails allocations that cannot be served from `nodes`; `preferred` falls back
	//     to other nodes; `interleave` spreads pages across `nodes` round-robin.
	//
	//     Optional; defaults to `strict`.
	//   values:
	//     - strict
	//     - preferred
	//     - interleave
	NUMAMode hypervisorhelpers.VirtualMachineNUMAMode `yaml:"mode,omitempty"`
	//   description: |
	//     Host NUMA nodes the guest memory is placed on, as a Linux node list, e.g. `1` or `0-1`.
	//
	//     Whether a named node exists is only known on the host, when the virtual machine starts.
	//   examples:
	//     - value: '"1"'
	//   schemaRequired: true
	NUMANodes string `yaml:"nodes"`
}

// IsZero implements yaml.IsZeroer.
func (p VirtualMachineCPUPinning) IsZero() bool {
	return len(p.VCPUsConfig) == 0 && p.EmulatorConfig == ""
}

// VCPUs implements config.VirtualMachineCPUPinningConfig interface.
func (p *VirtualMachineCPUPinning) VCPUs() []config.VirtualMachineVCPUPinConfig {
	out := make([]config.VirtualMachineVCPUPinConfig, 0, len(p.VCPUsConfig))

	for i := range p.VCPUsConfig {
		out = append(out, &p.VCPUsConfig[i])
	}

	return out
}

// Emulator implements config.VirtualMachineCPUPinningConfig interface.
func (p *VirtualMachineCPUPinning) Emulator() string {
	return canonicalHostIDList(p.EmulatorConfig, hypervisorhelpers.MaxHostCPUID)
}

// VCPU implements config.VirtualMachineVCPUPinConfig interface.
func (p *VirtualMachineVCPUPin) VCPU() uint32 {
	return p.PinVCPU
}

// CPUs implements config.VirtualMachineVCPUPinConfig interface.
func (p *VirtualMachineVCPUPin) CPUs() string {
	return canonicalHostIDList(p.PinCPUs, hypervisorhelpers.MaxHostCPUID)
}

// Mode implements config.VirtualMachineNUMAConfig interface.
func (n *VirtualMachineNUMA) Mode() hypervisorhelpers.VirtualMachineNUMAMode {
	if n.NUMAMode == hypervisorhelpers.VirtualMachineNUMAModeUnknown {
		return hypervisorhelpers.VirtualMachineNUMAModeStrict
	}

	return n.NUMAMode
}

// Nodes implements config.VirtualMachineNUMAConfig interface.
func (n *VirtualMachineNUMA) Nodes() string {
	return canonicalHostIDList(n.NUMANodes, hypervisorhelpers.MaxHostNUMANodeID)
}

// canonicalHostIDList returns the list in canonical form, or as written when it does not parse:
// validation has already rejected such a value, so nothing downstream relies on it.
func canonicalHostIDList(list string, maxID int) string {
	set, err := hypervisorhelpers.ParseHostIDList(list, maxID)
	if err != nil {
		return list
	}

	return set.String()
}

// validate checks the pins against the number of vCPUs the guest has.
func (p *VirtualMachineCPUPinning) validate(count uint32) error {
	var validationErrors error

	vcpus := map[uint32]struct{}{}

	for i, pin := range p.VCPUsConfig {
		// A pin past the vCPU count is only reported when there is a count to compare against;
		// the missing count is reported on its own.
		if count != 0 && pin.PinVCPU >= count {
			validationErrors = errors.Join(validationErrors,
				fmt.Errorf("cpu.topology.pinning.vcpus[%d]: vcpu %d must be less than cpu.count %d", i, pin.PinVCPU, count))
		}

		if _, exists := vcpus[pin.PinVCPU]; exists {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("cpu.topology.pinning.vcpus[%d]: duplicate vcpu %d", i, pin.PinVCPU))
		}

		vcpus[pin.PinVCPU] = struct{}{}

		validationErrors = errors.Join(validationErrors,
			validateHostIDList(fmt.Sprintf("cpu.topology.pinning.vcpus[%d].cpus", i), pin.PinCPUs, hypervisorhelpers.MaxHostCPUID))
	}

	if p.EmulatorConfig != "" {
		validationErrors = errors.Join(validationErrors,
			validateHostIDList("cpu.topology.pinning.emulator", p.EmulatorConfig, hypervisorhelpers.MaxHostCPUID))
	}

	return validationErrors
}

// validate checks the NUMA placement.
func (n *VirtualMachineNUMA) validate() error {
	var validationErrors error

	if n.NUMAMode != hypervisorhelpers.VirtualMachineNUMAModeUnknown && !n.NUMAMode.IsAVirtualMachineNUMAMode() {
		validationErrors = errors.Join(validationErrors,
			fmt.Errorf("unsupported memory.numa.mode %q, expected %s", n.NUMAMode,
				expectedValues(hypervisorhelpers.VirtualMachineNUMAModeStrings())))
	}

	return errors.Join(validationErrors, validateHostIDList("memory.numa.nodes", n.NUMANodes, hypervisorhelpers.MaxHostNUMANodeID))
}

// validateHostIDList checks a required host ID list: it must name at least one ID, every one of
// them within the software bound.
func validateHostIDList(path, list string, maxID int) error {
	if list == "" {
		return fmt.Errorf("%s is required", path)
	}

	if _, err := hypervisorhelpers.ParseHostIDList(list, maxID); err != nil {
		return fmt.Errorf("%s %q: %w", path, list, err)
	}

	return nil
}
