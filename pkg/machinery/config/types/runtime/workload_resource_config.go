// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

//docgen:jsonschema

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// WorkloadResourceConfigKind is a workload resource config document kind.
const WorkloadResourceConfigKind = "WorkloadResourceConfig"

// virtualMachinesRootField is the document field of the virtual machine root, which differs from its cgroup name.
const virtualMachinesRootField = "virtualMachines"

func init() {
	registry.Register(WorkloadResourceConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &WorkloadResourceConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.Validator              = &WorkloadResourceConfigV1Alpha1{}
	_ config.WorkloadResourceConfig = &WorkloadResourceConfigV1Alpha1{}
)

// WorkloadResourceConfigV1Alpha1 is a workload resource config document.
//
//	description: |
//	  WorkloadResourceConfig splits the host memory between the Talos workload roots.
//
//	  `reserved.memory` stays with the host (`/init`, `/system`, `/podruntime` and the kernel); each
//	  root gets an aggregate memory ceiling of a whole percentage of the rest. The roots are
//	  `kubepods`, `taloscontainers` and `virtualMachines`; every root is optional, and the
//	  percentages must not add up to more than 100%.
//	  Limits are resolved against the host memory at runtime and follow memory hotplug. They are
//	  rounded down to the host page size before they are applied, and bound RAM only (including
//	  page cache charged to the root): swap and hugepages are not limited by them.
//
//	  Talos exclusively owns `memory.max` on `/taloscontainers` and `/virtualmachines.partition`,
//	  even without this document. An omitted root, `kubepods` included, is capped at the host memory
//	  minus `reserved.memory`, which defaults to the kubelet's default `systemReserved.memory`
//	  (512Mi on control plane nodes, 384Mi on workers). Individual child limits still apply.
//	  Kubelet `system-reserved` and `kube-reserved` enforcement on these roots and on
//	  `/system/extensions`, whose `pids.max` Talos also owns, is rejected, compressible included.
//	  Static validation cannot inspect external kubelet drop-in files;
//	  drop-in directories are rejected while this document sets any limit.
//	  Lowering a limit below current usage makes the kernel reclaim and OOM-kill within that root,
//	  which can terminate containers or virtual machines.
//
//	  The `kubepods` cap is applied by the kubelet, which stays the only writer of its cgroup:
//	  Talos sets `systemReserved.memory` to `reserved.memory`, or derives it from the `kubepods`
//	  limit. A limit makes `systemReserved.memory` depend on the host memory, so a memory hotplug
//	  restarts the kubelet; without a limit it does not. Changing either value restarts the kubelet,
//	  and a limit below the memory used by pods (control plane static pods included) causes
//	  OOM kills. The schedulable Node Allocatable is the limit minus the hard eviction
//	  threshold and any hugepage capacity, so it is lower than the limit. With the limit,
//	  `systemReserved.memory`, `kubeReserved.memory`, the equivalent kubelet command line flags,
//	  drop-in directories and the `Static` memory manager policy are rejected. Without it, an
//	  explicit `systemReserved.memory`, the `--system-reserved` flag or a drop-in file overrides
//	  `reserved.memory` for the kubelet, and keeping them consistent is the user's responsibility.
//	  The limit is inactive while Kubernetes is not configured on the machine; the other roots are
//	  enforced regardless.
//
//	  In container mode the cgroups are nested, so the limits are not applied; `reserved.memory`
//	  still sets the kubelet `systemReserved.memory`.
//	examples:
//	  - value: exampleWorkloadResourceConfigV1Alpha1()
//	    name: Kubernetes node with containers and virtual machines.
//	  - value: exampleWorkloadResourceConfigV1Alpha1VirtualMachines()
//	    name: Virtual machine host without Kubernetes.
//	alias: WorkloadResourceConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/WorkloadResourceConfig
type WorkloadResourceConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Resources left out of the split for the host.
	ReservedConfig *WorkloadReserved `yaml:"reserved,omitempty"`
	//   description: |
	//     Limits for Kubernetes pods.
	KubepodsConfig *WorkloadResourceRoot `yaml:"kubepods,omitempty"`
	//   description: |
	//     Limits for containers declared via `ContainerConfig`.
	TalosContainersConfig *WorkloadResourceRoot `yaml:"taloscontainers,omitempty"`
	//   description: |
	//     Limits for virtual machines.
	VirtualMachinesConfig *WorkloadResourceRoot `yaml:"virtualMachines,omitempty"`
}

// WorkloadReserved holds the resources left for the host.
type WorkloadReserved struct {
	//   description: |
	//     Host memory left out of the split. Defaults to the kubelet's default
	//     `systemReserved.memory`: 512Mi on control plane nodes, 384Mi on workers.
	//
	//     It is not written to any cgroup: it is the room left for the host processes
	//     and the kernel. Talos passes it to the kubelet as `systemReserved.memory`
	//     unless `kubepods` sets a limit.
	//   examples:
	//     - value: '"2GiB"'
	//   schema:
	//     type: string
	MemoryConfig meta.ByteSize `yaml:"memory,omitempty"`
}

// WorkloadResourceRoot holds the limits of one workload root.
type WorkloadResourceRoot struct {
	//   description: |
	//     Memory limits of the root.
	//   schemaRequired: true
	MemoryConfig *WorkloadMemoryResource `yaml:"memory,omitempty"`
}

