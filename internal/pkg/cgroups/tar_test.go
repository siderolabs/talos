// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroups_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/cgroups"
)

func TestTreeFromTarGz(t *testing.T) {
	t.Parallel()

	tarFile, err := os.Open("testdata/cgroup.tar.gz")
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, tarFile.Close())
	})

	tree, err := cgroups.TreeFromTarGz(tarFile)
	require.NoError(t, err)

	assert.Equal(t, []string{"init", "kubepods", "podruntime", "system", "taloscontainers", "virtualmachines.partition"}, tree.Root.SortedChildren())
	assert.Equal(t, "108802048", tree.Find("init").MemoryCurrent.String())
	assert.Equal(t, "100", tree.Find("system").IOWeight["default"].String())
	assert.Equal(t, "50000", tree.Find("podruntime/kubelet").IOLatency["251:0"]["target"].String())
}
