// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cel_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

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
