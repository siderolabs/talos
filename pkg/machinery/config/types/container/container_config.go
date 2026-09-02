// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// ContainerConfigKind is a config document kind.
const ContainerConfigKind = "ContainerConfig"

// maxNameLength is the maximum length of a container name.
const maxNameLength = 63

// validNamePattern matches the characters a container name may contain.
var validNamePattern = regexp.MustCompile(`^[a-z0-9-]+$`)

func init() {
	registry.Register(ContainerConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1":
			return &ContainerConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.ContainerConfig = &ContainerConfigV1Alpha1{}
	_ config.NamedDocument   = &ContainerConfigV1Alpha1{}
	_ config.Validator       = &ContainerConfigV1Alpha1{}
)

// ContainerConfigV1Alpha1 is a container configuration document.
//
//	description: |
//	  ContainerConfig declares a container to be run by Talos directly, without Kubernetes.
//
//	  The container is started as soon as the configuration is applied, with no image rebuild
//	  and no reboot. It runs against the CRI containerd instance in the dedicated
//	  `taloscontainers` namespace, and is restarted automatically 5 seconds after it stops.
//
//	  Containers are not Talos services: they do not appear in `talosctl services`, and
//	  `talosctl service` does not apply to them. Status is reported via `ContainerStatus`.
//	examples:
//	  - value: exampleContainerConfigV1Alpha1()
//	alias: ContainerConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/ContainerConfig
type ContainerConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the container.
	//
	//     Must be between 1 and 63 characters long, and can only contain lowercase ASCII
	//     letters, digits and hyphens. It is used as the containerd container ID and as the
	//     container's log identifier, so it may not collide with a Talos service name.
	MetaName string `yaml:"name"`
	//   description: |
	//     OCI image reference supplying the container's root filesystem.
	//
	//     A digest-pinned reference (`repo@sha256:...`) is recommended: it is the only form
	//     that guarantees the same bytes on every pull. Short references are accepted and
	//     normalized, so `nginx` becomes `index.docker.io/library/nginx:latest`.
	//   examples:
	//     - value: '"docker.io/library/nginx:1.27"'
	ContainerImage string `yaml:"image"`
	//   description: |
	//     Overrides the image's ENTRYPOINT.
	//
	//     Unset means the image's own entrypoint is used.
	ContainerEntrypoint []string `yaml:"entrypoint,omitempty"`
	//   description: |
	//     Overrides the image's CMD.
	ContainerArgs []string `yaml:"args,omitempty"`
	//   description: |
	//     Overrides the image's WORKDIR.
	ContainerWorkingDir string `yaml:"workingDir,omitempty"`
	//   description: |
	//     Overrides the image's USER uid and/or gid.
	//
	//     There are no user namespaces, so a container running as uid 0 is root on the host.
	RunAsConfig *ContainerRunAs `yaml:"runAs,omitempty"`
	//   description: |
	//     Environment variables, in `KEY=value` form, merged over the image's own ENV.
	//
	//     Values are stored in the machine configuration verbatim, so treat anything put here
	//     as being as sensitive as the machine configuration itself.
	ContainerEnvironment []string `yaml:"environment,omitempty"`
	//   description: |
	//     Filesystems to mount into the container.
	MountsConfig []ContainerMount `yaml:"mounts,omitempty"`
	//   description: |
	//     Security settings for the container.
	SecurityConfig *ContainerSecurity `yaml:"security,omitempty"`
	//   description: |
	//     Network settings for the container.
	NetworkConfig *ContainerNetwork `yaml:"network,omitempty"`
	//   description: |
	//     Resource limits, applied as cgroup v2 settings.
	ResourcesConfig *ContainerResources `yaml:"resources,omitempty"`
	//   description: |
	//     Conditions which must be satisfied before the container is started.
	DependsOnConfig *ContainerDependsOn `yaml:"dependsOn,omitempty"`
}

