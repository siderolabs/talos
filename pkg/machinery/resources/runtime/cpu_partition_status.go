// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"cmp"
	"slices"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// CPUPartitionStatusType is type of CPUPartitionStatus resource.
const CPUPartitionStatusType = resource.Type("CPUPartitionStatuses.runtime.talos.dev")

// CPUPartitionStatus is the CPU partition policy actually applied to the kernel.
//
// It exists from the coordinator's first cgroup write until the last managed boundary has
// been restored; while it exists, virtual machine admission is bound to it: a virtual machine
// starts only in the placement the applied policy grants it. The desired policy lives in
// CPUPartitionSpec; a desired change which cannot be applied live leaves this resource, and the
// kernel, untouched and is reported in Blocked.
type CPUPartitionStatus = typed.Resource[CPUPartitionStatusSpec, CPUPartitionStatusExtension]

// CPUPartitionStatusID is the ID of the singleton CPUPartitionStatus.
const CPUPartitionStatusID = CPUPartitionSpecID

// CPUPartitionPhase is the coordinator's phase.
type CPUPartitionPhase int

// CPUPartitionPhase values.
//
//structprotogen:gen_enum
const (
	// CPUPartitionPhaseReady: the applied policy equals the desired one.
	CPUPartitionPhaseReady CPUPartitionPhase = iota // ready
	// CPUPartitionPhaseConverging: a live plan is being executed.
	CPUPartitionPhaseConverging // converging
	// CPUPartitionPhaseBlocked: the desired policy cannot be applied live; the applied policy stays.
	CPUPartitionPhaseBlocked // blocked
	// CPUPartitionPhaseRestoring: the policy is being removed; boundaries are being restored.
	CPUPartitionPhaseRestoring // restoring
	// CPUPartitionPhaseApplying: admission is closed while the coordinator snapshots occupancy
	// and writes; a virtual machine start seeing this phase must wait.
	CPUPartitionPhaseApplying // applying
)

// AdmissionOpen reports whether virtual machines may start under this phase: the kernel is not
// being rewritten and no occupancy snapshot is in progress.
func (p CPUPartitionPhase) AdmissionOpen() bool {
	return p == CPUPartitionPhaseReady || p == CPUPartitionPhaseBlocked
}

// CPUPartitionStatusSpec describes the applied CPU partition policy.
//
//gotagsrewrite:gen
type CPUPartitionStatusSpec struct {
	Phase CPUPartitionPhase `yaml:"phase" protobuf:"1"`
	// Targets lists every managed cgroup, keyed as `init`, `kubepods`, ..., `virtualMachines`,
	// `virtualMachines/shared`, `virtualMachines/<slice>`, with what was applied to it; sorted by key.
	Targets []CPUPartitionTargetStatus `yaml:"targets,omitempty" protobuf:"2"`
	// Exclusive names the applied exclusive slices.
	Exclusive []string `yaml:"exclusive,omitempty" protobuf:"3"`
	// Blocked explains why the desired policy is not applied; empty unless Phase is blocked.
	Blocked []CPUPartitionBlock `yaml:"blocked,omitempty" protobuf:"4"`
	// EnforcementLoss lists managed targets whose applied mask names offline CPUs or whose
	// kernel mask no longer matches what was applied (foreign change).
	//
	// A loss is reported and closes admission for new virtual machine starts; virtual machines
	// already running are left alone, so their CPU isolation may no longer hold until the
	// boundaries are restored (CPUs back online, or the foreign mask reverted) and the loss
	// clears. Talos never stops, restarts or changes the power state of a running machine to
	// react to a loss.
	EnforcementLoss []string `yaml:"enforcementLoss,omitempty" protobuf:"5"`
	// Error is the last execution error, when a plan step failed.
	Error string `yaml:"error,omitempty" protobuf:"6"`
	// AdmissionErrors names virtual machines refused a placement and why (undeclared slice, more
	// than one owner of an exclusive slice, pins outside the slice); unrelated machines are unaffected.
	AdmissionErrors []CPUPartitionBlock `yaml:"admissionErrors,omitempty" protobuf:"7"`
	// Waiting names the barrier a plan is waiting on (a kubepods leaf still on removed CPUs, a
	// cgroup the kubelet has not created yet); empty unless Phase is converging.
	Waiting string `yaml:"waiting,omitempty" protobuf:"8"`
}

// CPUPartitionTargetStatus is what the coordinator did to one cgroup.
//
//gotagsrewrite:gen
type CPUPartitionTargetStatus struct {
	// Key names the target: a root name, `virtualMachines/shared` or `virtualMachines/<slice>`.
	Key string `yaml:"key" protobuf:"1"`
	// Initial is the cpuset.cpus read before the coordinator's first write; restored on removal
	// when the current value is still LastApplied.
	Initial string `yaml:"initial" protobuf:"2"`
	// LastApplied is the coordinator's last successful write.
	LastApplied string `yaml:"lastApplied" protobuf:"3"`
	// Intended is the value of a write in progress; equals LastApplied when none is.
	// A restart finding Intended != LastApplied re-verifies the kernel before trusting either.
	Intended string `yaml:"intended,omitempty" protobuf:"4"`
}

// Target returns the status of the target with the given key.
func (s *CPUPartitionStatusSpec) Target(key string) (CPUPartitionTargetStatus, bool) {
	for _, target := range s.Targets {
		if target.Key == key {
			return target, true
		}
	}

	return CPUPartitionTargetStatus{}, false
}

// SetTarget inserts or replaces the target's status, keeping the list sorted by key.
func (s *CPUPartitionStatusSpec) SetTarget(target CPUPartitionTargetStatus) {
	i, found := slices.BinarySearchFunc(s.Targets, target, func(a, b CPUPartitionTargetStatus) int {
		return cmp.Compare(a.Key, b.Key)
	})

	if found {
		s.Targets[i] = target

		return
	}

	s.Targets = slices.Insert(s.Targets, i, target)
}

// DeleteTarget removes the target with the given key.
func (s *CPUPartitionStatusSpec) DeleteTarget(key string) {
	s.Targets = slices.DeleteFunc(s.Targets, func(target CPUPartitionTargetStatus) bool { return target.Key == key })
}

// CPUPartitionBlock names one reason the desired policy is not applied.
//
//gotagsrewrite:gen
type CPUPartitionBlock struct {
	Reason string `yaml:"reason" protobuf:"1"`
	// VirtualMachines the operator has to stop and release to let the transition through.
	VirtualMachines []string `yaml:"virtualMachines,omitempty" protobuf:"2"`
	CPUs            string   `yaml:"cpus,omitempty" protobuf:"3"`
}

// NewCPUPartitionStatus initializes a CPUPartitionStatus resource.
func NewCPUPartitionStatus() *CPUPartitionStatus {
	return typed.NewResource[CPUPartitionStatusSpec, CPUPartitionStatusExtension](
		resource.NewMetadata(NamespaceName, CPUPartitionStatusType, CPUPartitionStatusID, resource.VersionUndefined),
		CPUPartitionStatusSpec{},
	)
}

// CPUPartitionStatusExtension is auxiliary resource data for CPUPartitionStatus.
type CPUPartitionStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (CPUPartitionStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             CPUPartitionStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Phase",
				JSONPath: `{.phase}`,
			},
			{
				Name:     "Error",
				JSONPath: `{.error}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[CPUPartitionStatusSpec](CPUPartitionStatusType, &CPUPartitionStatus{})
	if err != nil {
		panic(err)
	}
}
