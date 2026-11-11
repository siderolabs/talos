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

func TestMemoryLimitResolve(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		limit    config.MemoryLimit
		capacity uint64
		reserved uint64
		expected uint64
	}{
		{"unlimited", config.MemoryLimit{}, 16 << 30, 1 << 30, 0},
		{"percentage of the capacity minus reserved", config.MemoryLimit{Percent: 40}, 11 << 30, 1 << 30, 4 << 30},
		{"whole capacity", config.MemoryLimit{Percent: 100}, 16 << 30, 0, 16 << 30},
		{"rounds down", config.MemoryLimit{Percent: 33}, 1000, 0, 330},
		{"no overflow near the int64 limit", config.MemoryLimit{Percent: 100}, math.MaxInt64, 0, math.MaxInt64},
		{"reserved above the capacity stays limited", config.MemoryLimit{Percent: 50}, 1 << 30, 2 << 30, 1},
		{"rounding to zero stays limited", config.MemoryLimit{Percent: 1}, 99, 0, 1},
		{"absolute size ignores the capacity and the reservation", config.MemoryLimit{Bytes: 16 << 30}, 8 << 30, 1 << 30, 16 << 30},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, test.limit.Resolve(test.capacity, test.reserved))
		})
	}
}

func TestDefaultMemoryReserved(t *testing.T) {
	t.Parallel()

	assert.Equal(t, uint64(384<<20), config.DefaultMemoryReserved(machine.TypeWorker).Value())
	assert.Equal(t, uint64(512<<20), config.DefaultMemoryReserved(machine.TypeControlPlane).Value())
	assert.Equal(t, uint64(512<<20), config.DefaultMemoryReserved(machine.TypeInit).Value())
}
