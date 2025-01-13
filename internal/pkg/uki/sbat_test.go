// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package uki_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/uki"
)

func TestGetSBAT(t *testing.T) {
	t.Parallel()

	data, err := uki.GetSBAT("internal/pe/testdata/sd-stub-amd64.efi")
	require.NoError(t, err)

	require.Equal(t,
		"sbat,1,SBAT Version,sbat,1,https://github.com/rhboot/shim/blob/main/SBAT.md\nsystemd,1,The systemd Developers,systemd,254,https://systemd.io/\nsystemd.talos,1,Talos Linux,systemd,254,https://github.com/siderolabs/tools/issues\n\x00", //nolint:lll
		string(data),
	)
}
