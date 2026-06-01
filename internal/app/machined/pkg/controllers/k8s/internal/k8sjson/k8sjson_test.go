// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8sjson_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8sjsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s/internal/k8sjson"
)

//nolint:gocyclo
func TestDeepCopyToJSON(t *testing.T) {
	t.Parallel()

	type stringly string

	for _, tt := range []struct {
		name     string
		input    any
		expected any
	}{
		{
			name:     "nil any",
			input:    nil,
			expected: nil,
		},
		{
			name:     "typed nil map",
			input:    map[string]any(nil),
			expected: map[string]any(nil),
		},
		{
			name:     "typed nil slice",
			input:    []any(nil),
			expected: []any(nil),
		},
		{
			name:     "string passes through",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "bool passes through",
			input:    true,
			expected: true,
		},
		{
			name:     "int -> int64",
			input:    int(42),
			expected: int64(42),
		},
		{
			name:     "negative int -> int64",
			input:    int(-7),
			expected: int64(-7),
		},
		{
			name:     "int8 -> int64",
			input:    int8(8),
			expected: int64(8),
		},
		{
			name:     "int16 -> int64",
			input:    int16(16),
			expected: int64(16),
		},
		{
			name:     "int32 -> int64",
			input:    int32(32),
			expected: int64(32),
		},
		{
			name:     "int64 passes through",
			input:    int64(64),
			expected: int64(64),
		},
		{
			name:     "uint -> int64",
			input:    uint(1),
			expected: int64(1),
		},
		{
			name:     "uint8 -> int64",
			input:    uint8(8),
			expected: int64(8),
		},
		{
			name:     "uint16 -> int64",
			input:    uint16(16),
			expected: int64(16),
		},
		{
			name:     "uint32 -> int64",
			input:    uint32(32),
			expected: int64(32),
		},
		{
			name:     "uint64 -> float64",
			input:    uint64(1 << 40),
			expected: float64(1 << 40),
		},
		{
			name:     "float32 -> float64",
			input:    float32(1.5),
			expected: float64(float32(1.5)),
		},
		{
			name:     "float64 passes through",
			input:    float64(2.5),
			expected: float64(2.5),
		},
		{
			name:     "unknown type passes through unchanged",
			input:    stringly("named"),
			expected: stringly("named"),
		},
		{
			name: "flat map normalizes ints",
			input: map[string]any{
				"a": int(1),
				"b": "two",
				"c": float64(3.5),
			},
			expected: map[string]any{
				"a": int64(1),
				"b": "two",
				"c": float64(3.5),
			},
		},
		{
			name:  "flat slice normalizes ints",
			input: []any{int(1), int32(2), "three", nil},
			expected: []any{
				int64(1),
				int64(2),
				"three",
				nil,
			},
		},
		{
			name: "deeply nested ints are normalized",
			input: map[string]any{
				"profiles": []any{
					map[string]any{
						"pluginConfig": []any{
							map[string]any{
								"args": map[string]any{
									"defaultConstraints": []any{
										map[string]any{
											"maxSkew":           int(1),
											"topologyKey":       "kubernetes.io/hostname",
											"whenUnsatisfiable": "ScheduleAnyway",
										},
									},
								},
							},
						},
					},
				},
			},
			expected: map[string]any{
				"profiles": []any{
					map[string]any{
						"pluginConfig": []any{
							map[string]any{
								"args": map[string]any{
									"defaultConstraints": []any{
										map[string]any{
											"maxSkew":           int64(1),
											"topologyKey":       "kubernetes.io/hostname",
											"whenUnsatisfiable": "ScheduleAnyway",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "empty map is preserved as non-nil",
			input: map[string]any{
				"empty": map[string]any{},
			},
			expected: map[string]any{
				"empty": map[string]any{},
			},
		},
		{
			name: "empty slice is preserved as non-nil",
			input: map[string]any{
				"empty": []any{},
			},
			expected: map[string]any{
				"empty": []any{},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := k8sjson.DeepCopyToJSON(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestDeepCopyToJSON_DeepCopySemantics verifies that mutating the returned
// value does not mutate the input — this is a deep copy, not a normalize
// in-place.
func TestDeepCopyToJSON_DeepCopySemantics(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"outer": map[string]any{
			"inner": []any{int(1), int(2)},
		},
	}

	cloned, ok := k8sjson.DeepCopyToJSON(input).(map[string]any)
	require.True(t, ok)

	clonedOuter := cloned["outer"].(map[string]any) //nolint:forcetypeassert
	clonedOuter["inner"] = []any{int64(99)}
	clonedOuter["added"] = "new"

	inputOuter := input["outer"].(map[string]any) //nolint:forcetypeassert
	assert.Equal(t, []any{int(1), int(2)}, inputOuter["inner"], "input slice should be untouched")
	assert.NotContains(t, inputOuter, "added", "input map should be untouched")
}

// TestDeepCopyToJSON_Serializer verifies the full pipeline: a YAML document
// from testdata/ is parsed, normalized via DeepCopyToJSON, wrapped in
// unstructured.Unstructured, and serialized to JSON using the same Kubernetes
// JSON serializer that render_config_static_pods.go uses. The result is
// compared to the expected JSON from testdata/. Without DeepCopyToJSON, the
// serializer panics on go-yaml's native int values.
func TestDeepCopyToJSON_Serializer(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"scheduler-int-fields",
		"scheduler-topology-spread",
		"scheduler-mixed-types",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			yamlBytes, err := os.ReadFile(filepath.Join("testdata", name+".yaml"))
			require.NoError(t, err)

			var raw map[string]any

			require.NoError(t, yaml.Unmarshal(yamlBytes, &raw))

			normalized, ok := k8sjson.DeepCopyToJSON(raw).(map[string]any)
			require.True(t, ok)

			obj := runtime.Object(&unstructured.Unstructured{Object: normalized})

			serializer := k8sjsonserializer.NewSerializerWithOptions(
				k8sjsonserializer.DefaultMetaFactory, nil, nil,
				k8sjsonserializer.SerializerOptions{
					Yaml:   false,
					Pretty: true,
					Strict: true,
				},
			)

			var buf bytes.Buffer

			require.NoError(t, serializer.Encode(obj, &buf))

			expected, err := os.ReadFile(filepath.Join("testdata", name+".json"))
			require.NoError(t, err)

			assert.JSONEq(t, string(expected), buf.String())
		})
	}
}
