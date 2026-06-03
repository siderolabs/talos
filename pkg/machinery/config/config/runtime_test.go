// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// kvConfig is a minimal stub implementing both config.SysctlConfig and config.SysfsConfig.
type kvConfig map[string]string

func (c kvConfig) Sysctls() map[string]string { return c }
func (c kvConfig) Sysfs() map[string]string   { return c }

func TestWrapSysctlConfigList(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		configs  []config.SysctlConfig
		expected map[string]string
	}{
		{
			name:     "empty",
			configs:  nil,
			expected: map[string]string{},
		},
		{
			name:     "single",
			configs:  []config.SysctlConfig{kvConfig{"a": "1", "b": "2"}},
			expected: map[string]string{"a": "1", "b": "2"},
		},
		{
			name: "disjoint merge",
			configs: []config.SysctlConfig{
				kvConfig{"a": "1"},
				kvConfig{"b": "2"},
			},
			expected: map[string]string{"a": "1", "b": "2"},
		},
		{
			name: "collision - later wins",
			configs: []config.SysctlConfig{
				kvConfig{"a": "1", "shared": "v1alpha1"},
				kvConfig{"b": "2", "shared": "document"},
			},
			expected: map[string]string{"a": "1", "b": "2", "shared": "document"},
		},
		{
			name: "collision across three - last wins",
			configs: []config.SysctlConfig{
				kvConfig{"shared": "first"},
				kvConfig{"shared": "second"},
				kvConfig{"shared": "third"},
			},
			expected: map[string]string{"shared": "third"},
		},
		{
			name: "nil map entry is skipped",
			configs: []config.SysctlConfig{
				kvConfig(nil),
				kvConfig{"a": "1"},
			},
			expected: map[string]string{"a": "1"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, config.WrapSysctlConfigList(test.configs...))
		})
	}
}

func TestWrapSysfsConfigList(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		configs  []config.SysfsConfig
		expected map[string]string
	}{
		{
			name:     "empty",
			configs:  nil,
			expected: map[string]string{},
		},
		{
			name: "collision - later wins",
			configs: []config.SysfsConfig{
				kvConfig{"a": "1", "shared": "v1alpha1"},
				kvConfig{"b": "2", "shared": "document"},
			},
			expected: map[string]string{"a": "1", "b": "2", "shared": "document"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, config.WrapSysfsConfigList(test.configs...))
		})
	}
}