// NewContainerConfigV1Alpha1 creates a new container config document.
func NewContainerConfigV1Alpha1() *ContainerConfigV1Alpha1 {
	return &ContainerConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       ContainerConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleContainerConfigV1Alpha1() *ContainerConfigV1Alpha1 {
	cfg := NewContainerConfigV1Alpha1()
	cfg.MetaName = "nginx"
	cfg.ContainerImage = "docker.io/library/nginx:1.27"
	cfg.ContainerEnvironment = []string{"NGINX_PORT=8080"}
	cfg.MountsConfig = []ContainerMount{
		{
			UserVolumeMount: &UserVolumeMount{
				VolumeName:       "web-content",
				MountDestination: "/usr/share/nginx/html",
				MountOpts:        []string{"ro"},
			},
		},
		{
			TmpfsMount: &TmpfsMount{
				MountDestination: "/tmp",
				MountSize:        "64MiB",
			},
		},
	}
	cfg.ResourcesConfig = &ContainerResources{
		Limits: &ContainerResourceLimits{
			CPU:    "1500m",
			Memory: "512MiB",
		},
	}
	cfg.DependsOnConfig = &ContainerDependsOn{
		PathsConfig:    []string{"/var/mnt/web-content"},
		NetworksConfig: []string{"addresses"},
		TimeConfig:     new(true),
	}

	return cfg
}

// Name implements config.NamedDocument interface.
func (c *ContainerConfigV1Alpha1) Name() string {
	return c.MetaName
}

// Clone implements config.Document interface.
func (c *ContainerConfigV1Alpha1) Clone() config.Document {
	return c.DeepCopy()
}

// ContainerConfigSignal is a signal for container config.
func (c *ContainerConfigV1Alpha1) ContainerConfigSignal() {}

// Image implements config.ContainerConfig interface.
//
// The reference is returned in canonical form. Validation has already rejected anything
// unparseable, so the error path here is unreachable in practice.
func (c *ContainerConfigV1Alpha1) Image() string {
	normalized, err := c.NormalizedImage()
	if err != nil {
		return c.ContainerImage
	}

	return normalized
}

// Entrypoint implements config.ContainerConfig interface.
func (c *ContainerConfigV1Alpha1) Entrypoint() []string { return c.ContainerEntrypoint }

// Args implements config.ContainerConfig interface.
func (c *ContainerConfigV1Alpha1) Args() []string { return c.ContainerArgs }

// WorkingDir implements config.ContainerConfig interface.
func (c *ContainerConfigV1Alpha1) WorkingDir() string { return c.ContainerWorkingDir }

// RunAs implements config.ContainerConfig interface.
func (c *ContainerConfigV1Alpha1) RunAs() config.ContainerRunAsConfig {
	if c.RunAsConfig == nil {
		return &ContainerRunAs{}
	}

	return c.RunAsConfig
}

// Environment implements config.ContainerConfig interface.
func (c *ContainerConfigV1Alpha1) Environment() []string { return c.ContainerEnvironment }

// NormalizedImage returns the image reference in canonical form.
//
// Short references such as `nginx` or `nginx:latest` are expanded to
// `index.docker.io/library/nginx:latest`.
//
// Note: this canonical form uses `index.docker.io`, not the `docker.io` host used by the
// containerd pull path and image GC elsewhere in Talos (both refer to the same registry). If
// those paths ever match against this value directly, the host will need reconciling.
func (c *ContainerConfigV1Alpha1) NormalizedImage() (string, error) {
	ref, err := name.ParseReference(c.ContainerImage)
	if err != nil {
		return "", fmt.Errorf("invalid image reference %q: %w", c.ContainerImage, err)
	}

	return ref.Name(), nil
}

// Validate implements config.Validator interface.
//
//nolint:gocyclo,cyclop
func (c *ContainerConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		warnings         []string
		validationErrors error
	)

	validationErrors = errors.Join(validationErrors, c.ValidateName())

	if c.ContainerImage == "" {
		validationErrors = errors.Join(validationErrors, errors.New("image is required"))
	} else {
		normalized, err := c.NormalizedImage()
		if err != nil {
			validationErrors = errors.Join(validationErrors, err)
		} else if _, digested := mustParse(normalized).(name.Digest); !digested {
			warnings = append(warnings,
				fmt.Sprintf("container %q: image %q is not digest-pinned, the running image may change on restart", c.MetaName, c.ContainerImage))
		}
	}

	for i, env := range c.ContainerEnvironment {
		if !strings.Contains(env, "=") {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("environment[%d]: %q must be in KEY=value form", i, env))
		}
	}

	validationErrors = errors.Join(validationErrors, c.ValidateMounts())

	if c.SecurityConfig != nil {
		validationErrors = errors.Join(validationErrors, c.SecurityConfig.Validate())
	}

	if c.NetworkConfig != nil {
		validationErrors = errors.Join(validationErrors, c.NetworkConfig.Validate())
	}

	if c.ResourcesConfig != nil {
		validationErrors = errors.Join(validationErrors, c.ResourcesConfig.Validate())
	}

	if c.DependsOnConfig != nil {
		validationErrors = errors.Join(validationErrors, c.DependsOnConfig.Validate(c.MetaName))
	}

	if c.RunAsConfig != nil {
		validationErrors = errors.Join(validationErrors, c.RunAsConfig.Validate())
	}

	return warnings, validationErrors
}

