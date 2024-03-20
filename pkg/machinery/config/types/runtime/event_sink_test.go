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

//go:embed testdata/eventsink.yaml
var expectedEventSinkDocument []byte

func TestEventSinkMarshalStability(t *testing.T) {
	cfg := runtime.NewEventSinkV1Alpha1()
	cfg.Endpoint = "10.0.0.1:3333"

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedEventSinkDocument, marshaled)
}

func TestEventSinkValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *runtime.EventSinkV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  runtime.NewEventSinkV1Alpha1,

			expectedError: "event sink endpoint: missing port in address",
		},
		{
			name: "just IP",
			cfg: func() *runtime.EventSinkV1Alpha1 {
				cfg := runtime.NewEventSinkV1Alpha1()
				cfg.Endpoint = "10.0.0.1"

				return cfg
			},

			expectedError: "event sink endpoint: address 10.0.0.1: missing port in address",
		},
		{
			name: "valid",
			cfg: func() *runtime.EventSinkV1Alpha1 {
				cfg := runtime.NewEventSinkV1Alpha1()
				cfg.Endpoint = "[ff:80::01]:334"

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

type validationMode struct{}

func (validationMode) String() string {
	return ""
}

func (validationMode) RequiresInstall() bool {
	return false
}
