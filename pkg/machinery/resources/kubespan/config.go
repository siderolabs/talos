// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

import (
	"net/netip"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

//go:generate go tool github.com/siderolabs/deep-copy -type ConfigSpec -type EndpointSpec -type IdentitySpec -type PeerSpecSpec -type PeerStatusSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

// ConfigType is type of Config resource.
const ConfigType = resource.Type("KubeSpanConfigs.kubespan.talos.dev")

// ConfigID the singleton config resource ID.
const ConfigID = resource.ID("kubespan")

// Config resource holds KubeSpan configuration.
type Config = typed.Resource[ConfigSpec, ConfigExtension]

// ConfigSpec describes KubeSpan configuration..
//
//gotagsrewrite:gen
type ConfigSpec struct {
	Enabled      bool   `yaml:"enabled" protobuf:"1"`
	ClusterID    string `yaml:"clusterId" protobuf:"2"`
	SharedSecret string `yaml:"sharedSecret" protobuf:"3"`
	// Force routing via KubeSpan even if the peer connection is not up.
	ForceRouting bool `yaml:"forceRouting" protobuf:"4"`
	// Advertise Kubernetes pod networks or skip it completely.
	AdvertiseKubernetesNetworks bool `yaml:"advertiseKubernetesNetworks" protobuf:"5"`
	// Force kubeSpan MTU size.
	MTU uint32 `yaml:"mtu,omitempty" protobuf:"6"`
	// If not empty, filter advertised endpoints using the list of CIDRs.
	EndpointFilters []string `yaml:"endpointFilters,omitempty" protobuf:"7"`
	// Harvest endpoints from the peer statuses.
	HarvestExtraEndpoints bool `yaml:"harvestExtraEndpoints" protobuf:"8"`
	// Extra endpoints to announce.
	ExtraEndpoints []netip.AddrPort `yaml:"extraEndpoints,omitempty" protobuf:"9"`
	// If not empty, filter advertised networks using the list of CIDRs.
	ExcludeAdvertisedNetworks []netip.Prefix `yaml:"excludeAdvertisedNetworks,omitempty" protobuf:"10"`
}

// NewConfig initializes a Config resource.
func NewConfig(namespace resource.Namespace, id resource.ID) *Config {
	return typed.NewResource[ConfigSpec, ConfigExtension](
		resource.NewMetadata(namespace, ConfigType, id, resource.VersionUndefined),
		ConfigSpec{},
	)
}

// ConfigExtension provides auxiliary methods for Config.
type ConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: config.NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[ConfigSpec](ConfigType, &Config{})
	if err != nil {
		panic(err)
	}
}
