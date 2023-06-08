// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"net/url"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// KmsgLogConfigType is type of KmsgLogConfig resource.
const KmsgLogConfigType = resource.Type("KmsgLogConfigs.runtime.talos.dev")

// KmsgLogConfig resource holds configuration for kernel message log streaming.
type KmsgLogConfig = typed.Resource[KmsgLogConfigSpec, KmsgLogConfigExtension]

// KmsgLogConfigID is a resource ID for KmsgLogConfig.
const KmsgLogConfigID resource.ID = "kmsg-log"

// KmsgLogConfigSpec describes configuration for kmsg log streaming.
//
//gotagsrewrite:gen
type KmsgLogConfigSpec struct {
	Destinations []*url.URL `yaml:"destinations" protobuf:"1"`
}

// NewKmsgLogConfig initializes a KmsgLogConfig resource.
func NewKmsgLogConfig() *KmsgLogConfig {
	return typed.NewResource[KmsgLogConfigSpec, KmsgLogConfigExtension](
		resource.NewMetadata(NamespaceName, KmsgLogConfigType, KmsgLogConfigID, resource.VersionUndefined),
		KmsgLogConfigSpec{},
	)
}

// KmsgLogConfigExtension is auxiliary resource data for KmsgLogConfig.
type KmsgLogConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (KmsgLogConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KmsgLogConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[KmsgLogConfigSpec](KmsgLogConfigType, &KmsgLogConfig{})
	if err != nil {
		panic(err)
	}
}