func (c *ContainerConfigV1Alpha1) ValidateMounts() error {
	var (
		validationErrors error
		destinations     = map[string]struct{}{}
	)

	for i, mount := range c.MountsConfig {
		destination, err := mount.Validate()
		if err != nil {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("mounts[%d]: %w", i, err))

			continue
		}

		if _, exists := destinations[destination]; exists {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("mounts[%d]: duplicate destination %q", i, destination))
		}

		destinations[destination] = struct{}{}
	}

	return validationErrors
}

// Mounts implements config.ContainerConfig interface.
func (c *ContainerConfigV1Alpha1) Mounts() []config.ContainerMountConfig {
	out := make([]config.ContainerMountConfig, 0, len(c.MountsConfig))

	for i := range c.MountsConfig {
		out = append(out, &c.MountsConfig[i])
	}

	return out
}

// Security implements config.ContainerConfig interface.
func (c *ContainerConfigV1Alpha1) Security() config.ContainerSecurityConfig {
	if c.SecurityConfig == nil {
		return &ContainerSecurity{}
	}

	return c.SecurityConfig
}

// Network implements config.ContainerConfig interface.
func (c *ContainerConfigV1Alpha1) Network() config.ContainerNetworkConfig {
	if c.NetworkConfig == nil {
		return &ContainerNetwork{}
	}

	return c.NetworkConfig
}

// Resources implements config.ContainerConfig interface.
func (c *ContainerConfigV1Alpha1) Resources() config.ContainerResourcesConfig {
	if c.ResourcesConfig == nil {
		return &ContainerResources{}
	}

	return c.ResourcesConfig
}

// DependsOn implements config.ContainerConfig interface.
func (c *ContainerConfigV1Alpha1) DependsOn() config.ContainerDependsOnConfig {
	if c.DependsOnConfig == nil {
		return &ContainerDependsOn{}
	}

	return c.DependsOnConfig
}

// mustParse re-parses an already-normalized reference. Normalization has succeeded by the time
// this is called, so a failure here is impossible.
func mustParse(ref string) name.Reference {
	parsed, err := name.ParseReference(ref)
	if err != nil {
		panic(fmt.Sprintf("normalized reference %q failed to re-parse: %s", ref, err))
	}

	return parsed
}

// ValidateName checks the container name.
func (c *ContainerConfigV1Alpha1) ValidateName() error {
	switch {
	case c.MetaName == "":
		return errors.New("name is required")
	case len(c.MetaName) > maxNameLength:
		return fmt.Errorf("name %q must be %d characters or fewer", c.MetaName, maxNameLength)
	case !validNamePattern.MatchString(c.MetaName):
		return fmt.Errorf("name %q: name can only contain lowercase ASCII letters, digits and hyphens", c.MetaName)
	}

	return nil
}

func ValidateAbsPath(kind, path string) error {
	if path == "" {
		return fmt.Errorf("%s is required", kind)
	}

	if !filepath.IsAbs(path) {
		return fmt.Errorf("%s %q must be an absolute path", kind, path)
	}

	return nil
}
