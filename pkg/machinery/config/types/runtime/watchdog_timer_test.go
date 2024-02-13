// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	_ "embed"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
)

//go:embed testdata/watchdogtimer.yaml
var expectedWatchdogTimerDocument []byte

func TestWatchdogTimerMarshalStability(t *testing.T) {
	cfg := runtime.NewWatchdogTimerV1Alpha1()
	cfg.WatchdogDevice = "/dev/watchdog0"
	cfg.WatchdogTimeout = 3 * time.Minute

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedWatchdogTimerDocument, marshaled)
}

func TestWatchdogTimerValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *runtime.WatchdogTimerV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  runtime.NewWatchdogTimerV1Alpha1,

			expectedError: "watchdog device: empty value",
		},
		{
			name: "negative timeout",
			cfg: func() *runtime.WatchdogTimerV1Alpha1 {
				cfg := runtime.NewWatchdogTimerV1Alpha1()
				cfg.WatchdogDevice = "/dev/watchdog1"
				cfg.WatchdogTimeout = -1 * time.Minute

				return cfg
			},

			expectedError: "watchdog timeout: minimum value is 10s",
		},
		{
			name: "small timeout",
			cfg: func() *runtime.WatchdogTimerV1Alpha1 {
				cfg := runtime.NewWatchdogTimerV1Alpha1()
				cfg.WatchdogDevice = "/dev/watchdog1"
				cfg.WatchdogTimeout = time.Second

				return cfg
			},

			expectedError: "watchdog timeout: minimum value is 10s",
		},
		{
			name: "valid",
			cfg: func() *runtime.WatchdogTimerV1Alpha1 {
				cfg := runtime.NewWatchdogTimerV1Alpha1()
				cfg.WatchdogDevice = "/dev/watchdog2"

				return cfg
			},
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
