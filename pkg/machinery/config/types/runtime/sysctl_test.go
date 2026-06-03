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

//go:embed testdata/sysctl.yaml
var expectedSysctlDocument []byte

func TestSysctlMarshalStability(t *testing.T) {
	cfg := runtime.NewSysctlConfigV1Alpha1()
	cfg.Params = map[string]string{
		"net.ipv4.ip_forward": "1",
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedSysctlDocument, marshaled)
}

func TestSysctlValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *runtime.SysctlConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  runtime.NewSysctlConfigV1Alpha1,
		},
		{
			name: "valid",
			cfg: func() *runtime.SysctlConfigV1Alpha1 {
				cfg := runtime.NewSysctlConfigV1Alpha1()

				cfg.Params = map[string]string{
					"net.ipv4.ip_forward":                 "1",
					"net/ipv6/conf/eth0.100/disable_ipv6": "1",
				}

				return cfg
			},
		},
		{
			name: "invalid - empty key",
			cfg: func() *runtime.SysctlConfigV1Alpha1 {
				cfg := runtime.NewSysctlConfigV1Alpha1()

				cfg.Params = map[string]string{
					"": "value",
				}

				return cfg
			},

			expectedError: "sysctl key cannot be empty",
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
