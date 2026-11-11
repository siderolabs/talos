// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroups_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/cgroups"
)

func TestParseIOMax(t *testing.T) {
	t.Parallel()

	var node cgroups.Node

	// the example from the kernel's cgroup-v2.rst
	require.NoError(t, node.Parse("io.max", strings.NewReader("8:16 rbps=2097152 wbps=max riops=max wiops=120\n")))

	limits := node.IOMax["8:16"]
	assert.Equal(t, "2.0 MiB", limits["rbps"].HumanizeIBytes())
	assert.Equal(t, "max", limits["wbps"].HumanizeIBytes())
	assert.Equal(t, "max", limits["riops"].String())
	assert.Equal(t, "120", limits["wiops"].String())
}

func TestResolveDevices(t *testing.T) {
	t.Parallel()

	var node cgroups.Node

	require.NoError(t, node.Parse("io.weight", strings.NewReader("default 100\n8:16 200\n")))
	require.NoError(t, node.Parse("io.max", strings.NewReader("8:16 rbps=max wbps=max riops=max wiops=120\n")))

	node.ResolveDevices(map[string]string{"8:16": "sdb"})

	assert.Equal(t, "100", node.IOWeight["default"].String())
	assert.Equal(t, "200", node.IOWeight["sdb"].String())
	assert.NotContains(t, node.IOWeight, "8:16")
	assert.Equal(t, "120", node.IOMax["sdb"]["wiops"].String())
	assert.NotContains(t, node.IOMax, "8:16")
}
