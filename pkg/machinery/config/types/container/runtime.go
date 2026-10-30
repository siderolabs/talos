// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"slices"

	"github.com/dustin/go-humanize"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
)

// validNetworkConditions are the network readiness conditions accepted in dependsOn.network.
//
// These mirror the conditions the network subsystem already reports.
var validNetworkConditions = []string{"addresses", "connectivity", "hostname", "etcfiles"}

// ContainerNetwork configures the container's network namespace.
type ContainerNetwork struct {
	//   description: |
	//     Network mode.
	//
	//     `none` gives the container its own empty network namespace with no host access.
	//     `host` shares the host network namespace, so the container sees every interface and
	//     can bind any port.
	//   values:
	//     - none
	//     - host
	//   schema:
	//     type: string
	NetworkMode config.ContainerNetworkMode `yaml:"mode,omitempty"`
}

// Check interfaces.
var (
	_ config.ContainerNetworkConfig   = &ContainerNetwork{}
	_ config.ContainerResourcesConfig = &ContainerResources{}
	_ config.ContainerDependsOnConfig = &ContainerDependsOn{}
)

// Mode implements config.ContainerNetworkConfig interface.
func (n *ContainerNetwork) Mode() config.ContainerNetworkMode {
	if n.NetworkMode == "" {
		return config.ContainerNetworkModeNone
	}

	return n.NetworkMode
}

// Validate checks the network settings.
func (n *ContainerNetwork) Validate() error {
	switch n.Mode() {
	case config.ContainerNetworkModeNone, config.ContainerNetworkModeHost:
		return nil
	default:
		return fmt.Errorf("unsupported network mode %q, expected none or host", n.NetworkMode)
	}
}

// ContainerResources configures cgroup v2 resource limits.
type ContainerResources struct {
	//   description: |
	//     Hard ceilings the container cannot exceed.
	Limits *ContainerResourceLimits `yaml:"limits,omitempty"`
}

// ContainerResourceLimits are hard ceilings.
type ContainerResourceLimits struct {
	//   description: |
	//     CPU ceiling in millicores, mapped onto cgroup v2 `cpu.max`.
	//
	//     `1000m` is one core.
	//   examples:
	//     - value: '"1500m"'
	//   schema:
	//     type: string
	CPU string `yaml:"cpu,omitempty"`
	//   description: |
	//     Memory ceiling, mapped onto cgroup v2 `memory.max`.
	//
	//     Exceeding it OOM-kills the container.
	//   examples:
	//     - value: '"512MiB"'
	//   schema:
	//     type: string
	Memory string `yaml:"memory,omitempty"`
	//   description: |
	//     Swap ceiling, mapped onto cgroup v2 `memory.swap.max`.
	//
	//     `0` keeps the container's memory out of swap entirely, so its memory ceiling is enforced
	//     by the OOM killer rather than by swapping. Unset leaves the container free to use the
	//     node's swap.
	//   examples:
	//     - value: '"0"'
	//   schema:
	//     type: string
	Swap string `yaml:"swap,omitempty"`
}

// MemoryLimit implements config.ContainerResourcesConfig interface.
func (r *ContainerResources) MemoryLimit() optional.Optional[uint64] {
	if r.Limits == nil || r.Limits.Memory == "" {
		return optional.None[uint64]()
	}

	size, err := humanize.ParseBytes(r.Limits.Memory)
	if err != nil {
		return optional.None[uint64]()
	}

	return optional.Some(size)
}

// SwapLimit implements config.ContainerResourcesConfig interface.
func (r *ContainerResources) SwapLimit() optional.Optional[uint64] {
	if r.Limits == nil || r.Limits.Swap == "" {
		return optional.None[uint64]()
	}

	size, err := humanize.ParseBytes(r.Limits.Swap)
	if err != nil {
		return optional.None[uint64]()
	}

	return optional.Some(size)
}

// CPULimit implements config.ContainerResourcesConfig interface.
func (r *ContainerResources) CPULimit() optional.Optional[uint64] {
	if r.Limits == nil || r.Limits.CPU == "" {
		return optional.None[uint64]()
	}

	millicores, err := meta.ParseMillicores(r.Limits.CPU)
	if err != nil {
		return optional.None[uint64]()
	}

	return optional.Some(millicores)
}

// Validate checks the resource settings.
func (r *ContainerResources) Validate() error {
	var validationErrors error

	if r.Limits == nil {
		return validationErrors
	}

	if r.Limits.Memory != "" {
		if _, err := humanize.ParseBytes(r.Limits.Memory); err != nil {
			validationErrors = errors.Join(validationErrors,
				fmt.Errorf("limits.memory %q is not a valid size: %w", r.Limits.Memory, err))
		}
	}

	if r.Limits.Swap != "" {
		if _, err := humanize.ParseBytes(r.Limits.Swap); err != nil {
			validationErrors = errors.Join(validationErrors,
				fmt.Errorf("limits.swap %q is not a valid size: %w", r.Limits.Swap, err))
		}
	}

	if r.Limits.CPU != "" {
		if _, err := meta.ParseMillicores(r.Limits.CPU); err != nil {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("limits.cpu %w", err))
		}
	}

	return validationErrors
}

