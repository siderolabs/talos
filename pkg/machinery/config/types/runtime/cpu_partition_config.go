// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

//docgen:jsonschema

import (
	"errors"
	"fmt"

	"github.com/siderolabs/go-pointer"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
)

// CPUPartitionConfigKind is a CPU partition config document kind.
const CPUPartitionConfigKind = "CPUPartitionConfig"

// CPUPartitionSharedSliceName is the name of the implicit slice holding virtual machines which
// select no named slice. It cannot be declared.
const CPUPartitionSharedSliceName = "shared"

func init() {
	registry.Register(CPUPartitionConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &CPUPartitionConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.Validator               = &CPUPartitionConfigV1Alpha1{}
	_ config.CPUPartitionConfig      = &CPUPartitionConfigV1Alpha1{}
	_ config.CPUPartitionSliceConfig = &CPUPartitionSlice{}
)

// CPUPartitionConfigV1Alpha1 is a CPU partition config document.
//
//	description: |
//	  CPUPartitionConfig bounds the host CPUs each fixed Talos workload root may run on.
//
//	  The roots are the cgroups Talos already creates (`init`, `system`, `podruntime`,
//	  `taloscontainers`, `virtualMachines`) plus `kubepods`, the cgroup the kubelet
//	  creates for pods. A root left out of the document keeps running on every host CPU;
//	  a root listed in the document must name at least one CPU. Roots may overlap each other.
//
//	  The virtual machine root may additionally be divided into named slices which
//	  `VirtualMachineConfig` selects with `cpu.slice`. Virtual machines which select no
//	  slice run on the remainder of the virtual machine root, never on a named slice.
//
//	  An exclusive slice must not overlap any other root, and every other root must then be
//	  explicitly bounded. At most one virtual machine may select an exclusive slice.
//
//	  When `kubepods` is bounded, Talos owns the kubelet's `reservedSystemCPUs`.
//
//	  Changing the CPUs of a root or slice is applied live to running virtual machines
//	  when it can be done without ever letting an exclusive slice share a CPU with another
//	  workload and without moving a virtual machine between slices. Transitions which
//	  cannot be applied that way (introducing or removing slices while virtual machines
//	  run in the virtual machine root, changing a virtual machine's `cpu.slice`,
//	  swapping the CPUs of two occupied slices, shrinking a slice under a pinned virtual
//	  machine) are rejected and reported with the affected virtual machines; Talos never
//	  pauses, stops or restarts a virtual machine to apply a CPU policy change. To perform
//	  such a transition, set the affected virtual machines to `powerState: stopped`, wait
//	  until `CPUPartitionStatus` no longer lists them as blocking (Talos verifies the domain
//	  is removed and the slice cgroup is empty; a stopped `VirtualMachineStatus` alone is not
//	  release), apply the change, then set them back to `running`.
//
//	  If a bounded CPU goes offline or a managed cgroup mask is changed outside Talos, the
//	  loss is reported in `CPUPartitionStatus` and new virtual machine starts are refused;
//	  virtual machines already running are left alone, so their isolation may no longer hold
//	  until the boundary is restored. Talos never stops or restarts a virtual machine to
//	  react to such a loss.
//
//	  Removing the document restores every root it bounded. A root which had tasks before
//	  the policy and inherited its CPUs cannot be set back to inheriting while it has tasks
//	  (the kernel refuses an empty cpuset), so it is given its parent's full CPU set, which
//	  is the same set it ran on before; empty slices are removed.
//
//	  In container mode the document is validated but has no effect.
//	examples:
//	  - value: exampleCPUPartitionConfigV1Alpha1()
//	alias: CPUPartitionConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/CPUPartitionConfig
type CPUPartitionConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Host CPUs for machined and the early boot services.
	//
	//     Optional; omitting it leaves the root unrestricted.
	InitConfig *CPUPartitionRoot `yaml:"init,omitempty"`
	//   description: |
	//     Host CPUs for Talos system services (apid, trustd, udevd, ...).
	//
	//     Optional; omitting it leaves the root unrestricted.
	SystemConfig *CPUPartitionRoot `yaml:"system,omitempty"`
	//   description: |
	//     Host CPUs for the Kubernetes runtime components (containerd, the kubelet, etcd).
	//
	//     Optional; omitting it leaves the root unrestricted.
	PodRuntimeConfig *CPUPartitionRoot `yaml:"podruntime,omitempty"`
	//   description: |
	//     Host CPUs for Kubernetes pods.
	//
	//     Bounding this root makes Talos set the kubelet's `reservedSystemCPUs` to the
	//     complement of it; the kubelet configuration must not set that field itself.
	//
	//     Optional; omitting it leaves the root unrestricted.
	KubepodsConfig *CPUPartitionRoot `yaml:"kubepods,omitempty"`
	//   description: |
	//     Host CPUs for containers declared via `ContainerConfig`.
	//
	//     Optional; omitting it leaves the root unrestricted.
	TalosContainersConfig *CPUPartitionRoot `yaml:"taloscontainers,omitempty"`
	//   description: |
	//     Host CPUs for virtual machines declared via `VirtualMachineConfig`, optionally
	//     divided into named slices.
	//
	//     Optional; omitting it leaves the root unrestricted and declares no slice.
	VirtualMachinesConfig *CPUPartitionVirtualMachines `yaml:"virtualMachines,omitempty"`
}

