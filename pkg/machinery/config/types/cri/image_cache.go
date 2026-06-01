// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"errors"

	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

//docgen:jsonschema

// ImageCacheConfig defines the ImageCacheConfig configuration name.
const ImageCacheConfig = "ImageCacheConfig"

func init() {
	registry.Register(ImageCacheConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &ImageCacheConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.ImageCacheConfig             = &ImageCacheConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &ImageCacheConfigV1Alpha1{}
)

// ImageCacheConfigV1Alpha1 configures Image Cache feature.
//
//	examples:
//	  - value: exampleImageCacheConfigVAlpha1()
//	alias: ImageCacheConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/ImageCacheConfig
type ImageCacheConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Local (to the machine) image cache configuration.
	LocalConfig LocalImageCacheConfig `yaml:"local"`
}

// LocalImageCacheConfig configures local image cache.
type LocalImageCacheConfig struct {
	//   description: |
	//     Is the local image cache enabled.
	ConfigEnabled *bool `yaml:"enabled,omitempty"`
}

// NewImageCacheConfigV1Alpha1 creates a new ImageCacheConfig config document.
func NewImageCacheConfigV1Alpha1() *ImageCacheConfigV1Alpha1 {
	return &ImageCacheConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       ImageCacheConfig,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleImageCacheConfigVAlpha1() *ImageCacheConfigV1Alpha1 {
	cfg := NewImageCacheConfigV1Alpha1()
	cfg.LocalConfig.ConfigEnabled = new(true)

	return cfg
}

// Clone implements config.Document interface.
func (s *ImageCacheConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// LocalEnabled implements config.ImageCacheConfig interface.
func (s *ImageCacheConfigV1Alpha1) LocalEnabled() bool {
	return pointer.SafeDeref(s.LocalConfig.ConfigEnabled)
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *ImageCacheConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.ImageCacheConfig() != nil {
		return errors.New("image cache config is already set in v1alpha1 config")
	}

	return nil
}
