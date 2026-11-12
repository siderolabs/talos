// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extensions

//docgen:jsonschema

import (
	"fmt"
	"path"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/merge"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/extensions/services"
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
//	examples:
//	  - value: extensionServiceConfigV1Alpha1()
//	alias: ExtensionServiceConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/ExtensionServiceConfig
type ServiceConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the extension service.
	//   schemaRequired: true
	ServiceName string `yaml:"name"`
	//   description: |
	//     The config files for the extension service.
	ServiceConfigFiles ConfigFileList `yaml:"configFiles,omitempty"`
	//   description: |
	//     The environment for the extension service.
	ServiceEnvironment []string `yaml:"environment,omitempty"`
}

// ConfigFileList is a list of ConfigFiles.
//
//docgen:alias
type ConfigFileList []ConfigFile

// Merge the config files by mountPath.
func (list *ConfigFileList) Merge(other any) error {
	otherFiles, ok := other.(ConfigFileList)
	if !ok {
		return fmt.Errorf("unexpected type for config file merge %T", other)
	}

	for _, configFile := range otherFiles {
		if err := list.mergeConfigFile(configFile); err != nil {
			return err
		}
	}

	return nil
}

func (list *ConfigFileList) mergeConfigFile(configFile ConfigFile) error {
	var existing *ConfigFile

	for idx, cf := range *list {
		if cf.ConfigFileMountPath == configFile.ConfigFileMountPath {
			existing = &(*list)[idx]

			break
		}
	}

	if existing != nil {
		return merge.Merge(existing, &configFile)
	}

	*list = append(*list, configFile)

	return nil
}

// ConfigFile is a config file for extension services.
type ConfigFile struct {
	//   description: |
	//     The content of the extension service config file.
	ConfigFileContent string `yaml:"content"`
	//   description: |
	//     The mount path of the extension service config file.
	ConfigFileMountPath string `yaml:"mountPath"`
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
//
//nolint:gocyclo
func (e *ServiceConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	if e.ServiceName == "" {
		return nil, fmt.Errorf("name is required")
	}

	if !services.IsValidName(e.ServiceName) {
		return nil, fmt.Errorf("name %q is invalid", e.ServiceName)
	}

	if len(e.ServiceConfigFiles) == 0 && len(e.ServiceEnvironment) == 0 {
		if len(e.ServiceConfigFiles) == 0 {
			return nil, fmt.Errorf("no config files found for extension %q", e.ServiceName)
		}

		if len(e.ServiceEnvironment) == 0 {
			return nil, fmt.Errorf("no environment defined for extension %q", e.ServiceName)
		}
	}

	mountPaths := make(map[string]struct{}, len(e.ServiceConfigFiles))

	for _, file := range e.ServiceConfigFiles {
		if file.ConfigFileContent == "" {
			return nil, fmt.Errorf("extension content is required for extension %q", e.ServiceName)
		}

		if err := validateMountPath(file.ConfigFileMountPath, e.ServiceName); err != nil {
			return nil, err
		}

		if _, ok := mountPaths[file.ConfigFileMountPath]; ok {
			return nil, fmt.Errorf("extension mount path %q is duplicated for extension %q", file.ConfigFileMountPath, e.ServiceName)
		}

		mountPaths[file.ConfigFileMountPath] = struct{}{}
	}

	// files are stored at their mount paths, so one can't be a directory of another
	for _, file := range e.ServiceConfigFiles {
		for dir := path.Dir(file.ConfigFileMountPath); dir != "/"; dir = path.Dir(dir) {
			if _, ok := mountPaths[dir]; ok {
				return nil, fmt.Errorf("extension mount path %q is under %q for extension %q", file.ConfigFileMountPath, dir, e.ServiceName)
			}
		}
	}

	return nil, nil
}

func validateMountPath(mountPath, serviceName string) error {
	if mountPath == "" {
		return fmt.Errorf("extension mount path is required for extension %q", serviceName)
	}

	_, err := ConfigFileName(serviceName, mountPath)

	return err
}

// ConfigFileName returns the name of the config file for mountPath relative to the extension service directory.
//
// The file is stored at its mount path under the service directory, so the mount path must be absolute and clean.
func ConfigFileName(serviceName, mountPath string) (string, error) {
	if !path.IsAbs(mountPath) || mountPath != path.Clean(mountPath) || mountPath == "/" {
		return "", fmt.Errorf("extension mount path %q is invalid for extension %q", mountPath, serviceName)
	}

	return mountPath[1:], nil
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
