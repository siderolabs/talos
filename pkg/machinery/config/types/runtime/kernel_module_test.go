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

//go:embed testdata/kernel_module.yaml
var expectedKernelModuleDocument []byte

func TestKernelModuleMarshalStability(t *testing.T) {
	cfg := runtime.NewKernelModuleConfigV1Alpha1("btrfs")
	cfg.ModuleParameters = []string{"param1"}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKernelModuleDocument, marshaled)
}

func TestKernelModuleValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *runtime.KernelModuleConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  func() *runtime.KernelModuleConfigV1Alpha1 { return runtime.NewKernelModuleConfigV1Alpha1("") },

			expectedError: "name is required",
		},
		{
			name: "valid",
			cfg: func() *runtime.KernelModuleConfigV1Alpha1 {
				cfg := runtime.NewKernelModuleConfigV1Alpha1("btrfs")

				cfg.ModuleParameters = []string{"param1"}

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
