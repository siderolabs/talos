// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// FSScrubConfigType is type of FSScrubConfig resource.
const FSScrubConfigType = resource.Type("FSScrubConfigs.runtime.talos.dev")

// FSScrubConfig resource holds configuration for watchdog timer.
type FSScrubConfig = typed.Resource[FSScrubConfigSpec, FSScrubConfigExtension]

// FSScrubConfigID is a resource ID for FSScrubConfig.
const FSScrubConfigID resource.ID = "scrub"

// FilesystemScrubConfig represents mirror configuration for a registry.
//
//gotagsrewrite:gen
type FilesystemScrubConfig struct {
	Mountpoint string        `yaml:"mountpoint" protobuf:"1"`
	Period     time.Duration `yaml:"period" protobuf:"2"`
}

// FSScrubConfigSpec describes configuration of watchdog timer.
//
//gotagsrewrite:gen
type FSScrubConfigSpec struct {
	Filesystems []FilesystemScrubConfig `yaml:"filesystems,omitempty" protobuf:"1"`
}

// DeepCopy implements DeepCopyable.
func (r FSScrubConfigSpec) DeepCopy() FSScrubConfigSpec {
	return FSScrubConfigSpec{
		Filesystems: append([]FilesystemScrubConfig{}, r.Filesystems...),
	}
}

// NewFSScrubConfig initializes a FSScrubConfig resource.
func NewFSScrubConfig() *FSScrubConfig {
	return typed.NewResource[FSScrubConfigSpec, FSScrubConfigExtension](
		resource.NewMetadata(NamespaceName, FSScrubConfigType, FSScrubConfigID, resource.VersionUndefined),
		FSScrubConfigSpec{},
	)
}

// FSScrubConfigExtension is auxiliary resource data for FSScrubConfig.
type FSScrubConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (FSScrubConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             FSScrubConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[FSScrubConfigSpec](FSScrubConfigType, &FSScrubConfig{})
	if err != nil {
		panic(err)
	}
}
