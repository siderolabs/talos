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

// LVMLogicalVolumeStatusType is the type of LVMLogicalVolumeStatus resource.
const LVMLogicalVolumeStatusType = resource.Type("LVMLogicalVolumeStatuses.storage.talos.dev")

// LVMLogicalVolumeStatus resource contains information about the LVM logical volume status.
type LVMLogicalVolumeStatus = typed.Resource[LVMLogicalVolumeStatusSpec, LVMLogicalVolumeStatusExtension]

// LVMLogicalVolumeStatusSpec is the spec for LVMLogicalVolumeStatus resource.
//
// Fields mirror selected columns of `lvs -a -o +all --reportformat json --units b --nosuffix`.
// See lvs(8) for the source-of-truth definitions of each column.
//
//gotagsrewrite:gen
type LVMLogicalVolumeStatusSpec struct {
	// Path is the LV path (lv_path), e.g. /dev/vg0/data. Empty for hidden LVs.
	Path string `yaml:"path" protobuf:"1"`
	// DMPath is the device-mapper path (lv_dm_path), always populated when active.
	DMPath string `yaml:"dmPath" protobuf:"2"`
	// Name is the LV short name (lv_name).
	Name string `yaml:"name" protobuf:"3"`
	// FullName is the qualified LV name (lv_full_name), e.g. "vg0/data".
	FullName string `yaml:"fullName" protobuf:"4"`
	// VGName is the parent volume group name (vg_name).
	VGName string `yaml:"vgName" protobuf:"5"`
	// UUID is the stable LV identifier (lv_uuid).
	UUID string `yaml:"uuid" protobuf:"6"`
	// Layout is the LV layout (lv_layout), e.g. "linear", "raid1", "thin,pool".
	Layout string `yaml:"layout" protobuf:"7"`
	// Role is the LV role (lv_role), e.g. "public", "private,thin,pool,data".
	Role string `yaml:"role" protobuf:"8"`
	// Permissions reflects lv_permissions.
	Permissions string `yaml:"permissions" protobuf:"9"`
	// AllocationPolicy reflects lv_allocation_policy.
	AllocationPolicy string `yaml:"allocationPolicy" protobuf:"10"`
	// AllocationLocked is the raw lv_allocation_locked column ("locked" / "").
	AllocationLocked string `yaml:"allocationLocked" protobuf:"11"`
	// FixedMinor is the raw lv_fixed_minor column ("fixed" / "").
	FixedMinor string `yaml:"fixedMinor" protobuf:"12"`
	// Active is the raw lv_active column ("active" / "" / "unknown").
	Active string `yaml:"active" protobuf:"13"`
	// ActiveLocally is the raw lv_active_locally column ("active locally" / "" / "unknown").
	ActiveLocally string `yaml:"activeLocally" protobuf:"14"`
	// ActiveRemotely is the raw lv_active_remotely column ("active remotely" / "" / "unknown").
	ActiveRemotely string `yaml:"activeRemotely" protobuf:"15"`
	// ActiveExclusively is the raw lv_active_exclusively column ("active exclusively" / "" / "unknown").
	ActiveExclusively string `yaml:"activeExclusively" protobuf:"16"`
	// Suspended is the raw lv_suspended column ("suspended" / "" / "unknown").
	Suspended string `yaml:"suspended" protobuf:"17"`
	// DeviceOpen is the raw lv_device_open column ("open" / "" / "unknown").
	DeviceOpen string `yaml:"deviceOpen" protobuf:"18"`
	// SkipActivation is the raw lv_skip_activation column ("skip activation" / "").
	SkipActivation string `yaml:"skipActivation" protobuf:"19"`
	// Merging is the raw lv_merging column ("merging" / "").
	Merging string `yaml:"merging" protobuf:"20"`
	// Converting is the raw lv_converting column ("converting" / "").
	Converting string `yaml:"converting" protobuf:"21"`
	// Size is the raw lv_size column in bytes.
	Size string `yaml:"size" protobuf:"22"`
	// MetadataSize is the raw lv_metadata_size column in bytes ("" when not applicable).
	MetadataSize string `yaml:"metadataSize" protobuf:"23"`
	// ReadAhead is the raw lv_read_ahead column ("auto" or a byte count).
	ReadAhead string `yaml:"readAhead" protobuf:"24"`
	// KernelMajor is the raw lv_kernel_major column ("-1" when inactive, otherwise a number).
	KernelMajor string `yaml:"kernelMajor" protobuf:"25"`
	// KernelMinor is the raw lv_kernel_minor column ("-1" when inactive, otherwise a number).
	KernelMinor string `yaml:"kernelMinor" protobuf:"26"`
	// Origin is the LV name this LV is a snapshot/thin clone of (origin); empty otherwise.
	Origin string `yaml:"origin" protobuf:"27"`
	// OriginSize is the raw origin_size column in bytes ("" when not applicable).
	OriginSize string `yaml:"originSize" protobuf:"28"`
	// PoolLV is the parent thin/cache pool name (pool_lv); empty when not pool-backed.
	PoolLV string `yaml:"poolLV" protobuf:"29"`
	// DataLV is the data sub-LV name (data_lv); empty when not applicable.
	DataLV string `yaml:"dataLV" protobuf:"30"`
	// MetadataLV is the metadata sub-LV name (metadata_lv); empty when not applicable.
	MetadataLV string `yaml:"metadataLV" protobuf:"31"`
	// MovePV is the source PV of an in-progress pvmove (move_pv); empty otherwise.
	MovePV string `yaml:"movePV" protobuf:"32"`
	// ConvertLV is the target LV of an in-progress lvconvert (convert_lv); empty otherwise.
	ConvertLV string `yaml:"convertLV" protobuf:"33"`
	// WhenFull reflects lv_when_full ("error" / "queue" / "").
	WhenFull string `yaml:"whenFull" protobuf:"34"`
	// Tags is the list of tags attached to the LV (lv_tags).
	Tags []string `yaml:"tags" protobuf:"35"`
}

// NewLVMLogicalVolumeStatus initializes a LVMLogicalVolumeStatus resource.
func NewLVMLogicalVolumeStatus(namespace resource.Namespace, id resource.ID) *LVMLogicalVolumeStatus {
	return typed.NewResource[LVMLogicalVolumeStatusSpec, LVMLogicalVolumeStatusExtension](
		resource.NewMetadata(namespace, LVMLogicalVolumeStatusType, id, resource.VersionUndefined),
		LVMLogicalVolumeStatusSpec{},
	)
}

// LVMLogicalVolumeStatusExtension is auxiliary resource data for LVMLogicalVolumeStatus.
type LVMLogicalVolumeStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (LVMLogicalVolumeStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LVMLogicalVolumeStatusType,
		Aliases:          []resource.Type{"lv"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{Name: "Path", JSONPath: "{.path}"},
			{Name: "VG", JSONPath: "{.vgName}"},
			{Name: "Layout", JSONPath: "{.layout}"},
			{Name: "Size", JSONPath: "{.size}"},
			{Name: "Active", JSONPath: "{.active}"},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(LVMLogicalVolumeStatusType, &LVMLogicalVolumeStatus{})
	if err != nil {
		panic(err)
	}
}
