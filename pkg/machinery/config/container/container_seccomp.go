// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/hashicorp/go-multierror"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// validateContainerSeccompProfiles checks the seccomp profiles applied to containers.
//
// A localhost seccomp profile names a SeccompProfileConfig document (or a deprecated
// `.machine.seccompProfiles` entry), and is resolved against the rest of the configuration,
// so an undeclared profile would keep the container pending.
func validateContainerSeccompProfiles(container *Container) error {
	configs := container.ContainerConfigs()

	if len(configs) == 0 {
		return nil
	}

	declared := map[string]struct{}{}

	for _, profile := range container.SeccompProfiles() {
		declared[profile.Name()] = struct{}{}
	}

	// Sorted by name, so a given configuration always reports errors in the same order.
	sorted := slices.SortedFunc(slices.Values(configs), func(a, b config.ContainerConfig) int {
		return cmp.Compare(a.Name(), b.Name())
	})

	var errs *multierror.Error

	for _, containerConfig := range sorted {
		security := containerConfig.Security()

		if security.SeccompProfile() != config.ContainerSeccompProfileLocalhost || security.SeccompLocalhostProfile() == "" {
			// Not a localhost profile, or a missing name the document's own validation reports.
			continue
		}

		if _, ok := declared[security.SeccompLocalhostProfile()]; !ok {
			errs = multierror.Append(errs, fmt.Errorf(
				"container %q: security.seccompProfile: no SeccompProfileConfig declares seccomp profile %q",
				containerConfig.Name(), security.SeccompLocalhostProfile(),
			))
		}
	}

	return errs.ErrorOrNil()
}
