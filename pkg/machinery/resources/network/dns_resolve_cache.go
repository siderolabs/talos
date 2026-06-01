// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// DNSResolveCacheType is type of DNSResolveCache resource.
const DNSResolveCacheType = resource.Type("DNSResolveCaches.net.talos.dev")

// DNSResolveCache resource holds DNS resolver info.
type DNSResolveCache = typed.Resource[DNSResolveCacheSpec, DNSResolveCacheExtension]

// DNSResolveCacheSpec describes DNS servers status.
//
//gotagsrewrite:gen
type DNSResolveCacheSpec struct {
	Status string `yaml:"status" protobuf:"1"`
}

// NewDNSResolveCache initializes a DNSResolveCache resource.
func NewDNSResolveCache(id resource.ID) *DNSResolveCache {
	return typed.NewResource[DNSResolveCacheSpec, DNSResolveCacheExtension](
		resource.NewMetadata(NamespaceName, DNSResolveCacheType, id, resource.VersionUndefined),
		DNSResolveCacheSpec{},
	)
}

// DNSResolveCacheExtension provides auxiliary methods for DNSResolveCache.
type DNSResolveCacheExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (DNSResolveCacheExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             DNSResolveCacheType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Status",
				JSONPath: "{.status}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[DNSResolveCacheSpec](DNSResolveCacheType, &DNSResolveCache{})
	if err != nil {
		panic(err)
	}
}
