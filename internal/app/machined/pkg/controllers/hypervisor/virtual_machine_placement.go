// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"k8s.io/utils/cpuset"
	"libvirt.org/go/libvirtxml"

	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// errPlacementPending marks explicit host placement waiting for the host NUMA inventory.
var errPlacementPending = errors.New("host NUMA topology is not available yet")

// errPlacementInvalid marks explicit host placement naming CPUs or NUMA nodes this host cannot use.
var errPlacementInvalid = errors.New("invalid host placement")

// isPlacementHeld reports a start held back by host placement rather than failed.
func isPlacementHeld(err error) bool {
	return errors.Is(err, errPlacementPending) || errors.Is(err, errPlacementInvalid)
}

// hostIDSet is one host CPU or NUMA node list the domain definition names explicitly.
type hostIDSet struct {
	what string
	list string
}

// domainHostPlacement collects every host CPU and NUMA node list the definition binds itself to.
//
//nolint:gocyclo
func domainHostPlacement(domain *libvirtxml.Domain) (cpus, nodes []hostIDSet) {
	if domain.VCPU != nil && domain.VCPU.Placement != "auto" && domain.VCPU.CPUSet != "" {
		cpus = append(cpus, hostIDSet{"vcpu cpuset", domain.VCPU.CPUSet})
	}

	if tune := domain.CPUTune; tune != nil {
		for _, pin := range tune.VCPUPin {
			cpus = append(cpus, hostIDSet{fmt.Sprintf("vCPU %d pin", pin.VCPU), pin.CPUSet})
		}

		if tune.EmulatorPin != nil {
			cpus = append(cpus, hostIDSet{"emulator pin", tune.EmulatorPin.CPUSet})
		}

		for _, pin := range tune.IOThreadPin {
			cpus = append(cpus, hostIDSet{fmt.Sprintf("iothread %d pin", pin.IOThread), pin.CPUSet})
		}
	}

	if tune := domain.NUMATune; tune != nil {
		if tune.Memory != nil && tune.Memory.Placement != "auto" && tune.Memory.Nodeset != "" {
			nodes = append(nodes, hostIDSet{"memory nodeset", tune.Memory.Nodeset})
		}

		for _, node := range tune.MemNodes {
			nodes = append(nodes, hostIDSet{fmt.Sprintf("memnode %d nodeset", node.CellID), node.Nodeset})
		}
	}

	if domain.Devices != nil {
		for i, device := range domain.Devices.Memorydevs {
			if device.Source != nil && device.Source.NodeMask != "" {
				nodes = append(nodes, hostIDSet{fmt.Sprintf("memory device %d nodemask", i), device.Source.NodeMask})
			}
		}
	}

	return cpus, nodes
}

// parseLibvirtHostIDList parses libvirt's cpuset syntax ("0-3,^2"), bounding every ID before
// expanding a range.
func parseLibvirtHostIDList(list string, maxID int) (cpuset.CPUSet, error) {
	included, excluded := cpuset.New(), cpuset.New()

	for item := range strings.SplitSeq(list, ",") {
		item = strings.TrimSpace(item)

		target := &included
		if rest, ok := strings.CutPrefix(item, "^"); ok {
			item, target = strings.TrimSpace(rest), &excluded
		}

		set, err := hypervisorhelpers.ParseHostIDList(item, maxID)
		if err != nil {
			return cpuset.New(), err
		}

		*target = target.Union(set)
	}

	return included.Difference(excluded), nil
}

// validateDomainPlacement checks the host CPUs and NUMA nodes the exact domain definition names
// against the host inventory. A nil topology means the inventory is not published. Selected CPUs
// must be online and selected nodes must exist with memory; vCPUs and memory need not share a node.
//
//nolint:gocyclo
func validateDomainPlacement(name, domainXML string, topology *hardware.NUMATopologySpec) error {
	var domain libvirtxml.Domain

	if err := domain.Unmarshal(domainXML); err != nil {
		return fmt.Errorf("virtual machine %q: invalid domain XML: %w", name, err)
	}

	cpus, nodes := domainHostPlacement(&domain)
	if len(cpus) == 0 && len(nodes) == 0 {
		return nil
	}

	if topology == nil {
		return fmt.Errorf("virtual machine %q: explicit host placement: %w", name, errPlacementPending)
	}

	present, online := uint32Set(topology.PresentCPUs), uint32Set(topology.OnlineCPUs)

	for _, selected := range cpus {
		ids, err := parseLibvirtHostIDList(selected.list, hypervisorhelpers.MaxHostCPUID)
		if err != nil {
			return fmt.Errorf("virtual machine %q: %w: %s %q: %w", name, errPlacementInvalid, selected.what, selected.list, err)
		}

		for _, id := range ids.List() {
			switch {
			case hasID(online, uint32(id)):
			case hasID(present, uint32(id)):
				return fmt.Errorf("virtual machine %q: %w: %s names offline host CPU %d", name, errPlacementInvalid, selected.what, id)
			default:
				return fmt.Errorf("virtual machine %q: %w: %s names absent host CPU %d", name, errPlacementInvalid, selected.what, id)
			}
		}
	}

	memory := make(map[uint32]uint64, len(topology.Nodes))
	for _, node := range topology.Nodes {
		memory[node.ID] = node.MemoryTotalBytes
	}

	for _, selected := range nodes {
		ids, err := parseLibvirtHostIDList(selected.list, hypervisorhelpers.MaxHostNUMANodeID)
		if err != nil {
			return fmt.Errorf("virtual machine %q: %w: %s %q: %w", name, errPlacementInvalid, selected.what, selected.list, err)
		}

		for _, id := range ids.List() {
			switch total, ok := memory[uint32(id)]; {
			case !ok:
				return fmt.Errorf("virtual machine %q: %w: %s names absent host NUMA node %d", name, errPlacementInvalid, selected.what, id)
			case total == 0:
				return fmt.Errorf("virtual machine %q: %w: %s names host NUMA node %d without memory", name, errPlacementInvalid, selected.what, id)
			}
		}
	}

	return nil
}

func uint32Set(ids []uint32) map[uint32]struct{} {
	set := make(map[uint32]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}

	return set
}

func hasID(set map[uint32]struct{}, id uint32) bool {
	_, ok := set[id]

	return ok
}

// readNUMATopology returns the published host NUMA inventory, or nil while there is none.
func readNUMATopology(ctx context.Context, r controller.Reader) (*hardware.NUMATopologySpec, error) {
	topology, err := safe.ReaderGetByID[*hardware.NUMATopology](ctx, r, hardware.NUMATopologyID)
	if state.IsNotFoundError(err) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("get host NUMA topology: %w", err)
	}

	return topology.TypedSpec(), nil
}

// checkDomainPlacement validates the exact domain definition against the current host inventory.
func checkDomainPlacement(ctx context.Context, r controller.Reader, name, domainXML string) error {
	topology, err := readNUMATopology(ctx, r)
	if err != nil {
		return err
	}

	return validateDomainPlacement(name, domainXML, topology)
}
