// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package services contains definitions for non-system services.
package services

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/hashicorp/go-multierror"
	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// Spec is represents non-system service definition.
type Spec struct {
	// Name of the service to run, will be prefixed with `ext-` when registered as Talos service.
	//
	// Valid: [-_a-z0-9]+
	Name string `yaml:"name"`
	// Container to run.
	//
	// Container rootfs should be extracted to the /usr/local/lib/containers/<name>.
	Container Container `yaml:"container"`
	// Service dependencies.
	Depends []Dependency `yaml:"depends"`
	// Restart configuration.
	Restart RestartKind `yaml:"restart"`
	// LogToConsole enables sending service logs to the console.
	LogToConsole bool `yaml:"logToConsole"`
}

// Container specifies service container to run.
type Container struct {
	// Entrypoint for the service, relative to the container rootfs.
	Entrypoint string `yaml:"entrypoint"`
	// Environment variables for the service.
	Environment []string `yaml:"environment"`
	// EnvironmentFile to load environment vars before running the service.
	EnvironmentFile string `yaml:"environmentFile"`
	// Args to pass to the entrypoint.
	Args []string `yaml:"args"`
	// Volume mounts.
	Mounts []specs.Mount `yaml:"mounts"`
	// Security options.
	Security Security `yaml:"security"`
}

// Security options for containers.
type Security struct {
	// WriteableSysfs makes the '/sys' path writeable in the container namespace if set to true.
	WriteableSysfs bool `yaml:"writeableSysfs"`
	// MaskedPaths is a list of paths in the container namespace that should not be readable.
	MaskedPaths []string `yaml:"maskedPaths"`
	// ReadonlyPaths is a list of paths in the container namespace that should be read-only.
	ReadonlyPaths []string `yaml:"readonlyPaths"`
	// WriteableRootfs
	WriteableRootfs bool `yaml:"writeableRootfs"`
	// RootfsPropagation is the propagation mode for the rootfs mount.
	RootfsPropagation string `yaml:"rootfsPropagation,omitempty"`
}

// Dependency describes a service Dependency.
//
// Only a single dependency out of the list might be specified.
type Dependency struct {
	// Depends on a service being running and healthy (if health checks are available).
	Service string `yaml:"service,omitempty"`
	// Depends on file/directory existence.
	Path string `yaml:"path,omitempty"`
	// Network readiness checks.
	//
	// Valid options are nethelpers.Status string values.
	Network []nethelpers.Status `yaml:"network,omitempty"`
	// Time sync check.
	Time bool `yaml:"time,omitempty"`
	// Depends on configuration files to be present.
	Configuration bool `yaml:"configuration,omitempty"`
}

var nameRe = regexp.MustCompile(`^[-_a-z0-9]{1,}$`)

// Validate the service spec.
func (spec *Spec) Validate() error {
	var multiErr *multierror.Error

	if !nameRe.MatchString(spec.Name) {
		multiErr = multierror.Append(multiErr, fmt.Errorf("name %q is invalid", spec.Name))
	}

	if !spec.Restart.IsARestartKind() {
		multiErr = multierror.Append(multiErr, fmt.Errorf("restart kind is invalid: %s", spec.Restart))
	}

	multiErr = multierror.Append(multiErr, spec.Container.Validate())

	for _, dep := range spec.Depends {
		multiErr = multierror.Append(multiErr, dep.Validate())
	}

	return multiErr.ErrorOrNil()
}

// Validate the container spec.
func (ctr *Container) Validate() error {
	var multiErr *multierror.Error

	if ctr.Entrypoint == "" {
		multiErr = multierror.Append(multiErr, errors.New("container endpoint can't be empty"))
	}

	return multiErr.ErrorOrNil()
}

// Validate the dependency spec.
//
//nolint:gocyclo
func (dep *Dependency) Validate() error {
	var multiErr *multierror.Error

	nonZeroDeps := 0

	if dep.Service != "" {
		nonZeroDeps++
	}

	if dep.Path != "" {
		nonZeroDeps++

		if !filepath.IsAbs(dep.Path) {
			multiErr = multierror.Append(multiErr, fmt.Errorf("path is not absolute: %q", dep.Path))
		}
	}

	if len(dep.Network) > 0 {
		nonZeroDeps++

		for _, st := range dep.Network {
			if !st.IsAStatus() {
				multiErr = multierror.Append(multiErr, fmt.Errorf("invalid network dependency: %s", st))
			}
		}
	}

	if dep.Time {
		nonZeroDeps++
	}

	if dep.Configuration {
		nonZeroDeps++
	}

	if nonZeroDeps == 0 {
		multiErr = multierror.Append(multiErr, errors.New("no dependency specified"))
	}

	if nonZeroDeps > 1 {
		multiErr = multierror.Append(multiErr, errors.New("more than a single dependency is set"))
	}

	return multiErr.ErrorOrNil()
}
