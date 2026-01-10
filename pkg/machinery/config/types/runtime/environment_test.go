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

//go:embed testdata/environment.yaml
var expectedEnvironmentDocument []byte

func TestEnvironmentMarshalStability(t *testing.T) {
	cfg := runtime.NewEnvironmentV1Alpha1()
	cfg.EnvironmentVariables = map[string]string{
		"HTTP_PROXY": "http://proxy.example.com:8080",
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedEnvironmentDocument, marshaled)
}

func TestEnvironmentValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *runtime.EnvironmentV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  runtime.NewEnvironmentV1Alpha1,
		},
		{
			name: "valid",
			cfg: func() *runtime.EnvironmentV1Alpha1 {
				cfg := runtime.NewEnvironmentV1Alpha1()

				cfg.EnvironmentVariables = map[string]string{
					"HTTP_PROXY":  "http://proxy.example.com:8080",
					"HTTPS_PROXY": "http://proxy.example.com:8080",
					"NO_PROXY":    "localhost",
				}

				return cfg
			},
		},
		{
			name: "invalid - starts with invalid character",
			cfg: func() *runtime.EnvironmentV1Alpha1 {
				cfg := runtime.NewEnvironmentV1Alpha1()

				cfg.EnvironmentVariables = map[string]string{
					"1INVALID": "value",
				}

				return cfg
			},

			expectedError: "invalid environment variable name: \"1INVALID\"",
		},
		{
			name: "invalid - contains invalid character",
			cfg: func() *runtime.EnvironmentV1Alpha1 {
				cfg := runtime.NewEnvironmentV1Alpha1()

				cfg.EnvironmentVariables = map[string]string{
					"INVALID-CHAR": "value",
				}

				return cfg
			},

			expectedError: "invalid environment variable name: \"INVALID-CHAR\"",
		},
		{
			name: "invalid - empty",
			cfg: func() *runtime.EnvironmentV1Alpha1 {
				cfg := runtime.NewEnvironmentV1Alpha1()

				cfg.EnvironmentVariables = map[string]string{
					"": "value",
				}

				return cfg
			},

			expectedError: "invalid environment variable name: \"\"",
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
