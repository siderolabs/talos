// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package environment_test

import (
	"testing"

	"github.com/siderolabs/go-procfs/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/environment"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

func TestGet(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		cmdline string
		cfg     map[string]string

		expected []string
	}{
		{
			name: "empty",
		},
		{
			name: "machine config only",
			cfg: map[string]string{
				"http_proxy": "http://proxy.example.com:8080",
			},
			expected: []string{
				"http_proxy=http://proxy.example.com:8080",
			},
		},
		{
			name:    "cmdline only",
			cmdline: "talos.environment=foo=bar talos.environment=bar=baz",
			expected: []string{
				"foo=bar",
				"bar=baz",
			},
		},
		{
			name:    "cmdline and machine config",
			cmdline: "talos.environment=foo=bar",
			cfg: map[string]string{
				"http_proxy": "http://proxy.example.com:8080",
			},
			expected: []string{
				"foo=bar",
				"http_proxy=http://proxy.example.com:8080",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cmdline := procfs.NewCmdline(test.cmdline)

			var cfg config.Config

			if test.cfg != nil {
				var err error

				cfg, err = container.New(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineEnv: test.cfg,
					},
				})
				require.NoError(t, err)
			}

			result := environment.GetCmdline(cmdline, cfg)

			assert.Equal(t, test.expected, result)
		})
	}
}