// CPUPartitionRoot bounds one fixed root to a set of host CPUs.
type CPUPartitionRoot struct {
	//   description: |
	//     Host CPUs the root may run on, as a Linux CPU list, e.g. `0-1` or `0,2-3`.
	//
	//     Host CPU IDs are the kernel's logical CPU numbers, SMT threads included.
	//   examples:
	//     - value: '"0-1"'
	//   schemaRequired: true
	RootCPUs string `yaml:"cpus"`
}

// CPUPartitionVirtualMachines bounds the virtual machine root and divides it into slices.
type CPUPartitionVirtualMachines struct {
	//   description: |
	//     Host CPUs the virtual machine root may run on, as a Linux CPU list.
	//
	//     Every named slice is a subset of it; virtual machines selecting no slice run on
	//     what the slices leave of it.
	//   examples:
	//     - value: '"4-7"'
	//   schemaRequired: true
	RootCPUs string `yaml:"cpus"`
	//   description: |
	//     Named, pairwise disjoint subsets of `cpus` a `VirtualMachineConfig` selects with
	//     `cpu.slice`. The name `shared` is reserved for the remainder.
	//
	//     A configuration patch replaces this list as a whole rather than appending to it.
	SlicesConfig []CPUPartitionSlice `yaml:"slices,omitempty" merge:"replace"`
}

// CPUPartitionSlice is one named subset of the virtual machine root.
type CPUPartitionSlice struct {
	//   description: |
	//     Name of the slice, unique within the document.
	//
	//     Follows the virtual machine name rule: between 1 and 63 ASCII letters, digits
	//     and hyphens. `shared` is reserved.
	//   examples:
	//     - value: '"database"'
	//   schemaRequired: true
	SliceName string `yaml:"name"`
	//   description: |
	//     Host CPUs of the slice, as a Linux CPU list. Must be a subset of the virtual
	//     machine root and disjoint from every other slice.
	//   examples:
	//     - value: '"6-7"'
	//   schemaRequired: true
	SliceCPUs string `yaml:"cpus"`
	//   description: |
	//     Reserve the slice for a single virtual machine.
	//
	//     An exclusive slice must not overlap any other root, all of which must then be
	//     explicitly bounded, and at most one virtual machine may select it, whether it is
	//     running or not. Its CPUs stay reserved while no virtual machine selects it.
	//
	//     Optional; defaults to false.
	SliceExclusive *bool `yaml:"exclusive,omitempty"`
}

