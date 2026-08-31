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

// WifiSpecType is type of WifiSpec resource.
const WifiSpecType = resource.Type("WifiSpecs.net.talos.dev")

// WifiSpec resource holds WiFi network link configuration.
type WifiSpec = typed.Resource[WifiSpecSpec, WifiSpecExtension]

// WifiSpecSpec describes WiFi configuration of a wireless link.
//
//gotagsrewrite:gen
type WifiSpecSpec struct {
	CountryCode string        `yaml:"countryCode,omitempty" protobuf:"1"`
	Networks    []WifiNetwork `yaml:"networks" protobuf:"2"`
}

// WifiNetwork describes a single WiFi network to connect to.
//
//gotagsrewrite:gen
type WifiNetwork struct {
	SSID   string `yaml:"ssid" protobuf:"1"`
	PSK    string `yaml:"psk,omitempty" protobuf:"2"`
	Hidden bool   `yaml:"hidden,omitempty" protobuf:"3"`
}

// NewWifiSpec initializes a WifiSpec resource.
func NewWifiSpec(namespace resource.Namespace, id resource.ID) *WifiSpec {
	return typed.NewResource[WifiSpecSpec, WifiSpecExtension](
		resource.NewMetadata(namespace, WifiSpecType, id, resource.VersionUndefined),
		WifiSpecSpec{},
	)
}

// WifiSpecExtension provides auxiliary methods for WifiSpec.
type WifiSpecExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (WifiSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             WifiSpecType,
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[WifiSpecSpec](WifiSpecType, &WifiSpec{})
	if err != nil {
		panic(err)
	}
}
