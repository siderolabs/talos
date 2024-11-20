// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ImageCacheConfigType is type of ImageCacheConfig resource.
const ImageCacheConfigType = resource.Type("ImageCacheConfigs.cri.talos.dev")

// ImageCacheConfig represents ImageCacheConfig typed resource.
type ImageCacheConfig = typed.Resource[ImageCacheConfigSpec, ImageCacheConfigExtension]

// ImageCacheConfigID is the ID of the ImageCacheConfig resource.
const ImageCacheConfigID = "image-cache"

// ImageCacheConfigSpec represents the ImageCacheConfig.
//
//gotagsrewrite:gen
type ImageCacheConfigSpec struct {
	Status ImageCacheStatus `yaml:"status" protobuf:"1"`
	Roots  []string         `yaml:"roots" protobuf:"2"`
}

// NewImageCacheConfig creates new ImageCacheConfig object.
func NewImageCacheConfig() *ImageCacheConfig {
	return typed.NewResource[ImageCacheConfigSpec, ImageCacheConfigExtension](
		resource.NewMetadata(NamespaceName, ImageCacheConfigType, ImageCacheConfigID, resource.VersionUndefined),
		ImageCacheConfigSpec{},
	)
}

// ImageCacheConfigExtension is an auxiliary type for ImageCacheConfig resource.
type ImageCacheConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ImageCacheConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ImageCacheConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Status",
				JSONPath: "{.status}",
			},
			{
				Name:     "Roots",
				JSONPath: "{.roots}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[ImageCacheConfigSpec](ImageCacheConfigType, &ImageCacheConfig{})
	if err != nil {
		panic(err)
	}
}
