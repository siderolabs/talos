// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	containersctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/containers"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

// TestLogHealthTransitionLevel covers the level a health transition is logged at: a degradation is
// a warning, a recovery is not.
func TestLogHealthTransitionLevel(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		from  containers.ContainerHealth
		to    containers.ContainerHealth
		level zapcore.Level
	}{
		{name: "degraded", from: containers.ContainerHealthHealthy, to: containers.ContainerHealthDegraded, level: zapcore.WarnLevel},
		{name: "recovered", from: containers.ContainerHealthDegraded, to: containers.ContainerHealthHealthy, level: zapcore.InfoLevel},
		{name: "passed", from: containers.ContainerHealthPending, to: containers.ContainerHealthHealthy, level: zapcore.InfoLevel},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			core, logs := observer.New(zapcore.DebugLevel)

			before := containers.ContainerStatusSpec{State: containers.ContainerStateRunning, Health: test.from}
			after := containers.ContainerStatusSpec{State: containers.ContainerStateRunning, Health: test.to}

			containersctrl.LogContainerStatusTransition(zap.New(core), "web", before, after)

			entries := logs.All()
			require.Len(t, entries, 1)
			assert.Equal(t, "container health changed", entries[0].Message)
			assert.Equal(t, test.level, entries[0].Level)
		})
	}
}
