// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// KernelCmdlineType is type of KernelCmdline resource.
const KernelCmdlineType = resource.Type("KernelCmdlines.runtime.talos.dev")

// KernelCmdline resource holds configuration for kernel message log streaming.
type KernelCmdline = typed.Resource[KernelCmdlineSpec, KernelCmdlineExtension]

// KernelCmdlineID is a resource ID for KernelCmdline.
const KernelCmdlineID resource.ID = "cmdline"

// KernelCmdlineSpec presents kernel command line (contents of /proc/cmdline).
//
//gotagsrewrite:gen
type KernelCmdlineSpec struct {
	Cmdline string `yaml:"cmdline" protobuf:"1"`
}

// NewKernelCmdline initializes a KernelCmdline resource.
func NewKernelCmdline() *KernelCmdline {
	return typed.NewResource[KernelCmdlineSpec, KernelCmdlineExtension](
		resource.NewMetadata(NamespaceName, KernelCmdlineType, KernelCmdlineID, resource.VersionUndefined),
		KernelCmdlineSpec{},
	)
}

// KernelCmdlineExtension is auxiliary resource data for KernelCmdline.
type KernelCmdlineExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (KernelCmdlineExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KernelCmdlineType,
		Aliases:          []resource.Type{"cmdline"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Cmdline",
				JSONPath: "{.cmdline}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[KernelCmdlineSpec](KernelCmdlineType, &KernelCmdline{})
	if err != nil {
		panic(err)
	}
}