// WorkloadMemoryResource bounds the memory of one workload root.
type WorkloadMemoryResource struct {
	//   description: |
	//     Aggregate memory ceiling of the root, as a whole percentage of the host memory
	//     minus `reserved.memory`. It must be positive.
	//   examples:
	//     - value: '"40%"'
	//   schema:
	//     type: string
	//   schemaRequired: true
	MemoryLimit meta.PercentageSize `yaml:"limit,omitempty"`
}

// NewWorkloadResourceConfigV1Alpha1 creates a new workload resource config document.
func NewWorkloadResourceConfigV1Alpha1() *WorkloadResourceConfigV1Alpha1 {
	return &WorkloadResourceConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       WorkloadResourceConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleWorkloadResourceConfigV1Alpha1() *WorkloadResourceConfigV1Alpha1 {
	cfg := NewWorkloadResourceConfigV1Alpha1()
	cfg.ReservedConfig = &WorkloadReserved{MemoryConfig: meta.MustByteSize("2GiB")}
	cfg.KubepodsConfig = newWorkloadResourceRoot("50%")
	cfg.TalosContainersConfig = newWorkloadResourceRoot("10%")
	cfg.VirtualMachinesConfig = newWorkloadResourceRoot("40%")

	return cfg
}

func exampleWorkloadResourceConfigV1Alpha1VirtualMachines() *WorkloadResourceConfigV1Alpha1 {
	cfg := NewWorkloadResourceConfigV1Alpha1()
	cfg.VirtualMachinesConfig = newWorkloadResourceRoot("80%")

	return cfg
}

func newWorkloadResourceRoot(limit string) *WorkloadResourceRoot {
	return &WorkloadResourceRoot{
		MemoryConfig: &WorkloadMemoryResource{
			MemoryLimit: meta.MustPercentageSize(limit),
		},
	}
}

// Clone implements config.Document interface.
func (s *WorkloadResourceConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// WorkloadResourceConfigSignal implements config.WorkloadResourceConfig interface.
func (s *WorkloadResourceConfigV1Alpha1) WorkloadResourceConfigSignal() {}

// MemoryReserved implements config.WorkloadResourceConfig interface.
func (s *WorkloadResourceConfigV1Alpha1) MemoryReserved() optional.Optional[uint64] {
	if s.ReservedConfig == nil || s.ReservedConfig.MemoryConfig.IsZero() || s.ReservedConfig.MemoryConfig.IsNegative() {
		return optional.None[uint64]()
	}

	return optional.Some(s.ReservedConfig.MemoryConfig.Value())
}

// KubepodsMemoryPercent implements config.WorkloadResourceConfig interface.
func (s *WorkloadResourceConfigV1Alpha1) KubepodsMemoryPercent() uint64 {
	return s.KubepodsConfig.memoryPercent()
}

// TalosContainersMemoryPercent implements config.WorkloadResourceConfig interface.
func (s *WorkloadResourceConfigV1Alpha1) TalosContainersMemoryPercent() uint64 {
	return s.TalosContainersConfig.memoryPercent()
}

// VirtualMachinesMemoryPercent implements config.WorkloadResourceConfig interface.
func (s *WorkloadResourceConfigV1Alpha1) VirtualMachinesMemoryPercent() uint64 {
	return s.VirtualMachinesConfig.memoryPercent()
}

func (r *WorkloadResourceRoot) memoryPercent() uint64 {
	if r == nil || r.MemoryConfig == nil || r.MemoryConfig.MemoryLimit.IsNegative() {
		return 0
	}

	return r.MemoryConfig.MemoryLimit.Value()
}

// Validate implements config.Validator interface.
func (s *WorkloadResourceConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var validationErrors error

	if reserved, ok := s.MemoryReserved().Get(); ok && reserved == 0 {
		validationErrors = errors.New("reserved.memory must be greater than zero")
	}

	if s.ReservedConfig != nil && s.ReservedConfig.MemoryConfig.IsNegative() {
		validationErrors = errors.New("reserved.memory cannot be negative")
	}

	var total uint64

	for _, root := range s.roots() {
		if root.cfg == nil {
			continue
		}

		err := validateMemoryLimit(root.name+".memory.limit", root.cfg.MemoryConfig)
		if err == nil {
			total += root.cfg.memoryPercent()
		}

		validationErrors = errors.Join(validationErrors, err)
	}

	if total > 100 {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("memory limits add up to %d%%, more than 100%%", total))
	}

	return nil, validationErrors
}

// workloadRoot names one workload root of the document.
//
//docgen:nodoc
type workloadRoot struct {
	name string
	cfg  *WorkloadResourceRoot
}

func (s *WorkloadResourceConfigV1Alpha1) roots() []workloadRoot {
	return []workloadRoot{
		{constants.CgroupKubepods, s.KubepodsConfig},
		{constants.CgroupTalosContainersRoot, s.TalosContainersConfig},
		{virtualMachinesRootField, s.VirtualMachinesConfig},
	}
}

// validateMemoryLimit accepts only an explicit positive whole percentage.
func validateMemoryLimit(path string, memory *WorkloadMemoryResource) error {
	if memory == nil || memory.MemoryLimit.IsZero() {
		return fmt.Errorf("%s is required", path)
	}

	// The percentage parser truncates fractions.
	text, err := memory.MemoryLimit.MarshalText()
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	switch {
	case memory.MemoryLimit.IsNegative():
		return fmt.Errorf("%s cannot be negative", path)
	case bytes.ContainsRune(text, '.'):
		return fmt.Errorf("%s must be a whole percentage", path)
	case memory.MemoryLimit.Value() == 0:
		return fmt.Errorf("%s must be greater than zero", path)
	}

	return nil
}
