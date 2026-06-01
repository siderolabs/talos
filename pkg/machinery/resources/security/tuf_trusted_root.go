// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:revive
package security

import (
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// TUFTrustedRootType is type of TUFTrustedRoot resource.
const TUFTrustedRootType = resource.Type("TUFTrustedRoots.security.talos.dev")

// TUFTrustedRoot represents TUFTrustedRoot typed resource.
type TUFTrustedRoot = typed.Resource[TUFTrustedRootSpec, TUFTrustedRootExtension]

// TrustedRootID is the ID for the TUF trusted root resource.
const TrustedRootID = resource.ID("trusted_root.json")

// TUFTrustedRootSpec represents a sigstore's TUF trusted root information.
//
//gotagsrewrite:gen
type TUFTrustedRootSpec struct {
	// LastRefreshTime is the last time the trusted root was refreshed.
	LastRefreshTime time.Time `yaml:"lastRefreshTime,omitempty" protobuf:"1"`
	// JSONData is the trusted root data in JSON format.
	JSONData string `yaml:"jsonData,omitempty" protobuf:"2"`
}

// NewTUFTrustedRoot creates new TUFTrustedRoot object.
func NewTUFTrustedRoot(id resource.ID) *TUFTrustedRoot {
	return typed.NewResource[TUFTrustedRootSpec, TUFTrustedRootExtension](
		resource.NewMetadata(NamespaceName, TUFTrustedRootType, id, resource.VersionUndefined),
		TUFTrustedRootSpec{},
	)
}

// TUFTrustedRootExtension is an auxiliary type for TUFTrustedRoot resource.
type TUFTrustedRootExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (TUFTrustedRootExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             TUFTrustedRootType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Last Refresh",
				JSONPath: "{.lastRefreshTime}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(TUFTrustedRootType, &TUFTrustedRoot{})
	if err != nil {
		panic(err)
	}
}
