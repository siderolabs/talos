// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
)

func TestConfigMerge(t *testing.T) {
	context1 := &clientconfig.Context{}
	context2 := &clientconfig.Context{}

	for _, tt := range []struct {
		name          string
		config        *clientconfig.Config
		configToMerge *clientconfig.Config

		expectedContext  string
		expectedContexts map[string]*clientconfig.Context
	}{
		{
			name:   "IntoEmpty",
			config: &clientconfig.Config{},
			configToMerge: &clientconfig.Config{
				Context: "foo",
				Contexts: map[string]*clientconfig.Context{
					"foo": context1,
				},
			},

			expectedContext: "foo",
			expectedContexts: map[string]*clientconfig.Context{
				"foo": context1,
			},
		},
		{
			name: "NoConflict",
			config: &clientconfig.Config{
				Context: "bar",
				Contexts: map[string]*clientconfig.Context{
					"bar": context2,
				},
			},
			configToMerge: &clientconfig.Config{
				Context: "",
				Contexts: map[string]*clientconfig.Context{
					"foo": context1,
				},
			},

			expectedContext: "bar",
			expectedContexts: map[string]*clientconfig.Context{
				"foo": context1,
				"bar": context2,
			},
		},
		{
			name: "WithRename",
			config: &clientconfig.Config{
				Context: "bar",
				Contexts: map[string]*clientconfig.Context{
					"bar": context2,
				},
			},
			configToMerge: &clientconfig.Config{
				Context: "bar",
				Contexts: map[string]*clientconfig.Context{
					"bar": context1,
				},
			},

			expectedContext: "bar-1",
			expectedContexts: map[string]*clientconfig.Context{
				"bar-1": context1,
				"bar":   context2,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.config
			c.Merge(tt.configToMerge)

			assert.Equal(t, tt.expectedContext, c.Context)
			assert.Equal(t, tt.expectedContexts, c.Contexts)
		})
	}
}
