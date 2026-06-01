// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ZswapStatusType is type of ZswapStatus resource.
const ZswapStatusType = resource.Type("ZswapStatuses.block.talos.dev")

// ZswapStatus resource holds status of zwap subsystem.
type ZswapStatus = typed.Resource[ZswapStatusSpec, ZswapStatusExtension]

// ZswapStatusID is the ID of the singleton ZswapStatus resource.
const ZswapStatusID resource.ID = "zswap"

// ZswapStatusSpec is the spec for ZswapStatus resource.
//
//gotagsrewrite:gen
type ZswapStatusSpec struct {
	TotalSizeBytes      uint64 `yaml:"totalSize" protobuf:"1"`
	TotalSizeHuman      string `yaml:"totalSizeHuman" protobuf:"2"`
	StoredPages         uint64 `yaml:"storedPages" protobuf:"3"`
	PoolLimitHit        uint64 `yaml:"poolLimitHit" protobuf:"4"`
	RejectReclaimFail   uint64 `yaml:"rejectReclaimFail" protobuf:"5"`
	RejectAllocFail     uint64 `yaml:"rejectAllocFail" protobuf:"6"`
	RejectKmemcacheFail uint64 `yaml:"rejectKmemcacheFail" protobuf:"7"`
	RejectCompressFail  uint64 `yaml:"rejectCompressFail" protobuf:"8"`
	RejectCompressPoor  uint64 `yaml:"rejectCompressPoor" protobuf:"9"`
	WrittenBackPages    uint64 `yaml:"writtenBackPages" protobuf:"10"`
}

// NewZswapStatus initializes a ZswapStatus resource.
func NewZswapStatus(namespace resource.Namespace, id resource.ID) *ZswapStatus {
	return typed.NewResource[ZswapStatusSpec, ZswapStatusExtension](
		resource.NewMetadata(namespace, ZswapStatusType, id, resource.VersionUndefined),
		ZswapStatusSpec{},
	)
}

// ZswapStatusExtension is auxiliary resource data for ZswapStatus.
type ZswapStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ZswapStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ZswapStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Total Size",
				JSONPath: "{.totalSizeHuman}",
			},
			{
				Name:     "Stored Pages",
				JSONPath: "{.storedPages}",
			},
			{
				Name:     "Written Back",
				JSONPath: "{.writtenBackPages}",
			},
			{
				Name:     "Pool Limit Hit",
				JSONPath: "{.poolLimitHit}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(ZswapStatusType, &ZswapStatus{})
	if err != nil {
		panic(err)
	}
}
