// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubelet

import (
	"errors"
	"fmt"
	"slices"
)

// cpuReservationArgs are the kubelet flags which set the reserved CPUs.
var cpuReservationArgs = []string{"reserved-cpus"}

// CPUManagerPolicyStatic is the CPU manager policy Talos sets while kubepods is bounded.
const CPUManagerPolicyStatic = "static"

// ValidateCPUReservationConfiguration rejects kubelet settings which conflict with the static CPU
// manager policy and reserved CPUs Talos derives from a CPU partition policy bounding kubepods.
func ValidateCPUReservationConfiguration(extraConfig map[string]any, extraArgs map[string][]string) error {
	var errs error

	if _, ok := extraConfig["reservedSystemCPUs"]; ok {
		errs = errors.Join(errs, errors.New(`"reservedSystemCPUs" is derived from the CPU partition kubepods root and can't be set`))
	}

	if policy, ok := extraConfig["cpuManagerPolicy"]; ok && policy != CPUManagerPolicyStatic {
		errs = errors.Join(errs, fmt.Errorf(`"cpuManagerPolicy" %v conflicts with the CPU partition kubepods root, which requires %q`, policy, CPUManagerPolicyStatic))
	}

	for flag, values := range extraArgs {
		switch normalizeFlag(flag) {
		case cpuReservationArgs[0]:
			errs = errors.Join(errs, errors.New(`"reserved-cpus" is derived from the CPU partition kubepods root and can't be set`))
		case "cpu-manager-policy":
			if slices.ContainsFunc(values, func(v string) bool { return v != CPUManagerPolicyStatic }) {
				errs = errors.Join(errs, fmt.Errorf(`"cpu-manager-policy" conflicts with the CPU partition kubepods root, which requires %q`, CPUManagerPolicyStatic))
			}
		}
	}

	return errs
}
