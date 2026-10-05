// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

//docgen:jsonschema

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/kernel"
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
	_ config.RuntimeValidator       = &WorkloadResourceConfigV1Alpha1{}
	_ config.WorkloadResourceConfig = &WorkloadResourceConfigV1Alpha1{}
)

// WorkloadResourceConfigV1Alpha1 is a workload resource config document.
//
//	description: |
//	  WorkloadResourceConfig declares an aggregate memory ceiling for each Talos workload root.
//
//	  The roots are `kubepods`, `taloscontainers` and `virtualMachines`; every root is optional
//	  and independent of the others. Omitting a root removes its aggregate cap; individual child
//	  limits still apply.
//	  A limit must be positive; it is rounded down to the host page size before it is applied,
//	  and bounds RAM only (including page cache charged to the root): swap and hugepages are
//	  not limited by it.
//
//	  Talos exclusively owns `memory.max` on `/taloscontainers` and `/virtualmachines.partition`,
//	  even without this document. An omitted limit (zero internally) means an unlimited aggregate cap.
//	  Kubelet reserved-memory enforcement on these roots is rejected; CPU-only or compressible
//	  enforcement is allowed. Static validation cannot inspect external kubelet drop-in files;
//	  drop-in directories are rejected while any workload cap is active.
//	  Lowering a limit below current usage makes the kernel reclaim and OOM-kill within that root,
//	  which can terminate containers or virtual machines.
//
//	  The `kubepods` limit is applied by the kubelet, which stays the only writer of its cgroup:
//	  Talos derives `systemReserved.memory` from the limit, so changing it restarts the kubelet,
//	  and a limit below the memory used by pods (control plane static pods included) causes
//	  OOM kills. The schedulable Node Allocatable is the limit minus the hard eviction
//	  threshold and any hugepage capacity, so it is lower than the limit. Setting
//	  `systemReserved.memory`, the equivalent kubelet command line flags or the `Static` memory
//	  manager policy together with the limit is rejected. The limit is inactive while Kubernetes
//	  is not configured on the machine; the other roots are enforced regardless.
//
//	  In container mode the document is validated but declares no policy.
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
	//     Limits for Kubernetes pods.
	KubepodsConfig *WorkloadResourceRoot `yaml:"kubepods,omitempty"`
	//   description: |
	//     Limits for containers declared via `ContainerConfig`.
	TalosContainersConfig *WorkloadResourceRoot `yaml:"taloscontainers,omitempty"`
	//   description: |
	//     Limits for virtual machines.
	VirtualMachinesConfig *WorkloadResourceRoot `yaml:"virtualMachines,omitempty"`
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
	//     Aggregate memory ceiling of the root, in bytes.
	//
	//     The value can be expressed in human readable format, e.g. 16GiB, and must be positive.
	//   examples:
	//     - value: '"16GiB"'
	//   schema:
	//     type: string
	//   schemaRequired: true
	MemoryLimit meta.ByteSize `yaml:"limit,omitempty"`
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
	cfg.KubepodsConfig = newWorkloadResourceRoot("16GiB")
	cfg.TalosContainersConfig = newWorkloadResourceRoot("4GiB")
	cfg.VirtualMachinesConfig = newWorkloadResourceRoot("32GiB")

	return cfg
}

func exampleWorkloadResourceConfigV1Alpha1VirtualMachines() *WorkloadResourceConfigV1Alpha1 {
	cfg := NewWorkloadResourceConfigV1Alpha1()
	cfg.VirtualMachinesConfig = newWorkloadResourceRoot("48GiB")

	return cfg
}

func newWorkloadResourceRoot(limit string) *WorkloadResourceRoot {
	return &WorkloadResourceRoot{
		MemoryConfig: &WorkloadMemoryResource{
			MemoryLimit: meta.MustByteSize(limit),
		},
	}
}

// Clone implements config.Document interface.
func (s *WorkloadResourceConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// WorkloadResourceConfigSignal implements config.WorkloadResourceConfig interface.
func (s *WorkloadResourceConfigV1Alpha1) WorkloadResourceConfigSignal() {}

// KubepodsMemoryLimit implements config.WorkloadResourceConfig interface.
func (s *WorkloadResourceConfigV1Alpha1) KubepodsMemoryLimit() uint64 {
	return s.KubepodsConfig.memoryLimit()
}

// TalosContainersMemoryLimit implements config.WorkloadResourceConfig interface.
func (s *WorkloadResourceConfigV1Alpha1) TalosContainersMemoryLimit() uint64 {
	return s.TalosContainersConfig.memoryLimit()
}

// VirtualMachinesMemoryLimit implements config.WorkloadResourceConfig interface.
func (s *WorkloadResourceConfigV1Alpha1) VirtualMachinesMemoryLimit() uint64 {
	return s.VirtualMachinesConfig.memoryLimit()
}

func (r *WorkloadResourceRoot) memoryLimit() uint64 {
	if r == nil || r.MemoryConfig == nil || r.MemoryConfig.MemoryLimit.IsNegative() {
		return 0
	}

	return r.MemoryConfig.MemoryLimit.Value()
}

// Validate implements config.Validator interface.
func (s *WorkloadResourceConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var validationErrors error

	limited := false

	for _, root := range []struct {
		name string
		cfg  *WorkloadResourceRoot
	}{
		{constants.CgroupKubepods, s.KubepodsConfig},
		{constants.CgroupTalosContainersRoot, s.TalosContainersConfig},
		{virtualMachinesRootField, s.VirtualMachinesConfig},
	} {
		if root.cfg == nil {
			continue
		}

		limited = true

		validationErrors = errors.Join(validationErrors, validateMemoryLimit(root.name+".memory.limit", root.cfg.MemoryConfig))
	}

	if !limited {
		return nil, errors.New("at least one root must be limited")
	}

	return nil, validationErrors
}

// RuntimeValidate implements config.RuntimeValidator using the target node's page size.
func (s *WorkloadResourceConfigV1Alpha1) RuntimeValidate(context.Context, state.State, validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var validationErrors error

	for _, root := range []struct {
		name string
		cfg  *WorkloadResourceRoot
	}{
		{constants.CgroupKubepods, s.KubepodsConfig},
		{constants.CgroupTalosContainersRoot, s.TalosContainersConfig},
		{virtualMachinesRootField, s.VirtualMachinesConfig},
	} {
		if root.cfg == nil {
			continue
		}

		path := root.name + ".memory.limit"

		// Static validation reports missing, negative, and out-of-range values.
		if validateMemoryLimit(path, root.cfg.MemoryConfig) != nil {
			continue
		}

		if _, err := kernel.NormalizeMemoryLimit(root.cfg.MemoryConfig.MemoryLimit.Value(), os.Getpagesize()); err != nil {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("%s: %w", path, err))
		}
	}

	return nil, validationErrors
}

// validateMemoryLimit accepts only an explicit positive limit which fits the kubelet's int64 quantities.
//
// Page alignment is a host property and is checked at runtime.
func validateMemoryLimit(path string, memory *WorkloadMemoryResource) error {
	if memory == nil || memory.MemoryLimit.IsZero() {
		return fmt.Errorf("%s is required", path)
	}

	switch limit := memory.MemoryLimit; {
	case limit.IsNegative():
		return fmt.Errorf("%s cannot be negative", path)
	case limit.Value() == 0:
		return fmt.Errorf("%s must be greater than zero", path)
	case limit.Value() > math.MaxInt64:
		return fmt.Errorf("%s cannot be greater than %d bytes", path, int64(math.MaxInt64))
	}

	return nil
}
