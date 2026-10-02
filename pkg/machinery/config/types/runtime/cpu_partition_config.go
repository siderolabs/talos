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
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
)

// CPUPartitionConfigKind is a CPU partition config document kind.
const CPUPartitionConfigKind = "CPUPartitionConfig"

// CPUPartitionSharedSliceName is reserved for the part of the virtual machine root outside every named slice.
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
//	  CPUPartitionConfig declares the host CPUs each fixed Talos workload root may run on.
//
//	  The roots are `init`, `system`, `podruntime`, `kubepods`, `taloscontainers` and
//	  `virtualMachines`. A root left out of the document is unrestricted; a root listed in
//	  the document must name at least one CPU. Roots may overlap each other.
//
//	  The document does not require virtual machines: omit `virtualMachines` to partition
//	  the host services and Kubernetes pods of an ordinary node.
//
//	  The virtual machine root may be divided into named, pairwise disjoint slices. An
//	  exclusive slice must not overlap any other root, and every other root must then be
//	  bounded explicitly.
//
//	  In container mode the document is validated but declares no policy.
//	examples:
//	  - value: exampleCPUPartitionConfigV1Alpha1()
//	    name: Kubernetes node with host services and pods on separate CPUs.
//	  - value: exampleCPUPartitionConfigV1Alpha1Slices()
//	    name: Virtual machine root with an exclusive slice.
//	alias: CPUPartitionConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/CPUPartitionConfig
type CPUPartitionConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Host CPUs for machined and the early boot services.
	InitConfig *CPUPartitionRoot `yaml:"init,omitempty"`
	//   description: |
	//     Host CPUs for Talos system services (apid, trustd, udevd, ...).
	SystemConfig *CPUPartitionRoot `yaml:"system,omitempty"`
	//   description: |
	//     Host CPUs for the Kubernetes runtime components (containerd, the kubelet, etcd).
	PodRuntimeConfig *CPUPartitionRoot `yaml:"podruntime,omitempty"`
	//   description: |
	//     Host CPUs for Kubernetes pods.
	KubepodsConfig *CPUPartitionRoot `yaml:"kubepods,omitempty"`
	//   description: |
	//     Host CPUs for containers declared via `ContainerConfig`.
	TalosContainersConfig *CPUPartitionRoot `yaml:"taloscontainers,omitempty"`
	//   description: |
	//     Host CPUs for virtual machines, optionally divided into named slices.
	VirtualMachinesConfig *CPUPartitionVirtualMachines `yaml:"virtualMachines,omitempty"`
}

// CPUPartitionRoot bounds one fixed root to a set of host CPUs.
type CPUPartitionRoot struct {
	//   description: |
	//     Host CPUs as a Linux CPU list, e.g. `0-1` or `0,2-3`.
	//
	//     CPU IDs are the kernel's logical CPU numbers, SMT threads included.
	//   examples:
	//     - value: '"0-1"'
	//   schemaRequired: true
	RootCPUs string `yaml:"cpus"`
}

// CPUPartitionVirtualMachines bounds the virtual machine root and divides it into slices.
type CPUPartitionVirtualMachines struct {
	//   description: |
	//     Host CPUs of the virtual machine root, as a Linux CPU list.
	//   examples:
	//     - value: '"4-7"'
	//   schemaRequired: true
	RootCPUs string `yaml:"cpus"`
	//   description: |
	//     Named, pairwise disjoint subsets of `cpus`.
	//
	//     A configuration patch replaces this list as a whole.
	SlicesConfig []CPUPartitionSlice `yaml:"slices,omitempty" merge:"replace"`
}

// CPUPartitionSlice is one named subset of the virtual machine root.
type CPUPartitionSlice struct {
	//   description: |
	//     Unique slice name: 1 to 63 ASCII letters, digits and hyphens. `shared` is reserved.
	//   examples:
	//     - value: '"database"'
	//   schemaRequired: true
	SliceName string `yaml:"name"`
	//   description: |
	//     Host CPUs of the slice, as a Linux CPU list within the virtual machine root.
	//   examples:
	//     - value: '"6-7"'
	//   schemaRequired: true
	SliceCPUs string `yaml:"cpus"`
	//   description: |
	//     Keep the slice's CPUs out of every other root.
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
	cfg.KubepodsConfig = &CPUPartitionRoot{RootCPUs: "2-7"}

	return cfg
}

func exampleCPUPartitionConfigV1Alpha1Slices() *CPUPartitionConfigV1Alpha1 {
	cfg := exampleCPUPartitionConfigV1Alpha1()
	cfg.KubepodsConfig.RootCPUs = "2-3"
	cfg.TalosContainersConfig = &CPUPartitionRoot{RootCPUs: cfg.InitConfig.RootCPUs}
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

func (s *CPUPartitionConfigV1Alpha1) rootCPUs() func(yield func(config.CPUPartitionRoot, string) bool) {
	return func(yield func(config.CPUPartitionRoot, string) bool) {
		for _, root := range []struct {
			name config.CPUPartitionRoot
			cfg  *CPUPartitionRoot
		}{
			{constants.CgroupInit, s.InitConfig},
			{constants.CgroupSystem, s.SystemConfig},
			{constants.CgroupPodRuntimeRoot, s.PodRuntimeConfig},
			{constants.CgroupKubepods, s.KubepodsConfig},
			{constants.CgroupTalosContainersRoot, s.TalosContainersConfig},
		} {
			if root.cfg == nil {
				continue
			}

			if !yield(root.name, root.cfg.RootCPUs) {
				return
			}
		}

		if s.VirtualMachinesConfig != nil {
			yield(constants.CgroupVirtualMachinesRoot, s.VirtualMachinesConfig.RootCPUs)
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

		name := string(root)
		if root == constants.CgroupVirtualMachinesRoot {
			name = "virtualMachines"
		}

		set, err := parseRequiredCPUList(name+".cpus", list)
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

	vmRoot, haveVMRoot := roots[constants.CgroupVirtualMachinesRoot]
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

		// An unparseable root is already reported; checking its slices against it would only repeat that.
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
			if root == constants.CgroupVirtualMachinesRoot {
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

// parseRequiredCPUList rejects an empty list, which ParseHostIDList accepts as the empty set.
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

// canonicalCPUList returns an unparseable list unchanged; validation rejects it before any consumer reads it.
func canonicalCPUList(list string) string {
	set, err := hypervisorhelpers.ParseHostIDList(list, hypervisorhelpers.MaxHostCPUID)
	if err != nil {
		return list
	}

	return set.String()
}
