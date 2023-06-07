// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

/* Commented out to workaround an issue with go1.20.5.
import (
	_ "embed"
	"net/url"
	"strings"
	"testing"

	validatejsonschema "github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//go:embed schemas/v1alpha1_config.schema.json
var schemaData string

func TestSchemaValidation(t *testing.T) {
	t.Parallel()

	schema, err := validatejsonschema.CompileString("test-id", schemaData)
	require.NoError(t, err)

	for _, test := range []struct {
		name                  string
		config                map[string]any
		expectedErrorContains string
	}{
		{
			name:   "valid",
			config: newConfig(t, nil, nil),
		},
		{
			name: "invalid-version",
			config: newConfig(t, func(config *v1alpha1.Config) {
				config.ConfigVersion = "v1alpha2"
			}, nil),
			expectedErrorContains: `value must be "v1alpha1"`,
		},
		{
			name: "invalid-control-plane-endpoint",
			config: newConfig(t, func(config *v1alpha1.Config) {
				endpointURL, urlErr := url.Parse("ftp://127.0.0.1:6443")
				require.NoError(t, urlErr)

				config.ClusterConfig.ControlPlane.Endpoint = &v1alpha1.Endpoint{
					URL: endpointURL,
				}
			}, nil),
			expectedErrorContains: `does not match pattern '^https://'`,
		},
		{
			name: "invalid-duration",
			config: newConfig(t, nil, func(rawConfig map[string]any) {
				setNestedField(t, rawConfig, "100y", "machine", "time", "bootTimeout")
			}),
			expectedErrorContains: `does not match pattern`,
		},
		{
			name: "invalid-persist-type",
			config: newConfig(t, nil, func(rawConfig map[string]any) {
				setNestedField(t, rawConfig, "something", "persist")
			}),
			expectedErrorContains: `expected boolean, but got string`,
		},
		{
			name: "invalid-machine-type",
			config: newConfig(t, func(config *v1alpha1.Config) {
				config.MachineConfig.MachineType = "invalidtype"
			}, nil),
			expectedErrorContains: `value must be one of "controlplane", "worker"`,
		},
	} {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			testErr := schema.Validate(test.config)

			if test.expectedErrorContains != "" {
				assert.ErrorContains(t, testErr, test.expectedErrorContains)
			} else {
				assert.NoError(t, testErr)
			}
		})
	}
}

func newConfig(t *testing.T, modifications func(config *v1alpha1.Config), rawModifications func(rawConfig map[string]any)) map[string]any {
	input, err := generate.NewInput("test", "https://doesntmatter:6443", constants.DefaultKubernetesVersion)
	require.NoError(t, err)

	config, err := input.Config(machine.TypeControlPlane)
	require.NoError(t, err)

	if modifications != nil {
		modifications(config.RawV1Alpha1())
	}

	configBytes, err := config.Bytes()
	require.NoError(t, err)

	var data map[string]any

	require.NoError(t, yaml.Unmarshal(configBytes, &data))

	if rawModifications != nil {
		rawModifications(data)
	}

	return data
}

func setNestedField(t *testing.T, obj map[string]any, value any, fields ...string) {
	m := obj

	for i, field := range fields[:len(fields)-1] {
		if val, ok := m[field]; ok {
			valMap, valMapOk := val.(map[string]any)

			require.Truef(t, valMapOk, "value cannot be set because %v is not a map[string]any", jsonPath(fields[:i+1]))

			m = valMap
		} else {
			newVal := make(map[string]any)
			m[field] = newVal
			m = newVal
		}
	}

	m[fields[len(fields)-1]] = value
}

func jsonPath(fields []string) string {
	return "." + strings.Join(fields, ".")
}
*/
