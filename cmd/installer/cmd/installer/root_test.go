// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage
package installer

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/installer/pkg/install"
	installerexitcode "github.com/siderolabs/talos/pkg/installer/exitcode"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestSetFlagsFromEnvironmentGrubUseUKICmdline(t *testing.T) {
	t.Setenv(constants.InstallerGrubUseUKICmdlineEnvVar, "true")

	oldOptions := options
	options = &install.Options{}

	t.Cleanup(func() { options = oldOptions })

	require.NoError(t, setFlagsFromEnvironment())
	require.True(t, options.GrubUseUKICmdline)
}

func TestExecuteInvalidMetaEnv(t *testing.T) {
	t.Setenv(constants.MetaValuesEnvVar, "!!!")

	err := execute()
	require.Error(t, err)
	require.Equal(t, constants.ExitInvalidInput, installerexitcode.Resolve(err))
}
