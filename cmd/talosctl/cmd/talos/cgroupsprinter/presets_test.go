// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroupsprinter_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/talos/cgroupsprinter"
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
