// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/container/internal/files"
)

func TestReadHostname(t *testing.T) {
	t.Parallel()

	spec, err := files.ReadHostname("testdata/hostname")
	require.NoError(t, err)

	require.Equal(t, "foo", spec.Hostname)
	require.Equal(t, "example.com", spec.Domainname)
}
