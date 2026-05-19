// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package meta_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
)

func TestArgsMarshalYAML(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		filename string
		args     meta.Args
	}{
		{
			name:     "strings",
			filename: "args_strings.yaml",
			args: meta.Args{
				"feature-gates":                    meta.NewArgValue("ServerSideApply=true", nil),
				"http2-max-streams-per-connection": meta.NewArgValue("32", nil),
			},
		},
		{
			name:     "lists",
			filename: "args_lists.yaml",
			args: meta.Args{
				"api-audiences":     meta.NewArgValue("", []string{"kubernetes.default.svc", "test.local"}),
				"oidc-groups-claim": meta.NewArgValue("", []string{"groups"}),
			},
		},
		{
			name:     "mixed",
			filename: "args_mixed.yaml",
			args: meta.Args{
				"feature-gates":     meta.NewArgValue("ServerSideApply=true", nil),
				"advertise-address": meta.NewArgValue("10.0.0.1", nil),
				"api-audiences":     meta.NewArgValue("", []string{"kubernetes.default.svc", "api.cluster.local"}),
				"runtime-config":    meta.NewArgValue("", []string{"api/all=true", "api/beta=false"}),
			},
		},
		{
			name:     "single-element list",
			filename: "args_single_element_list.yaml",
			args: meta.Args{
				"enable-admission-plugins": meta.NewArgValue("", []string{"PodSecurity"}),
			},
		},
		{
			name:     "empty",
			filename: "args_empty.yaml",
			args:     meta.Args{},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			marshaled, err := yaml.Marshal(test.args)
			require.NoError(t, err)

			expected, err := os.ReadFile(filepath.Join("testdata", test.filename))
			require.NoError(t, err)

			assert.Equal(t, string(expected), string(marshaled))

			var decoded meta.Args

			require.NoError(t, yaml.Unmarshal(expected, &decoded))
			assert.Equal(t, test.args.ToMap(), decoded.ToMap())
		})
	}
}

func TestArgValueUnmarshalYAML(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		input    string
		expected map[string][]string
	}{
		{
			name:  "scalar string",
			input: "key: value\n",
			expected: map[string][]string{
				"key": {"value"},
			},
		},
		{
			name:  "list of strings",
			input: "key:\n  - a\n  - b\n",
			expected: map[string][]string{
				"key": {"a", "b"},
			},
		},
		{
			name:  "empty list",
			input: "key: []\n",
			expected: map[string][]string{
				"key": {},
			},
		},
		{
			name:  "quoted numeric string",
			input: "key: \"32\"\n",
			expected: map[string][]string{
				"key": {"32"},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var args meta.Args

			require.NoError(t, yaml.Unmarshal([]byte(test.input), &args))
			assert.Equal(t, test.expected, args.ToMap())
		})
	}
}

func TestArgValueUnmarshalYAMLError(t *testing.T) {
	t.Parallel()

	var args meta.Args

	err := yaml.Unmarshal([]byte("key:\n  nested: object\n"), &args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "arg value must be a string or list of strings")
}

func TestArgsMerge(t *testing.T) {
	t.Parallel()

	t.Run("into nil receiver", func(t *testing.T) {
		t.Parallel()

		var a meta.Args

		require.NoError(t, a.Merge(meta.Args{
			"foo": meta.NewArgValue("bar", nil),
		}))
		assert.Equal(t, map[string][]string{"foo": {"bar"}}, a.ToMap())
	})

	t.Run("overwrites existing keys", func(t *testing.T) {
		t.Parallel()

		a := meta.Args{
			"foo": meta.NewArgValue("old", nil),
			"baz": meta.NewArgValue("keep", nil),
		}

		require.NoError(t, a.Merge(meta.Args{
			"foo": meta.NewArgValue("new", nil),
			"qux": meta.NewArgValue("", []string{"x", "y"}),
		}))

		assert.Equal(t, map[string][]string{
			"foo": {"new"},
			"baz": {"keep"},
			"qux": {"x", "y"},
		}, a.ToMap())
	})

	t.Run("empty other is a no-op", func(t *testing.T) {
		t.Parallel()

		a := meta.Args{"foo": meta.NewArgValue("bar", nil)}

		require.NoError(t, a.Merge(meta.Args{}))
		assert.Equal(t, map[string][]string{"foo": {"bar"}}, a.ToMap())
	})

	t.Run("wrong type", func(t *testing.T) {
		t.Parallel()

		a := meta.Args{}

		err := a.Merge(map[string]string{"foo": "bar"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot merge Args with")
	})
}
