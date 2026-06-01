// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// KubePrismConfigType is type of KubePrismConfig resource.
const KubePrismConfigType = resource.Type("KubePrismConfigs.kubernetes.talos.dev")

// KubePrismConfigID the singleton config resource ID.
const KubePrismConfigID = resource.ID("k8s-loadbalancer-config")

// KubePrismConfig resource holds load balancer health data.
type KubePrismConfig = typed.Resource[KubePrismConfigSpec, KubePrismConfigExtension]

// NewKubePrismConfig initializes an KubePrismConfig resource.
func NewKubePrismConfig(namespace resource.Namespace, id resource.ID) *KubePrismConfig {
	return typed.NewResource[KubePrismConfigSpec, KubePrismConfigExtension](
		resource.NewMetadata(namespace, KubePrismConfigType, id, resource.VersionUndefined),
		KubePrismConfigSpec{},
	)
}

// KubePrismConfigSpec describes KubePrismConfig data.
//
//gotagsrewrite:gen
type KubePrismConfigSpec struct {
	Host      string              `yaml:"host" protobuf:"1"`
	Port      int                 `yaml:"port" protobuf:"2"`
	Endpoints []KubePrismEndpoint `yaml:"endpoints" protobuf:"3"`
}

// KubePrismConfigExtension provides auxiliary methods for KubePrismConfig.
type KubePrismConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (KubePrismConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubePrismConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Host",
				JSONPath: "{.host}",
			},
			{
				Name:     "Port",
				JSONPath: "{.port}",
			},
			{
				Name:     "Endpoints",
				JSONPath: "{.endpoints}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[KubePrismConfigSpec](KubePrismConfigType, &KubePrismConfig{})
	if err != nil {
		panic(err)
	}
}
