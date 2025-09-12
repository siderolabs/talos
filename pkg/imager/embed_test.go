// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imager_test

import (
	"archive/tar"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/imager"
)

func TestBuildEmbeddedConfigExtension(t *testing.T) {
	t.Parallel()

	out, err := imager.BuildEmbeddedConfigExtension([]byte("test"))
	require.NoError(t, err)

	tr := tar.NewReader(out)

	var paths []string

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		require.NoError(t, err)

		paths = append(paths, hdr.Name)
	}

	assert.Equal(t, []string{
		"manifest.yaml",
		"rootfs",
		"rootfs/usr",
		"rootfs/usr/local",
		"rootfs/usr/local/etc",
		"rootfs/usr/local/etc/talos",
		"rootfs/usr/local/etc/talos/config.yaml",
	}, paths)
}
