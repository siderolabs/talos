// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"fmt"
	"hash/fnv"
	"slices"

	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

const (
	selinuxCategoryStart = constants.SELinuxCRICategoryRange
	selinuxCategoryCount = constants.SELinuxReservedCategoryStart - constants.SELinuxCRICategoryRange
	selinuxPairCount     = selinuxCategoryCount * (selinuxCategoryCount - 1) / 2
)

// allocateSELinuxLevels gives each container a pair of MCS categories from the share of the Talos containers.
//
// The files a container writes carry its categories, so the pair has to survive restarts and reboots without
// being stored anywhere: it is derived from the name of the container. Two containers hashing to the same pair
// are told apart by probing for the next free one, in the order of their names, so a pair only moves when a
// container colliding with it comes or goes.
//
// Privileged containers get no categories, like privileged pods: they have the run of the host anyway.
func allocateSELinuxLevels(containerConfigs []configcfg.ContainerConfig) map[string]string {
	names := make([]string, 0, len(containerConfigs))

	for _, containerConfig := range containerConfigs {
		if containerConfig.Security().Profile() == configcfg.ContainerSecurityProfilePrivileged {
			continue
		}

		names = append(names, containerConfig.Name())
	}

	slices.Sort(names)

	levels := make(map[string]string, len(names))
	taken := make(map[uint64]struct{}, len(names))

	for _, name := range names {
		hash := fnv.New64a()
		hash.Write([]byte(name)) //nolint:errcheck

		pair := hash.Sum64() % selinuxPairCount

		for {
			if _, collides := taken[pair]; !collides {
				break
			}

			pair = (pair + 1) % selinuxPairCount
		}

		taken[pair] = struct{}{}

		low, high := selinuxPair(pair)
		levels[name] = fmt.Sprintf("s0:c%d,c%d", selinuxCategoryStart+low, selinuxCategoryStart+high)
	}

	return levels
}

// selinuxPair maps an index in [0, selinuxPairCount) to the pair of distinct categories it stands for, in the
// order (0, 1), (0, 2), ..., (1, 2), ...
func selinuxPair(index uint64) (low, high uint64) {
	for low = 0; index >= selinuxCategoryCount-1-low; low++ {
		index -= selinuxCategoryCount - 1 - low
	}

	return low, low + 1 + index
}
