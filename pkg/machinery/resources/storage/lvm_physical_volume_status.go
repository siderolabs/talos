// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// LVMPhysicalVolumeStatusType is the type of LVMPhysicalVolumeStatus resource.
const LVMPhysicalVolumeStatusType = resource.Type("LVMPhysicalVolumeStatuses.storage.talos.dev")

// LVMPhysicalVolumeStatus resource contains information about the LVM physical volume status.
type LVMPhysicalVolumeStatus = typed.Resource[LVMPhysicalVolumeStatusSpec, LVMPhysicalVolumeStatusExtension]

// LVMPhysicalVolumeStatusSpec is the spec for LVMPhysicalVolumeStatus resource.
//
// Fields mirror selected columns of `pvs -a -o +all --reportformat json --units b --nosuffix`.
// Numeric / tri-state columns are exposed as raw strings so LVM's sentinels
// ("", "-1") are surfaced verbatim. See pvs(8) for the source-of-truth
// definitions of each column.
//
//gotagsrewrite:gen
type LVMPhysicalVolumeStatusSpec struct {
	// Device is the block-device path backing the PV (pv_name).
	Device string `yaml:"device" protobuf:"1"`
	// VGName is the parent volume group name; empty if the PV is not in a VG.
	VGName string `yaml:"vgName" protobuf:"2"`
	// UUID is the stable PV identifier written to the PV label (pv_uuid).
	UUID string `yaml:"uuid" protobuf:"3"`
	// Format is the on-disk metadata format (pv_fmt), e.g. "lvm2".
	Format string `yaml:"format" protobuf:"4"`
	// Allocatable is the raw pv_allocatable column ("allocatable" / "").
	Allocatable string `yaml:"allocatable" protobuf:"5"`
	// Exported is the raw pv_exported column ("exported" / "").
	Exported string `yaml:"exported" protobuf:"6"`
	// Missing is the raw pv_missing column ("missing" / "").
	Missing string `yaml:"missing" protobuf:"7"`
	// InUse is the raw pv_in_use column ("used" / "").
	InUse string `yaml:"inUse" protobuf:"8"`
	// Size is the raw pv_size column in bytes.
	Size string `yaml:"size" protobuf:"9"`
	// DeviceSize is the raw dev_size column in bytes.
	DeviceSize string `yaml:"deviceSize" protobuf:"10"`
	// Free is the raw pv_free column in bytes.
	Free string `yaml:"free" protobuf:"11"`
	// Used is the raw pv_used column in bytes.
	Used string `yaml:"used" protobuf:"12"`
	// PECount is the raw pv_pe_count column.
	PECount string `yaml:"peCount" protobuf:"13"`
	// PEAllocCount is the raw pv_pe_alloc_count column.
	PEAllocCount string `yaml:"peAllocCount" protobuf:"14"`
	// Major is the raw pv_major column ("-1" for orphan PVs, otherwise the kernel major).
	Major string `yaml:"major" protobuf:"15"`
	// Minor is the raw pv_minor column ("-1" for orphan PVs, otherwise the kernel minor).
	Minor string `yaml:"minor" protobuf:"16"`
	// Tags is the list of tags attached to the PV (pv_tags).
	Tags []string `yaml:"tags" protobuf:"17"`
}

// NewLVMPhysicalVolumeStatus initializes a LVMPhysicalVolumeStatus resource.
func NewLVMPhysicalVolumeStatus(namespace resource.Namespace, id resource.ID) *LVMPhysicalVolumeStatus {
	return typed.NewResource[LVMPhysicalVolumeStatusSpec, LVMPhysicalVolumeStatusExtension](
		resource.NewMetadata(namespace, LVMPhysicalVolumeStatusType, id, resource.VersionUndefined),
		LVMPhysicalVolumeStatusSpec{},
	)
}

// LVMPhysicalVolumeStatusExtension is auxiliary resource data for LVMPhysicalVolumeStatus.
type LVMPhysicalVolumeStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (LVMPhysicalVolumeStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LVMPhysicalVolumeStatusType,
		Aliases:          []resource.Type{"pv"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{Name: "Device", JSONPath: "{.device}"},
			{Name: "VG", JSONPath: "{.vgName}"},
			{Name: "Size", JSONPath: "{.size}"},
			{Name: "Free", JSONPath: "{.free}"},
			{Name: "Allocatable", JSONPath: "{.allocatable}"},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(LVMPhysicalVolumeStatusType, &LVMPhysicalVolumeStatus{})
	if err != nil {
		panic(err)
	}
}
