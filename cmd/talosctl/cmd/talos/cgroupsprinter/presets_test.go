// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroupsprinter_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/talos/cgroupsprinter"
	"github.com/siderolabs/talos/internal/pkg/cgroups"
)

func TestGetPresetNames(t *testing.T) {
	assert.Equal(t, []string{"cpu", "cpuset", "io", "memory", "process", "psi", "swap"}, cgroupsprinter.GetPresetNames())
}

func TestGetPreset(t *testing.T) {
	for _, name := range cgroupsprinter.GetPresetNames() {
		t.Run(name, func(t *testing.T) {
			assert.NotEmpty(t, cgroupsprinter.GetPreset(name))
		})
	}
}

func TestIOWeightColumn(t *testing.T) {
	t.Parallel()

	schema := cgroupsprinter.GetPreset("io")
	require.NoError(t, schema.Compile())
	require.Equal(t, "IOWeight", schema.Columns[0].Name)

	var weighted cgroups.Node

	require.NoError(t, weighted.Parse("io.weight", strings.NewReader("default 100\n8:16 200\n")))
	weighted.ResolveDevices(map[string]string{"8:16": "sdb"})

	for _, test := range []struct {
		name string
		node *cgroups.Node
		want string
	}{
		{name: "per-device override", node: &weighted, want: "   100 sdb: 200"},
		{name: "root without io.weight", node: &cgroups.Node{}, want: " unset"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var out strings.Builder

			require.NoError(t, schema.Columns[0].Render(&out, struct{ *cgroups.Node }{test.node}))
			assert.Equal(t, test.want, out.String())
		})
	}
}
