// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// TestSchedulerConfig_IntNormalization is a regression test for
// siderolabs/talos#13445: the YAML parser emits Go ints for integers, and
// runtime.DeepCopyJSON panics on those. schedulerConfig must not panic when
// the config map contains native Go int values.
func TestSchedulerConfig_IntNormalization(t *testing.T) {
	spec := &k8s.SchedulerConfigSpec{
		Config: map[string]any{
			"parallelism": int(16),
			"profiles": []any{
				map[string]any{
					"schedulerName": "default-scheduler",
					"pluginConfig": []any{
						map[string]any{
							"name": "PodTopologySpread",
							"args": map[string]any{
								"defaultingType": "List",
								"defaultConstraints": []any{
									map[string]any{
										"maxSkew":           int(1),
										"topologyKey":       "kubernetes.io/hostname",
										"whenUnsatisfiable": "ScheduleAnyway",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	fn := k8sctrl.SchedulerConfig(spec)

	require.NotPanics(t, func() {
		obj, err := fn()
		require.NoError(t, err)
		assert.NotNil(t, obj)
	})
}
