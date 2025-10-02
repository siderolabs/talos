// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package blockutils_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/blockutils"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestVolumeMatch(t *testing.T) {
	t.Parallel()

	expr, err := blockutils.VolumeMatch([]string{constants.MetalConfigISOLabel})
	require.NoError(t, err)

	assert.Equal(t, `(volume.label in ["metal-iso"] || volume.partition_label in ["metal-iso"]) && volume.name != ""`, expr.String())
}
