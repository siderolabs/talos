// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/block"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
)

func TestCELMarshal(t *testing.T) {
	t.Parallel()

	env := celenv.DiskLocator()

	type yamlTest struct {
		Expr cel.Expression `yaml:"expr,omitempty"`
	}

	for _, test := range []struct {
		name string

		expression cel.Expression

		expectedYAML string
	}{
		{
			name: "empty",

			expectedYAML: "{}\n",
		},
		{
			name: "system disk",

			expression: cel.MustExpression(cel.ParseBooleanExpression("system_disk", env)),

			expectedYAML: "expr: system_disk\n",
		},
		{
			name: "disk size and rotational",

			expression: cel.MustExpression(cel.ParseBooleanExpression("disk.size > 1000u && !disk.rotational", env)),

			expectedYAML: "expr: disk.size > 1000u && !disk.rotational\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			yamlTest := yamlTest{
				Expr: test.expression,
			}

			yaml, err := yaml.Marshal(yamlTest)
			require.NoError(t, err)

			require.Equal(t, test.expectedYAML, string(yaml))
		})
	}
}

func TestCELEvalFromYAML(t *testing.T) {
	t.Parallel()

	env := celenv.DiskLocator()

	type yamlRaw struct {
		Expr string `yaml:"expr,omitempty"`
	}

	type yamlTest struct {
		Expr cel.Expression `yaml:"expr,omitempty"`
	}

	for _, test := range []struct {
		name string

		expression string

		expected bool
	}{
		{
			name: "consts",

			expression: "1u * GiB < 2u * GiB",

			expected: true,
		},
		{
			name: "vars",

			expression: "!system_disk",

			expected: false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			yamlRaw := yamlRaw{
				Expr: test.expression,
			}

			marshaled, err := yaml.Marshal(yamlRaw)
			require.NoError(t, err)

			var yamlTest yamlTest

			err = yaml.Unmarshal(marshaled, &yamlTest)
			require.NoError(t, err)

			val, err := yamlTest.Expr.EvalBool(env, map[string]any{
				"system_disk": true,
				"disk": block.DiskSpec{
					Size: 1024,
				},
			})
			require.NoError(t, err)

			assert.Equal(t, test.expected, val)
		})
	}
}
