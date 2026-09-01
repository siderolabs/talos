// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mountconfig_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/mountconfig"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestParametersChanged(t *testing.T) {
	initial := []block.ParameterSpec{
		block.NewStringParameter("vers", "3"),
		block.NewBooleanParameter("nolock"),
	}
	snapshot := mountconfig.NewSnapshot(initial)

	require.False(t, snapshot.ParametersChanged(initial))
	require.True(t, snapshot.ParametersChanged([]block.ParameterSpec{
		block.NewStringParameter("vers", "4.1"),
	}))
}

func TestSnapshotDoesNotAliasInput(t *testing.T) {
	initial := []block.ParameterSpec{block.NewStringParameter("vers", "3")}
	snapshot := mountconfig.NewSnapshot(initial)

	initial[0] = block.NewStringParameter("vers", "4.1")

	require.False(t, snapshot.ParametersChanged([]block.ParameterSpec{
		block.NewStringParameter("vers", "3"),
	}))
}
