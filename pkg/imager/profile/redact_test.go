// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/meta"
)

const embeddedConfig = `version: v1alpha1
machine:
  type: worker
  token: machine-token-secret
cluster:
  secret: cluster-secret-value
  controlPlane:
    endpoint: https://example.com:6443
`

func TestRedacted(t *testing.T) {
	t.Parallel()

	prof := profile.Profile{
		Arch:     "amd64",
		Platform: "metal",
		Customization: profile.CustomizationProfile{
			ExtraKernelArgs: []string{"console=ttyS0"},
			MetaContents: meta.Values{
				{Key: meta.UniqueMachineToken, Value: "unique-token-secret"},
				{Key: meta.UserReserved1, Value: "user-data"},
			},
			EmbeddedMachineConfiguration: embeddedConfig,
		},
	}

	redacted := prof.Redacted()

	assert.Equal(t, meta.Values{
		{Key: meta.UniqueMachineToken, Value: constants.Redacted},
		{Key: meta.UserReserved1, Value: "user-data"},
	}, redacted.Customization.MetaContents)

	assert.NotContains(t, redacted.Customization.EmbeddedMachineConfiguration, "machine-token-secret")
	assert.NotContains(t, redacted.Customization.EmbeddedMachineConfiguration, "cluster-secret-value")
	assert.Contains(t, redacted.Customization.EmbeddedMachineConfiguration, constants.Redacted)
	assert.Contains(t, redacted.Customization.EmbeddedMachineConfiguration, "https://example.com:6443")

	assert.Equal(t, []string{"console=ttyS0"}, redacted.Customization.ExtraKernelArgs)

	// the original profile is left as it was
	assert.Equal(t, "unique-token-secret", prof.Customization.MetaContents[0].Value)
	assert.Equal(t, embeddedConfig, prof.Customization.EmbeddedMachineConfiguration)

	var dump strings.Builder

	require.NoError(t, redacted.Dump(&dump))

	assert.NotContains(t, dump.String(), "unique-token-secret")
	assert.NotContains(t, dump.String(), "machine-token-secret")
	assert.NotContains(t, dump.String(), "cluster-secret-value")
}

func TestRedactedInvalidConfig(t *testing.T) {
	t.Parallel()

	prof := profile.Profile{
		Customization: profile.CustomizationProfile{
			EmbeddedMachineConfiguration: "not: [a valid config",
		},
	}

	assert.Equal(t, constants.Redacted, prof.Redacted().Customization.EmbeddedMachineConfiguration)
}

func TestRedactedEmpty(t *testing.T) {
	t.Parallel()

	prof := profile.Profile{Arch: "amd64"}

	assert.Equal(t, prof, prof.Redacted())
}
