// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
)

//go:embed testdata/sysfs.yaml
var expectedSysfsDocument []byte

func TestSysfsMarshalStability(t *testing.T) {
	cfg := runtime.NewSysfsConfigV1Alpha1()
	cfg.Params = map[string]string{
		"devices.system.cpu.cpu0.cpufreq.scaling_governor": "performance",
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedSysfsDocument, marshaled)
}

func TestSysfsValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *runtime.SysfsConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  runtime.NewSysfsConfigV1Alpha1,
		},
		{
			name: "valid",
			cfg: func() *runtime.SysfsConfigV1Alpha1 {
				cfg := runtime.NewSysfsConfigV1Alpha1()

				cfg.Params = map[string]string{
					"devices.system.cpu.cpu0.cpufreq.scaling_governor": "performance",
				}

				return cfg
			},
		},
		{
			name: "invalid - empty key",
			cfg: func() *runtime.SysfsConfigV1Alpha1 {
				cfg := runtime.NewSysfsConfigV1Alpha1()

				cfg.Params = map[string]string{
					"": "value",
				}

				return cfg
			},

			expectedError: "sysfs key cannot be empty",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})

			assert.Equal(t, test.expectedWarnings, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
