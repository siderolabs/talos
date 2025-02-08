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

	data, err := uki.GetSBAT("internal/pe/testdata/linuxx64.efi.stub")
	require.NoError(t, err)

	require.Equal(t,
		"sbat,1,SBAT Version,sbat,1,https://github.com/rhboot/shim/blob/main/SBAT.md\nsystemd-stub,1,The systemd Developers,systemd,257,https://systemd.io/\nsystemd-stub.talos,1,Talos Linux,systemd,257.2257.2,https://github.com/siderolabs/tools/issues\n", //nolint:lll
		string(data),
	)
}
