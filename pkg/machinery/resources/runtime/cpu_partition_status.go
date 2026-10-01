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

// CPUPartitionStatus records the applied CPU policy.
type CPUPartitionStatus = typed.Resource[CPUPartitionStatusSpec, CPUPartitionStatusExtension]

// CPUPartitionStatusID is the ID of the singleton CPUPartitionStatus.
const CPUPartitionStatusID = CPUPartitionSpecID

// CPUPartitionPhase is the coordinator's phase.
type CPUPartitionPhase int

// CPUPartitionPhase values.
//
//structprotogen:gen_enum
const (
	CPUPartitionPhaseReady      CPUPartitionPhase = iota // ready
	CPUPartitionPhaseConverging                          // converging
	// CPUPartitionPhaseBlocked leaves admission open under the applied policy.
	CPUPartitionPhaseBlocked   // blocked
	CPUPartitionPhaseRestoring // restoring
	// CPUPartitionPhaseApplying closes admission before the occupancy snapshot.
	CPUPartitionPhaseApplying // applying
)

// AdmissionOpen reports whether this phase allows new VM starts.
func (p CPUPartitionPhase) AdmissionOpen() bool {
	return p == CPUPartitionPhaseReady || p == CPUPartitionPhaseBlocked
}

// CPUPartitionStatusSpec describes the applied CPU partition policy.
//
//gotagsrewrite:gen
type CPUPartitionStatusSpec struct {
	Phase CPUPartitionPhase `yaml:"phase" protobuf:"1"`
	// Sorted by key.
	Targets   []CPUPartitionTargetStatus `yaml:"targets,omitempty" protobuf:"2"`
	Exclusive []string                   `yaml:"exclusive,omitempty" protobuf:"3"`
	Blocked   []CPUPartitionBlock        `yaml:"blocked,omitempty" protobuf:"4"`
	// Closes admission without stopping running VMs.
	EnforcementLoss []string            `yaml:"enforcementLoss,omitempty" protobuf:"5"`
	Error           string              `yaml:"error,omitempty" protobuf:"6"`
	AdmissionErrors []CPUPartitionBlock `yaml:"admissionErrors,omitempty" protobuf:"7"`
	Waiting         string              `yaml:"waiting,omitempty" protobuf:"8"`
}

// CPUPartitionTargetStatus is what the coordinator did to one cgroup.
//
//gotagsrewrite:gen
type CPUPartitionTargetStatus struct {
	Key string `yaml:"key" protobuf:"1"`
	// Restore only if the current mask still matches LastApplied.
	Initial     string `yaml:"initial" protobuf:"2"`
	LastApplied string `yaml:"lastApplied" protobuf:"3"`
	// Persisted before the kernel write; a mismatch with LastApplied requires recovery.
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
	Reason          string   `yaml:"reason" protobuf:"1"`
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
