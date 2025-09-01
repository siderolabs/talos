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

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//go:embed testdata/oom.yaml
var expectedOOMDocument []byte

func TestOOMMarshalStability(t *testing.T) {
	cfg := runtime.NewOOMV1Alpha1()
	cfg.OOMSampleInterval = 100 * time.Millisecond
	cfg.OOMTriggerExpression = cel.MustExpression(cel.ParseBooleanExpression(
		constants.DefaultOOMTriggerExpression,
		celenv.OOMTrigger(),
	))
	cfg.OOMCgroupRankingExpression = cel.MustExpression(cel.ParseDoubleExpression(
		constants.DefaultOOMCgroupRankingExpression,
		celenv.OOMCgroupScoring(),
	))

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, string(expectedOOMDocument), string(marshaled))
}

func TestOOMValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *runtime.OOMV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  runtime.NewOOMV1Alpha1,
		},
		{
			name: "invalid expression",
			cfg: func() *runtime.OOMV1Alpha1 {
				cfg := runtime.NewOOMV1Alpha1()

				require.NoError(t, cfg.OOMCgroupRankingExpression.UnmarshalText([]byte(`disk.transport`)))

				return cfg
			},

			expectedError: "OOM cgroup scoring expression is invalid: ERROR: <input>:1:1: undeclared reference to 'disk' (in container '')\n | disk.transport\n | ^",
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
