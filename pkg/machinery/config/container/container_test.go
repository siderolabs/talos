// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"net/url"
	"testing"

	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

func TestNew(t *testing.T) {
	v1alpha1Cfg := &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineFeatures: &v1alpha1.FeaturesConfig{
				RBAC: pointer.To(true),
			},
		},
		ClusterConfig: &v1alpha1.ClusterConfig{
			ClusterSecret: "topsecret",
		},
	}

	sideroLinkCfg := siderolink.NewConfigV1Alpha1()
	sideroLinkCfg.APIUrlConfig.URL = must(url.Parse("https://siderolink.api/join?jointoken=secret&user=alice"))

	cfg, err := container.New(v1alpha1Cfg, sideroLinkCfg)
	require.NoError(t, err)

	assert.False(t, cfg.Readonly())
	assert.False(t, cfg.Debug())
	assert.True(t, cfg.Machine().Features().RBACEnabled())
	assert.Equal(t, "topsecret", cfg.Cluster().Secret())
	assert.Equal(t, "https://siderolink.api/join?jointoken=secret&user=alice", cfg.SideroLink().APIUrl().String())
	assert.Same(t, v1alpha1Cfg, cfg.RawV1Alpha1())

	bytes, err := cfg.Bytes()
	require.NoError(t, err)

	cfgBack, err := configloader.NewFromBytes(bytes)
	require.NoError(t, err)

	assert.True(t, cfgBack.Readonly())
	assert.NotEqual(t, v1alpha1Cfg, cfgBack.RawV1Alpha1())

	cfgRedacted := cfg.RedactSecrets("REDACTED")
	assert.Equal(t, "REDACTED", cfgRedacted.Cluster().Secret())
	assert.Equal(t, "https://siderolink.api/join?jointoken=REDACTED&user=alice", cfgRedacted.SideroLink().APIUrl().String())
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}

	return t
}
