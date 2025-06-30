// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config_test

import (
	_ "embed"
	"net/netip"
	"net/url"
	"strings"
	"testing"

	validatejsonschema "github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

//go:embed schemas/config.schema.json
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
			name:   "v1alpha1_valid",
			config: newV1Alpha1Config(t, nil, nil),
		},
		{
			name: "v1alpha1_invalid-version",
			config: newV1Alpha1Config(t, func(config *v1alpha1.Config) {
				config.ConfigVersion = "v1alpha2"
			}, nil),
			expectedErrorContains: `value must be "v1alpha1"`,
		},
		{
			name: "v1alpha1_invalid-control-plane-endpoint",
			config: newV1Alpha1Config(t, func(config *v1alpha1.Config) {
				endpointURL, urlErr := url.Parse("ftp://127.0.0.1:6443")
				require.NoError(t, urlErr)

				config.ClusterConfig.ControlPlane.Endpoint = &v1alpha1.Endpoint{
					URL: endpointURL,
				}
			}, nil),
			expectedErrorContains: `does not match pattern '^https://'`,
		},
		{
			name: "v1alpha1_invalid-duration",
			config: newV1Alpha1Config(t, nil, func(rawConfig map[string]any) {
				setNestedField(t, rawConfig, "100y", "machine", "time", "bootTimeout")
			}),
			expectedErrorContains: `does not match pattern`,
		},
		{
			name: "v1alpha1_invalid-machine-type",
			config: newV1Alpha1Config(t, func(config *v1alpha1.Config) {
				config.MachineConfig.MachineType = "invalidtype"
			}, nil),
			expectedErrorContains: `value must be one of "controlplane", "worker"`,
		},
		{
			name:   "network/RuleConfigV1Alpha1_valid",
			config: newRuleConfigV1Alpha1(t, nil, nil),
		},
		{
			name: "network/RuleConfigV1Alpha1_invalid-cidr-prefix",
			config: newRuleConfigV1Alpha1(t, nil, func(rawConfig map[string]any) {
				rawConfig["ingress"] = []any{
					map[string]any{
						"subnet": "10.42.0.0/16",
						"except": "10.42.43.0/24",
					},
					map[string]any{
						"subnet": "192.168.178.0/24",
						"except": "invalid-except/12343",
					},
				}
			}),
			expectedErrorContains: "'/ingress/1/except' does not validate with",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			testErr := schema.Validate(test.config)

			if test.expectedErrorContains != "" {
				errors := gatherValidationErrors(t, testErr)
				errorsStr := strings.Join(errors, "\n")

				assert.Contains(t, errorsStr, test.expectedErrorContains)
			} else {
				assert.NoError(t, testErr)
			}
		})
	}
}

func gatherValidationErrors(t *testing.T, err error) []string {
	var validationErr *validatejsonschema.ValidationError

	require.ErrorAs(t, err, &validationErr)

	messages := make([]string, 0, len(validationErr.Causes)+1)
	if len(validationErr.Causes) == 0 {
		messages = append(messages, validationErr.Error())
	}

	for _, cause := range validationErr.Causes {
		messages = append(messages, gatherValidationErrors(t, cause)...)
	}

	return messages
}

func newV1Alpha1Config(t *testing.T, modifications func(config *v1alpha1.Config), rawModifications func(rawConfig map[string]any)) map[string]any {
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

	// deprecated field
	delete(data, "persist")

	return data
}

func newRuleConfigV1Alpha1(t *testing.T, modifications func(config *network.RuleConfigV1Alpha1), rawModifications func(rawConfig map[string]any)) map[string]any {
	config := network.NewRuleConfigV1Alpha1()

	config.MetaName = "something"

	config.PortSelector = network.RulePortSelector{
		Ports: network.PortRanges{
			{Lo: 1000, Hi: 2000},
			{Lo: 3000, Hi: 4000},
		},
		Protocol: nethelpers.ProtocolTCP,
	}

	config.Ingress = network.IngressConfig{
		{
			Subnet: netip.MustParsePrefix("10.42.0.0/16"),
			Except: network.Prefix{Prefix: netip.MustParsePrefix("10.42.43.0/24")},
		},
	}

	if modifications != nil {
		modifications(config)
	}

	configBytes, err := yaml.Marshal(config)
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
