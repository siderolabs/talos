// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imager_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/imager"
	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/reporter"
)

// buildBaseInstaller writes a minimal linux/amd64 "installer" image as an OCI layout.
func buildBaseInstaller(t *testing.T) string {
	t.Helper()

	baseImg, err := mutate.ConfigFile(empty.Image, &v1.ConfigFile{
		Architecture: "amd64",
		OS:           "linux",
		Config: v1.Config{
			Entrypoint: []string{"/bin/installer"},
		},
	})
	require.NoError(t, err)

	baseImg, err = mutate.AppendLayers(baseImg, static.NewLayer([]byte("base-layer"), types.OCILayer))
	require.NoError(t, err)

	ociPath := filepath.Join(t.TempDir(), "oci")

	ociLayout, err := layout.Write(ociPath, empty.Index)
	require.NoError(t, err)

	require.NoError(t, ociLayout.AppendImage(baseImg, layout.WithPlatform(v1.Platform{OS: "linux", Architecture: "amd64"})))

	return ociPath
}

func TestOutInstallerReproducible(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "1700000000")

	// force a non-UTC local timezone: the image config must not depend on it
	oldLocal := time.Local                    //nolint:gosmopolitan
	time.Local = time.FixedZone("TEST", 3600) //nolint:gosmopolitan

	t.Cleanup(func() { time.Local = oldLocal }) //nolint:gosmopolitan

	ociPath := buildBaseInstaller(t)

	assetsDir := t.TempDir()
	kernelPath := filepath.Join(assetsDir, "vmlinuz")
	initramfsPath := filepath.Join(assetsDir, "initramfs.xz")

	require.NoError(t, os.WriteFile(kernelPath, []byte("kernel"), 0o644))
	require.NoError(t, os.WriteFile(initramfsPath, []byte("initramfs"), 0o644))

	digests := make([]v1.Hash, 0, 2)

	for range 2 {
		tempDir := t.TempDir()

		prof := profile.Profile{
			Arch:    "amd64",
			Version: "1.9.0",
			Input: profile.Input{
				Kernel: profile.FileAsset{Path: kernelPath},
				BaseInstaller: profile.ContainerAsset{
					ImageRef: "ghcr.io/siderolabs/installer:v1.9.0",
					OCIPath:  ociPath,
				},
			},
		}

		outPath := filepath.Join(tempDir, "installer.tar")

		img := imager.NewForInstallerTest(prof, tempDir, initramfsPath)
		require.NoError(t, img.OutInstaller(t.Context(), outPath, reporter.New()))

		installerImg, err := tarball.ImageFromPath(outPath, nil)
		require.NoError(t, err)

		rawConfig, err := installerImg.RawConfigFile()
		require.NoError(t, err)

		assert.Contains(t, string(rawConfig), `"created":"2023-11-14T22:13:20Z"`)

		digest, err := installerImg.Digest()
		require.NoError(t, err)

		digests = append(digests, digest)
	}

	assert.Equal(t, digests[0], digests[1], "installer image digest is not reproducible")
}
