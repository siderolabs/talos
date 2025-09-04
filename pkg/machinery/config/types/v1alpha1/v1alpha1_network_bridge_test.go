// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

// These tests ensure that v1alpha1 types properly implement new-style config interfaces.

func TestStaticHostsBridging(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config
	}{
		{
			name: "v1alpha1 only",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineNetwork: &v1alpha1.NetworkConfig{
							ExtraHostEntries: []*v1alpha1.ExtraHost{
								{
									HostIP: "10.5.0.2",
									HostAliases: []string{
										"example.com",
										"example",
									},
								},
								{
									HostIP: "10.5.0.3",
									HostAliases: []string{
										"my-machine",
									},
								},
								{
									HostIP: "2001:db8::1",
									HostAliases: []string{
										"ipv6-host",
									},
								},
							},
						},
					},
				})
			},
		},
		{
			name: "new style only",

			cfg: func(*testing.T) config.Config {
				host1 := network.NewStaticHostConfigV1Alpha1("10.5.0.2")
				host1.Hostnames = []string{"example.com", "example"}

				host2 := network.NewStaticHostConfigV1Alpha1("10.5.0.3")
				host2.Hostnames = []string{"my-machine"}

				host3 := network.NewStaticHostConfigV1Alpha1("2001:db8::1")
				host3.Hostnames = []string{"ipv6-host"}

				c, err := container.New(
					host1,
					host2,
					host3,
				)
				require.NoError(t, err)

				return c
			},
		},
		{
			name: "mixed",

			cfg: func(*testing.T) config.Config {
				host2 := network.NewStaticHostConfigV1Alpha1("10.5.0.3")
				host2.Hostnames = []string{"my-machine"}

				host3 := network.NewStaticHostConfigV1Alpha1("2001:db8::1")
				host3.Hostnames = []string{"ipv6-host"}

				c, err := container.New(
					host2,
					host3,
					&v1alpha1.Config{
						MachineConfig: &v1alpha1.MachineConfig{
							MachineNetwork: &v1alpha1.NetworkConfig{
								ExtraHostEntries: []*v1alpha1.ExtraHost{
									{
										HostIP: "10.5.0.2",
										HostAliases: []string{
											"example.com",
											"example",
										},
									},
								},
							},
						},
					},
				)
				require.NoError(t, err)

				return c
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			staticHosts := cfg.NetworkStaticHostConfig()

			require.Len(t, staticHosts, 3)

			assert.Equal(t, "10.5.0.2", staticHosts[0].IP())
			assert.Equal(t, []string{"example.com", "example"}, staticHosts[0].Aliases())

			assert.Equal(t, "10.5.0.3", staticHosts[1].IP())
			assert.Equal(t, []string{"my-machine"}, staticHosts[1].Aliases())

			assert.Equal(t, "2001:db8::1", staticHosts[2].IP())
			assert.Equal(t, []string{"ipv6-host"}, staticHosts[2].Aliases())
		})
	}
}
