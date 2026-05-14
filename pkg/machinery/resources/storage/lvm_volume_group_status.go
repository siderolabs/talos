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

// LVMVolumeGroupStatusType is the type of LVMVolumeGroupStatus resource.
const LVMVolumeGroupStatusType = resource.Type("LVMVolumeGroupStatuses.storage.talos.dev")

// LVMVolumeGroupStatus resource contains information about the LVM volume group status.
type LVMVolumeGroupStatus = typed.Resource[LVMVolumeGroupStatusSpec, LVMVolumeGroupStatusExtension]

// LVMVolumeGroupStatusSpec is the spec for LVMVolumeGroupStatus resource.
//
// Fields mirror selected columns of `vgs -a -o +all --reportformat json --units b --nosuffix`.
// Numeric / tri-state columns are exposed as raw strings so LVM's sentinels
// ("", "-1", "unknown", "unmanaged", "auto", …) are surfaced verbatim. See
// vgs(8) for the source-of-truth definitions of each column.
//
//gotagsrewrite:gen
type LVMVolumeGroupStatusSpec struct {
	// Name is the volume group name (vg_name).
	Name string `yaml:"name" protobuf:"1"`
	// UUID is the stable VG identifier (vg_uuid).
	UUID string `yaml:"uuid" protobuf:"2"`
	// Format is the on-disk metadata format (vg_fmt), e.g. "lvm2".
	Format string `yaml:"format" protobuf:"3"`
	// Permissions reflects vg_permissions ("writeable" / "read-only" / "unknown" / "").
	Permissions string `yaml:"permissions" protobuf:"4"`
	// Extendable is the raw vg_extendable column ("extendable" / "").
	Extendable string `yaml:"extendable" protobuf:"5"`
	// Exported is the raw vg_exported column ("exported" / "").
	Exported string `yaml:"exported" protobuf:"6"`
	// Partial is the raw vg_partial column ("partial" / "").
	Partial string `yaml:"partial" protobuf:"7"`
	// AllocationPolicy reflects vg_allocation_policy.
	AllocationPolicy string `yaml:"allocationPolicy" protobuf:"8"`
	// Clustered is the raw vg_clustered column ("clustered" / "").
	Clustered string `yaml:"clustered" protobuf:"9"`
	// Shared is the raw vg_shared column ("shared" / "").
	Shared string `yaml:"shared" protobuf:"10"`
	// Size is the raw vg_size column in bytes.
	Size string `yaml:"size" protobuf:"11"`
	// Free is the raw vg_free column in bytes.
	Free string `yaml:"free" protobuf:"12"`
	// ExtentSize is the raw vg_extent_size column in bytes.
	ExtentSize string `yaml:"extentSize" protobuf:"13"`
	// ExtentCount is the raw vg_extent_count column.
	ExtentCount string `yaml:"extentCount" protobuf:"14"`
	// FreeExtentCount is the raw vg_free_count column.
	FreeExtentCount string `yaml:"freeExtentCount" protobuf:"15"`
	// MaxLV is the raw max_lv column; "0" means no limit.
	MaxLV string `yaml:"maxLV" protobuf:"16"`
	// MaxPV is the raw max_pv column; "0" means no limit.
	MaxPV string `yaml:"maxPV" protobuf:"17"`
	// LVCount is the raw lv_count column.
	LVCount string `yaml:"lvCount" protobuf:"18"`
	// PVCount is the raw pv_count column.
	PVCount string `yaml:"pvCount" protobuf:"19"`
	// SnapCount is the raw snap_count column.
	SnapCount string `yaml:"snapCount" protobuf:"20"`
	// MissingPVCount is the raw vg_missing_pv_count column.
	MissingPVCount string `yaml:"missingPVCount" protobuf:"21"`
	// SeqNo is the raw vg_seqno column.
	SeqNo string `yaml:"seqNo" protobuf:"22"`
	// LockType is the raw vg_lock_type column.
	LockType string `yaml:"lockType" protobuf:"23"`
	// SystemID is the system_id stamped into VG metadata (vg_systemid).
	SystemID string `yaml:"systemID" protobuf:"24"`
	// Tags is the list of tags attached to the VG (vg_tags).
	Tags []string `yaml:"tags" protobuf:"25"`
}

// NewLVMVolumeGroupStatus initializes a LVMVolumeGroupStatus resource.
func NewLVMVolumeGroupStatus(namespace resource.Namespace, id resource.ID) *LVMVolumeGroupStatus {
	return typed.NewResource[LVMVolumeGroupStatusSpec, LVMVolumeGroupStatusExtension](
		resource.NewMetadata(namespace, LVMVolumeGroupStatusType, id, resource.VersionUndefined),
		LVMVolumeGroupStatusSpec{},
	)
}

// LVMVolumeGroupStatusExtension is auxiliary resource data for LVMVolumeGroupStatus.
type LVMVolumeGroupStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (LVMVolumeGroupStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LVMVolumeGroupStatusType,
		Aliases:          []resource.Type{"vg"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{Name: "Name", JSONPath: "{.name}"},
			{Name: "Permissions", JSONPath: "{.permissions}"},
			{Name: "Size", JSONPath: "{.size}"},
			{Name: "Free", JSONPath: "{.free}"},
			{Name: "PVs", JSONPath: "{.pvCount}"},
			{Name: "LVs", JSONPath: "{.lvCount}"},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(LVMVolumeGroupStatusType, &LVMVolumeGroupStatus{})
	if err != nil {
		panic(err)
	}
}
