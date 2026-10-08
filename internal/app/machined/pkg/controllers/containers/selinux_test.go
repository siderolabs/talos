// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	containersctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/containers"
	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	containercfg "github.com/siderolabs/talos/pkg/machinery/config/types/container"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestSELinuxPair(t *testing.T) {
	t.Parallel()

	seen := map[[2]uint64]struct{}{}

	for index := range uint64(containersctrl.SELinuxPairCount) {
		low, high := containersctrl.SELinuxPair(index)

		require.Less(t, low, high)
		require.Less(t, high, uint64(constants.SELinuxReservedCategoryStart-constants.SELinuxCRICategoryRange))

		seen[[2]uint64{low, high}] = struct{}{}
	}

	assert.Len(t, seen, containersctrl.SELinuxPairCount)
}

func TestAllocateSELinuxLevels(t *testing.T) {
	t.Parallel()

	// enough containers for the pairs of some to collide
	containerConfigs := make([]configcfg.ContainerConfig, 0, 1000)

	for i := range 1000 {
		containerConfigs = append(containerConfigs, newDoc(fmt.Sprintf("container-%d", i), "nginx"))
	}

	privileged := newDoc("privileged", "nginx")
	privileged.SecurityConfig = &containercfg.ContainerSecurity{SecurityProfile: "privileged"}
	containerConfigs = append(containerConfigs, privileged)

	levels := containersctrl.AllocateSELinuxLevels(containerConfigs)
	require.Len(t, levels, 1000)
	assert.NotContains(t, levels, "privileged")

	seen := map[string]struct{}{}

	for name, level := range levels {
		var low, high int

		_, err := fmt.Sscanf(level, "s0:c%d,c%d", &low, &high)
		require.NoError(t, err, name)

		assert.GreaterOrEqual(t, low, constants.SELinuxCRICategoryRange, name)
		assert.Less(t, low, high, name)
		assert.Less(t, high, constants.SELinuxReservedCategoryStart, name)

		assert.NotContains(t, seen, level, name)
		seen[level] = struct{}{}
	}

	// the pair of a container does not depend on the order of the documents
	reversed := slices.Clone(containerConfigs)
	slices.Reverse(reversed)

	assert.Equal(t, levels, containersctrl.AllocateSELinuxLevels(reversed))

	// nor on the other containers, unless they collide
	alone := containersctrl.AllocateSELinuxLevels(containerConfigs[:1])
	assert.Equal(t, levels["container-0"], alone["container-0"])
}
