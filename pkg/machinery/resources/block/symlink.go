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

// SymlinkType is type of Symlink resource.
const SymlinkType = resource.Type("BlockSymlinks.block.talos.dev")

// Symlink resource holds a list of stable symlinks to the blockdevice.
type Symlink = typed.Resource[SymlinkSpec, SymlinkExtension]

// SymlinkID is the singleton resource ID.
const SymlinkID resource.ID = "system-disk"

// SymlinkSpec is the spec for Symlinks resource.
//
//gotagsrewrite:gen
type SymlinkSpec struct {
	Paths []string `yaml:"paths" protobuf:"1"`
}

// NewSymlink initializes a BlockSymlink resource.
func NewSymlink(namespace resource.Namespace, id resource.ID) *Symlink {
	return typed.NewResource[SymlinkSpec, SymlinkExtension](
		resource.NewMetadata(namespace, SymlinkType, id, resource.VersionUndefined),
		SymlinkSpec{},
	)
}

// SymlinkExtension is auxiliary resource data for BlockSymlink.
type SymlinkExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (SymlinkExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SymlinkType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[SymlinkSpec](SymlinkType, &Symlink{})
	if err != nil {
		panic(err)
	}
}
