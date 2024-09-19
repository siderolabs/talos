// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroup_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/pkg/cgroup"
)

func TestAvailableMillicores(t *testing.T) {
	t.Logf("Available CPU milli-cores: %d", cgroup.AvailableMilliCores())

	assert.GreaterOrEqual(t, cgroup.AvailableMilliCores(), cgroup.MilliCores(1000))
}

func TestMillicoresToShares(t *testing.T) {
	assert.Equal(t, cgroup.CPUShare(102), cgroup.MilliCoresToShares(100))
	assert.Equal(t, cgroup.CPUShare(1024), cgroup.MilliCoresToShares(1000))
	assert.Equal(t, cgroup.CPUShare(2560), cgroup.MilliCoresToShares(2500))
}

func TestSharesToCPUWeight(t *testing.T) {
	assert.Equal(t, uint64(4), cgroup.SharesToCPUWeight(102))
	assert.Equal(t, uint64(79), cgroup.SharesToCPUWeight(2048))
	assert.Equal(t, uint64(313), cgroup.SharesToCPUWeight(8192))
}

func TestMillicoresToCPUWeight(t *testing.T) {
	// depends on number of CPUs available, but for < 1000 millicores it should be same result
	assert.Equal(t, uint64(4), cgroup.MillicoresToCPUWeight(100))
	assert.Equal(t, uint64(20), cgroup.MillicoresToCPUWeight(500))
	assert.Equal(t, uint64(39), cgroup.MillicoresToCPUWeight(1000))
}
