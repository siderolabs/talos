// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/install"
)

func TestWithGrubUseUKICmdline(t *testing.T) {
	options := install.DefaultInstallOptions()

	require.NoError(t, options.Apply(install.WithGrubUseUKICmdline(true)))
	require.True(t, options.GrubUseUKICmdline)
}

func TestEnvironmentIncludesGrubUseUKICmdline(t *testing.T) {
	options := install.DefaultInstallOptions()
	require.NoError(t, options.Apply(install.WithGrubUseUKICmdline(true)))

	require.Equal(t,
		[]string{"EXISTING=value", "INSTALLER_GRUB_USE_UKI_CMDLINE=true"},
		options.Environment([]string{"EXISTING=value"}),
	)
}

func TestEnvironmentOverridesGrubUseUKICmdline(t *testing.T) {
	options := install.DefaultInstallOptions()
	require.NoError(t, options.Apply(install.WithGrubUseUKICmdline(true)))

	require.Equal(t,
		[]string{"EXISTING=value", "INSTALLER_GRUB_USE_UKI_CMDLINE=true"},
		options.Environment([]string{"EXISTING=value", "INSTALLER_GRUB_USE_UKI_CMDLINE=false"}),
	)
}

func TestEnvironmentRemovesUnrequestedGrubUseUKICmdline(t *testing.T) {
	options := install.DefaultInstallOptions()

	require.Equal(t,
		[]string{"EXISTING=value"},
		options.Environment([]string{"EXISTING=value", "INSTALLER_GRUB_USE_UKI_CMDLINE=true"}),
	)
}
