// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/images"
)

func TestContainerAssetIsSet(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		asset profile.ContainerAsset
		want  bool
	}{
		{
			name: "zero",
		},
		{
			name: "force insecure only",
			asset: profile.ContainerAsset{
				ForceInsecure: true,
			},
		},
		{
			name: "image ref",
			asset: profile.ContainerAsset{
				ImageRef: "registry.local/foo:v1",
			},
			want: true,
		},
		{
			name: "tarball path",
			asset: profile.ContainerAsset{
				TarballPath: "/tmp/image.tar",
			},
			want: true,
		},
		{
			name: "oci path",
			asset: profile.ContainerAsset{
				OCIPath: "/tmp/oci",
			},
			want: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, tt.asset.IsSet())
		})
	}
}

func TestInputFillDefaultsBaseInstallerForceInsecureOnly(t *testing.T) {
	t.Parallel()

	input := profile.Input{
		BaseInstaller: profile.ContainerAsset{
			ForceInsecure: true,
		},
	}

	input.FillDefaults("amd64", "1.14.0", false)

	require.Equal(t, fmt.Sprintf("%s:%s", images.DefaultInstallerBaseImageRepository, "1.14.0"), input.BaseInstaller.ImageRef)
	require.True(t, input.BaseInstaller.ForceInsecure)
}

func TestInputFillDefaultsPreservesExplicitBaseInstaller(t *testing.T) {
	t.Parallel()

	input := profile.Input{
		BaseInstaller: profile.ContainerAsset{
			ImageRef:      "registry.local/foo/installer-base:v1",
			ForceInsecure: true,
		},
	}

	input.FillDefaults("amd64", "1.14.0", false)

	require.Equal(t, "registry.local/foo/installer-base:v1", input.BaseInstaller.ImageRef)
	require.True(t, input.BaseInstaller.ForceInsecure)
}
