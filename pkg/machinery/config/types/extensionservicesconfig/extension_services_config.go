// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package extensionservicesconfig provides extensions config documents.
package extensionservicesconfig

import (
	"errors"
	"fmt"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

//go:generate deep-copy -type V1Alpha1 -pointer-receiver -header-file ../../../../../hack/boilerplate.txt -o deep_copy.generated.go .

// Kind is a Extension config document kind.
const Kind = "ExtensionServicesConfig"

func init() {
	registry.Register(Kind, func(version string) config.Document {
		switch version {
		case "v1alpha1":
			return &V1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.ExtensionServicesConfigConfig = &V1Alpha1{}
	_ config.Document                      = &V1Alpha1{}
	_ config.Validator                     = &V1Alpha1{}
)

// V1Alpha1 is a extensionservicesconfig document.
type V1Alpha1 struct {
	meta.Meta `yaml:",inline"`
	Config    []ExtensionServiceConfig `yaml:"config"`
}

// ExtensionServiceConfig is a config for extension services.
type ExtensionServiceConfig struct {
	ExtensionName               string                       `yaml:"name"`
	ExtensionServiceConfigFiles []ExtensionServiceConfigFile `yaml:"configFiles"`
}

// ExtensionServiceConfigFile is a config file for extension services.
type ExtensionServiceConfigFile struct {
	ExtensionContent   string `yaml:"content"`
	ExtensionMountPath string `yaml:"mountPath"`
}

// NewExtensionServicesConfigV1Alpha1 creates a new siderolink config document.
func NewExtensionServicesConfigV1Alpha1() *V1Alpha1 {
	return &V1Alpha1{
		Meta: meta.Meta{
			MetaKind:       Kind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

// Clone implements config.Document interface.
func (e *V1Alpha1) Clone() config.Document {
	return e.DeepCopy()
}

// Validate implements config.Validatator interface.
func (e *V1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	if len(e.Config) == 0 {
		return nil, errors.New("no extensions config found")
	}

	for _, ext := range e.Config {
		if ext.ExtensionName == "" {
			return nil, errors.New("extension name is required")
		}

		if len(ext.ExtensionServiceConfigFiles) == 0 {
			return nil, fmt.Errorf("no config files found for extension %q", ext.ExtensionName)
		}

		for _, file := range ext.ExtensionServiceConfigFiles {
			if file.ExtensionContent == "" {
				return nil, fmt.Errorf("extension content is required for extension %q", ext.ExtensionName)
			}

			if file.ExtensionMountPath == "" {
				return nil, fmt.Errorf("extension mount path is required for extension %q", ext.ExtensionName)
			}
		}
	}

	return nil, nil
}

// ExtensionsCfg implements config.ExtensionsConfig interface.
func (e *V1Alpha1) ExtensionsCfg() config.ExtensionServicesConfigConfig {
	return e
}

// ConfigData implements config.ExtensionConfig interface.
func (e *V1Alpha1) ConfigData() []config.ExtensionServicesConfig {
	return xslices.Map(e.Config, func(c ExtensionServiceConfig) config.ExtensionServicesConfig {
		return &ExtensionServiceConfig{
			ExtensionName:               c.ExtensionName,
			ExtensionServiceConfigFiles: c.ExtensionServiceConfigFiles,
		}
	})
}

// Name implements config.ExtensionConfig interface.
func (e *ExtensionServiceConfig) Name() string {
	return e.ExtensionName
}

// ConfigFiles implements config.ExtensionConfig interface.
func (e *ExtensionServiceConfig) ConfigFiles() []config.ExtensionServicesConfigFile {
	return xslices.Map(e.ExtensionServiceConfigFiles, func(c ExtensionServiceConfigFile) config.ExtensionServicesConfigFile {
		return &ExtensionServiceConfigFile{
			ExtensionContent:   c.ExtensionContent,
			ExtensionMountPath: c.ExtensionMountPath,
		}
	})
}

// Content implements config.ConfigFile interface.
func (e *ExtensionServiceConfigFile) Content() string {
	return e.ExtensionContent
}

// Path implements config.ConfigFile interface.
func (e *ExtensionServiceConfigFile) Path() string {
	return e.ExtensionMountPath
}
