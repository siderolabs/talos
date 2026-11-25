// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
)

func TestValidateAliasName(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"data_1", "disk.0", "a-b", "A9"} {
		require.NoError(t, libvirtdomain.ValidateAliasName(name), name)
	}

	// "/" and "@" separate a disk name in its status ID.
	for _, name := range []string{"", "a@b", "a/b", "a b", "ä"} {
		require.Error(t, libvirtdomain.ValidateAliasName(name), name)
	}
}