// NewCPUPartitionConfigV1Alpha1 creates a new CPU partition config document.
func NewCPUPartitionConfigV1Alpha1() *CPUPartitionConfigV1Alpha1 {
	return &CPUPartitionConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       CPUPartitionConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleCPUPartitionConfigV1Alpha1() *CPUPartitionConfigV1Alpha1 {
	cfg := NewCPUPartitionConfigV1Alpha1()
	cfg.InitConfig = &CPUPartitionRoot{RootCPUs: "0-1"}
	cfg.SystemConfig = &CPUPartitionRoot{RootCPUs: "0-1"}
	cfg.PodRuntimeConfig = &CPUPartitionRoot{RootCPUs: "0-1"}
	cfg.KubepodsConfig = &CPUPartitionRoot{RootCPUs: "2-3"}
	cfg.TalosContainersConfig = &CPUPartitionRoot{RootCPUs: "0-1"}
	cfg.VirtualMachinesConfig = &CPUPartitionVirtualMachines{
		RootCPUs: "4-7",
		SlicesConfig: []CPUPartitionSlice{
			{
				SliceName:      "database",
				SliceCPUs:      "6-7",
				SliceExclusive: new(true),
			},
		},
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *CPUPartitionConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// CPUPartitionConfigSignal implements config.CPUPartitionConfig interface.
func (s *CPUPartitionConfigV1Alpha1) CPUPartitionConfigSignal() {}

// Roots implements config.CPUPartitionConfig interface.
func (s *CPUPartitionConfigV1Alpha1) Roots() map[config.CPUPartitionRoot]string {
	roots := map[config.CPUPartitionRoot]string{}

	for root, cpus := range s.rootCPUs() {
		roots[root] = canonicalCPUList(cpus)
	}

	return roots
}

// Slices implements config.CPUPartitionConfig interface.
func (s *CPUPartitionConfigV1Alpha1) Slices() []config.CPUPartitionSliceConfig {
	if s.VirtualMachinesConfig == nil {
		return nil
	}

	out := make([]config.CPUPartitionSliceConfig, 0, len(s.VirtualMachinesConfig.SlicesConfig))

	for i := range s.VirtualMachinesConfig.SlicesConfig {
		out = append(out, &s.VirtualMachinesConfig.SlicesConfig[i])
	}

	return out
}

// Name implements config.CPUPartitionSliceConfig interface.
func (p *CPUPartitionSlice) Name() string {
	return p.SliceName
}

// CPUs implements config.CPUPartitionSliceConfig interface.
func (p *CPUPartitionSlice) CPUs() string {
	return canonicalCPUList(p.SliceCPUs)
}

// Exclusive implements config.CPUPartitionSliceConfig interface.
func (p *CPUPartitionSlice) Exclusive() bool {
	return pointer.SafeDeref(p.SliceExclusive)
}

// rootCPUs yields the CPU list of every root present in the document, as written, in reporting order.
func (s *CPUPartitionConfigV1Alpha1) rootCPUs() func(yield func(config.CPUPartitionRoot, string) bool) {
	return func(yield func(config.CPUPartitionRoot, string) bool) {
		for _, root := range []struct {
			name config.CPUPartitionRoot
			cfg  *CPUPartitionRoot
		}{
			{config.CPUPartitionRootInit, s.InitConfig},
			{config.CPUPartitionRootSystem, s.SystemConfig},
			{config.CPUPartitionRootPodRuntime, s.PodRuntimeConfig},
			{config.CPUPartitionRootKubepods, s.KubepodsConfig},
			{config.CPUPartitionRootTalosContainers, s.TalosContainersConfig},
		} {
			if root.cfg == nil {
				continue
			}

			if !yield(root.name, root.cfg.RootCPUs) {
				return
			}
		}

		if s.VirtualMachinesConfig != nil {
			yield(config.CPUPartitionRootVirtualMachines, s.VirtualMachinesConfig.RootCPUs)
		}
	}
}

// Validate implements config.Validator interface.
//
//nolint:gocyclo
func (s *CPUPartitionConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var validationErrors error

	roots := map[config.CPUPartitionRoot]cpuset.CPUSet{}
	bounded := false

	for root, list := range s.rootCPUs() {
		bounded = true

		set, err := parseRequiredCPUList(string(root)+".cpus", list)
		if err != nil {
			validationErrors = errors.Join(validationErrors, err)

			continue
		}

		roots[root] = set
	}

	if !bounded {
		return nil, errors.New("at least one root must be bounded")
	}

	if s.VirtualMachinesConfig == nil {
		return nil, validationErrors
	}

	vmRoot, haveVMRoot := roots[config.CPUPartitionRootVirtualMachines]
	names := map[string]struct{}{}
	claimed := cpuset.New()

	for i := range s.VirtualMachinesConfig.SlicesConfig {
		slice := &s.VirtualMachinesConfig.SlicesConfig[i]
		path := fmt.Sprintf("virtualMachines.slices[%d]", i)

		switch err := hypervisorhelpers.ValidateName(slice.SliceName); {
		case err != nil:
			validationErrors = errors.Join(validationErrors, fmt.Errorf("%s: %w", path, err))
		case slice.SliceName == CPUPartitionSharedSliceName:
			validationErrors = errors.Join(validationErrors, fmt.Errorf("%s: name %q is reserved", path, slice.SliceName))
		default:
			if _, exists := names[slice.SliceName]; exists {
				validationErrors = errors.Join(validationErrors, fmt.Errorf("%s: duplicate slice name %q", path, slice.SliceName))
			}

			names[slice.SliceName] = struct{}{}
		}

		set, err := parseRequiredCPUList(path+".cpus", slice.SliceCPUs)
		if err != nil {
			validationErrors = errors.Join(validationErrors, err)

			continue
		}

		if haveVMRoot && !set.IsSubsetOf(vmRoot) {
			validationErrors = errors.Join(validationErrors,
				fmt.Errorf("%s: cpus %q must be a subset of virtualMachines.cpus %q", path, set, vmRoot))
		}

		if overlap := set.Intersection(claimed); !overlap.IsEmpty() {
			validationErrors = errors.Join(validationErrors,
				fmt.Errorf("%s: cpus %q overlap another slice on %q", path, set, overlap))
		}

		claimed = claimed.Union(set)

		if !slice.Exclusive() {
			continue
		}

		for _, root := range config.CPUPartitionRoots() {
			if root == config.CPUPartitionRootVirtualMachines {
				continue
			}

			rootSet, present := roots[root]

			switch {
			case !present:
				validationErrors = errors.Join(validationErrors,
					fmt.Errorf("%s: exclusive slice %q requires %s to be bounded", path, slice.SliceName, root))
			case !rootSet.Intersection(set).IsEmpty():
				validationErrors = errors.Join(validationErrors,
					fmt.Errorf("%s: exclusive slice %q overlaps %s on %q", path, slice.SliceName, root, rootSet.Intersection(set)))
			}
		}
	}

	return nil, validationErrors
}

// parseRequiredCPUList parses a CPU list which must name at least one host CPU.
//
// An empty list parses to an empty set, so "" and a list naming no CPU are one and the same error.
func parseRequiredCPUList(path, list string) (cpuset.CPUSet, error) {
	set, err := hypervisorhelpers.ParseHostIDList(list, hypervisorhelpers.MaxHostCPUID)
	if err != nil {
		return cpuset.New(), fmt.Errorf("%s %q: %w", path, list, err)
	}

	if set.IsEmpty() {
		return cpuset.New(), fmt.Errorf("%s is required", path)
	}

	return set, nil
}

// canonicalCPUList returns the list in canonical form, or as written when it does not parse:
// validation has already rejected such a value, so nothing downstream relies on it.
func canonicalCPUList(list string) string {
	set, err := hypervisorhelpers.ParseHostIDList(list, hypervisorhelpers.MaxHostCPUID)
	if err != nil {
		return list
	}

	return set.String()
}
