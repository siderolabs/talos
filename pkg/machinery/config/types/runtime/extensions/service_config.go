// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extensions

//docgen:jsonschema

import (
	"fmt"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// ServiceConfigKind is a Extension config document kind.
const ServiceConfigKind = "ExtensionServiceConfig"

func init() {
	registry.Register(ServiceConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1":
			return &ServiceConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.ExtensionServiceConfig = &ServiceConfigV1Alpha1{}
	_ config.Document               = &ServiceConfigV1Alpha1{}
	_ config.Validator              = &ServiceConfigV1Alpha1{}
)

// ServiceConfigV1Alpha1 is a extensionserviceconfig document.
//
//docgen:configuration
type ServiceConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`
	// The name of the extension service.
	ServiceName string `yaml:"name" docgen:"{'in': '1.7', 'optional': false}"`
	// The config files for the extension service.
	ServiceConfigFiles []ConfigFile `yaml:"configFiles,omitempty" docgen:"{'in': '1.7'}"`
	// The environment for the extension service.
	ServiceEnvironment []string `yaml:"environment,omitempty" docgen:"{'in': '1.7'}"`
}

// ConfigFile is a config file for extension services.
//
//docgen:configuration
type ConfigFile struct {
	// The content of the extension service config file.
	ConfigFileContent string `yaml:"content" docgen:"{'in': '1.7'}"`
	// The mount path of the extension service config file.
	ConfigFileMountPath string `yaml:"mountPath" docgen:"{'in': '1.7'}"`
}

// NewServicesConfigV1Alpha1 creates a new siderolink config document.
func NewServicesConfigV1Alpha1() *ServiceConfigV1Alpha1 {
	return &ServiceConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       ServiceConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

// Clone implements config.Document interface.
func (e *ServiceConfigV1Alpha1) Clone() config.Document {
	return e.DeepCopy()
}

// Validate implements config.Validatator interface.
func (e *ServiceConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	if e.ServiceName == "" {
		return nil, fmt.Errorf("name is required")
	}

	if len(e.ServiceConfigFiles) == 0 && len(e.ServiceEnvironment) == 0 {
		if len(e.ServiceConfigFiles) == 0 {
			return nil, fmt.Errorf("no config files found for extension %q", e.ServiceName)
		}

		if len(e.ServiceEnvironment) == 0 {
			return nil, fmt.Errorf("no environment defined for extension %q", e.ServiceName)
		}
	}

	for _, file := range e.ServiceConfigFiles {
		if file.ConfigFileContent == "" {
			return nil, fmt.Errorf("extension content is required for extension %q", e.ServiceName)
		}

		if file.ConfigFileMountPath == "" {
			return nil, fmt.Errorf("extension mount path is required for extension %q", e.ServiceName)
		}
	}

	return nil, nil
}

// Name implements config.ExtensionServiceConfig interface.
func (e *ServiceConfigV1Alpha1) Name() string {
	return e.ServiceName
}

// ConfigFiles implements config.ExtensionServiceConfig interface.
func (e *ServiceConfigV1Alpha1) ConfigFiles() []config.ExtensionServiceConfigFile {
	return xslices.Map(e.ServiceConfigFiles, func(c ConfigFile) config.ExtensionServiceConfigFile {
		return c
	})
}

// Environment implements config.ExtensionServiceConfig interface.
func (e *ServiceConfigV1Alpha1) Environment() []string {
	return e.ServiceEnvironment
}

// Content implements config.ExtensionServiceConfigFile interface.
func (e ConfigFile) Content() string {
	return e.ConfigFileContent
}

// MountPath implements config.ExtensionServiceConfigFile interface.
func (e ConfigFile) MountPath() string {
	return e.ConfigFileMountPath
}

func extensionServiceConfigV1Alpha1() *ServiceConfigV1Alpha1 {
	cfg := NewServicesConfigV1Alpha1()
	cfg.ServiceName = "nut-client"
	cfg.ServiceConfigFiles = []ConfigFile{
		{
			ConfigFileContent:   "MONITOR ${upsmonHost} 1 remote username password",
			ConfigFileMountPath: "/usr/local/etc/nut/upsmon.conf",
		},
	}
	cfg.ServiceEnvironment = []string{"NUT_UPS=upsname"}

	return cfg
}