// ContainerDependsOn gates container startup on external conditions.
type ContainerDependsOn struct {
	//   description: |
	//     Host paths which must exist before the container starts.
	//
	//     Polled, so a path that never appears leaves the container waiting indefinitely.
	//   examples:
	//     - value: '[]string{"/var/mnt/web-content"}'
	PathsConfig []string `yaml:"paths,omitempty"`
	//   description: |
	//     Network readiness conditions which must be satisfied.
	//   values:
	//     - addresses
	//     - connectivity
	//     - hostname
	//     - etcfiles
	NetworkConfig []string `yaml:"network,omitempty"`
	// docgen:nodoc
	//
	// Deprecated: Use `network` instead.
	NetworksConfig []string `yaml:"networks,omitempty"`
	//   description: |
	//     Whether the clock must be synchronized before the container starts.
	TimeConfig *bool `yaml:"time,omitempty"`
	//   description: |
	//     Other containers, by document name, which must be running first, or have succeeded.
	//
	//     An entry is either the name of a container which must be running and healthy, or a name with
	//     a condition. Cycles are rejected when the machine configuration is applied.
	//   examples:
	//     - value: >
	//         []ContainerDependency{{DependencyName: "migrate", DependencyCondition: config.ContainerDependencyConditionSucceeded}, {DependencyName: "db"}}
	//   schema:
	//     type: array
	//     items:
	//       oneOf:
	//         - type: string
	//         - $ref: "#/$defs/container.ContainerDependency"
	ContainersConfig []ContainerDependency `yaml:"containers,omitempty"`
}

// ContainerDependency is a container which must be running, or have succeeded, first.
type ContainerDependency struct {
	//   description: |
	//     Name of the ContainerConfig document.
	DependencyName string `yaml:"name"`
	//   description: |
	//     `running` (the default) waits for the container to be running and healthy, `succeeded` for
	//     it to have exited with code 0 and not to be restarted, e.g. a container preparing what this one
	//     needs. It is checked again on every boot, as the containers run again.
	//   values:
	//     - running
	//     - succeeded
	//   schema:
	//     type: string
	DependencyCondition config.ContainerDependencyCondition `yaml:"condition,omitempty"`
}

var _ config.ContainerDependencyConfig = ContainerDependency{}

// Name implements config.ContainerDependencyConfig interface.
func (d ContainerDependency) Name() string { return d.DependencyName }

// Condition implements config.ContainerDependencyConfig interface.
func (d ContainerDependency) Condition() config.ContainerDependencyCondition {
	if d.DependencyCondition == "" {
		return config.ContainerDependencyConditionRunning
	}

	return d.DependencyCondition
}

// UnmarshalYAML accepts a plain name as well.
func (d *ContainerDependency) UnmarshalYAML(unmarshal func(any) error) error {
	var name string

	if err := unmarshal(&name); err == nil {
		*d = ContainerDependency{DependencyName: name}

		return nil
	}

	type containerDependency ContainerDependency

	return unmarshal((*containerDependency)(d))
}

// MarshalYAML writes a dependency without a condition as a plain name.
func (d ContainerDependency) MarshalYAML() (any, error) {
	if d.DependencyCondition == "" {
		return d.DependencyName, nil
	}

	type containerDependency ContainerDependency

	return containerDependency(d), nil
}

// Paths implements config.ContainerDependsOnConfig interface.
func (d *ContainerDependsOn) Paths() []string { return d.PathsConfig }

// Network implements config.ContainerDependsOnConfig interface.
//
// The deprecated `networks` key is used only when `network` is empty.
func (d *ContainerDependsOn) Network() []string {
	if len(d.NetworkConfig) > 0 {
		return d.NetworkConfig
	}

	return d.NetworksConfig
}

// Time implements config.ContainerDependsOnConfig interface.
func (d *ContainerDependsOn) Time() bool {
	return pointer.SafeDeref(d.TimeConfig)
}

// Containers implements config.ContainerDependsOnConfig interface.
func (d *ContainerDependsOn) Containers() []config.ContainerDependencyConfig {
	return xslices.Map(d.ContainersConfig, func(d ContainerDependency) config.ContainerDependencyConfig { return d })
}

// Validate checks the dependency settings.
//
// selfName is the name of the owning document, used to reject self-dependency.
func (d *ContainerDependsOn) Validate(selfName string) error {
	var validationErrors error

	for _, path := range d.PathsConfig {
		validationErrors = errors.Join(validationErrors, ValidateAbsPath("dependsOn.paths entry", path))
	}

	for _, condition := range d.Network() {
		if !slices.Contains(validNetworkConditions, condition) {
			validationErrors = errors.Join(validationErrors,
				fmt.Errorf("unsupported dependsOn.network condition %q", condition))
		}
	}

	seen := map[string]struct{}{}

	for _, dependency := range d.ContainersConfig {
		name := dependency.DependencyName

		if name == "" {
			validationErrors = errors.Join(validationErrors, errors.New("dependsOn.containers entry must not be empty"))

			continue
		}

		if name == selfName {
			validationErrors = errors.Join(validationErrors,
				fmt.Errorf("container %q cannot depend on itself", selfName))
		}

		if _, duplicate := seen[name]; duplicate {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("dependsOn.containers lists %q more than once", name))
		}

		seen[name] = struct{}{}

		switch dependency.Condition() {
		case config.ContainerDependencyConditionRunning, config.ContainerDependencyConditionSucceeded:
		default:
			validationErrors = errors.Join(validationErrors,
				fmt.Errorf("unsupported dependsOn.containers condition %q, expected running or succeeded", dependency.DependencyCondition))
		}
	}

	return validationErrors
}
