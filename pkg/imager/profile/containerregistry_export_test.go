// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile_test

import (
	"archive/tar"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/require"
)

func TestCraneExport(t *testing.T) {
	t.Parallel()

	layer, err := tarball.LayerFromFile("testdata/relative-symlinks-layer.tar")
	require.NoError(t, err)

	img, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)

	r, w := io.Pipe()

	defer r.Close() //nolint:errcheck

	go func() {
		defer w.Close() //nolint:errcheck

		if exportErr := crane.Export(img, w); exportErr != nil {
			w.CloseWithError(exportErr)
		}
	}()

	var symlinkTargets []string

	tr := tar.NewReader(r)

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		require.NoError(t, err, "failed to read tar entry")

		if hdr.Typeflag == tar.TypeSymlink && strings.HasPrefix(hdr.Linkname, "../") {
			symlinkTargets = append(symlinkTargets, hdr.Linkname)
		}
	}

	require.ElementsMatch(t, []string{"../run/lock", "../run"}, symlinkTargets)
}
