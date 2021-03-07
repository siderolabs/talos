// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/pkg/machinery/client/config"
)

func TestConfigMerge(t *testing.T) {
	context1 := &config.Context{}
	context2 := &config.Context{}

	for _, tt := range []struct {
		name          string
		config        *config.Config
		configToMerge *config.Config

		expectedContext  string
		expectedContexts map[string]*config.Context
	}{
		{
			name:   "IntoEmpty",
			config: &config.Config{},
			configToMerge: &config.Config{
				Context: "foo",
				Contexts: map[string]*config.Context{
					"foo": context1,
				},
			},

			expectedContext: "foo",
			expectedContexts: map[string]*config.Context{
				"foo": context1,
			},
		},
		{
			name: "NoConflict",
			config: &config.Config{
				Context: "bar",
				Contexts: map[string]*config.Context{
					"bar": context2,
				},
			},
			configToMerge: &config.Config{
				Context: "",
				Contexts: map[string]*config.Context{
					"foo": context1,
				},
			},

			expectedContext: "bar",
			expectedContexts: map[string]*config.Context{
				"foo": context1,
				"bar": context2,
			},
		},
		{
			name: "WithRename",
			config: &config.Config{
				Context: "bar",
				Contexts: map[string]*config.Context{
					"bar": context2,
				},
			},
			configToMerge: &config.Config{
				Context: "bar",
				Contexts: map[string]*config.Context{
					"bar": context1,
				},
			},

			expectedContext: "bar-1",
			expectedContexts: map[string]*config.Context{
				"bar-1": context1,
				"bar":   context2,
			},
		},
	} {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			c := tt.config
			c.Merge(tt.configToMerge)

			assert.Equal(t, tt.expectedContext, c.Context)
			assert.Equal(t, tt.expectedContexts, c.Contexts)
		})
	}
}
