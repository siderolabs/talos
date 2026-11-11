// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
)

func TestMemoryLimit(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		percent  uint64
		capacity uint64
		reserved uint64
		expected uint64
	}{
		{"unlimited", 0, 16 << 30, 1 << 30, 0},
		{"percentage of the capacity minus reserved", 40, 11 << 30, 1 << 30, 4 << 30},
		{"whole capacity", 100, 16 << 30, 0, 16 << 30},
		{"rounds down", 33, 1000, 0, 330},
		{"no overflow near the int64 limit", 100, math.MaxInt64, 0, math.MaxInt64},
		{"reserved above the capacity stays limited", 50, 1 << 30, 2 << 30, 1},
		{"rounding to zero stays limited", 1, 99, 0, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, config.MemoryLimit(test.percent, test.capacity, test.reserved))
		})
	}
}

func TestDefaultMemoryReserved(t *testing.T) {
	t.Parallel()

	assert.Equal(t, uint64(384<<20), config.DefaultMemoryReserved(machine.TypeWorker))
	assert.Equal(t, uint64(512<<20), config.DefaultMemoryReserved(machine.TypeControlPlane))
	assert.Equal(t, uint64(512<<20), config.DefaultMemoryReserved(machine.TypeInit))
}
