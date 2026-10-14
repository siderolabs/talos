// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	containercfg "github.com/siderolabs/talos/pkg/machinery/config/types/container"
)

// newVolumeMountContainerDoc builds a ContainerConfig document mounting the named volume.
func newVolumeMountContainerDoc(volumeName string, options ...string) *containercfg.ContainerConfigV1Alpha1 {
	doc := containercfg.NewContainerConfigV1Alpha1()
	doc.MetaName = "app"
	doc.ContainerImage = "docker.io/library/nginx:1.27"
	doc.MountsConfig = []containercfg.ContainerMount{
		{
			VolumeMount: &containercfg.VolumeMount{
				VolumeName:       volumeName,
				MountDestination: "/data",
				MountOpts:        options,
			},
		},
	}

	return doc
}

func TestContainerVolumeMounts(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name         string
		docs         []config.Document
		expectedErrs []string
	}{
		{
			name: "user volume",
			docs: []config.Document{
				newUserVolumeDoc("data"),
				newVolumeMountContainerDoc("data"),
			},
		},
		{
			name: "writable existing volume",
			docs: []config.Document{
				newExistingVolumeDoc("data", false),
				newVolumeMountContainerDoc("data"),
			},
		},
		{
			name: "writable external volume",
			docs: []config.Document{
				newExternalVolumeDoc("data", false),
				newVolumeMountContainerDoc("data"),
			},
		},
		{
			name: "read-only existing volume mounted ro",
			docs: []config.Document{
				newExistingVolumeDoc("data", true),
				newVolumeMountContainerDoc("data", "ro"),
			},
		},
		{
			name: "read-only external volume mounted ro",
			docs: []config.Document{
				newExternalVolumeDoc("data", true),
				newVolumeMountContainerDoc("data", "ro"),
			},
		},
		{
			name: "volume not configured",
			docs: []config.Document{
				newVolumeMountContainerDoc("nowhere"),
			},
			expectedErrs: []string{
				`container "app": mounts[0]: no UserVolumeConfig, ExistingVolumeConfig or ExternalVolumeConfig declares volume "nowhere"`,
			},
		},
		{
			// The ID is derived from the document kind and the name, so the prefixed ID is not a name.
			name: "volume named by its ID",
			docs: []config.Document{
				newUserVolumeDoc("data"),
				newVolumeMountContainerDoc("u-data"),
			},
			expectedErrs: []string{
				`container "app": mounts[0]: no UserVolumeConfig, ExistingVolumeConfig or ExternalVolumeConfig declares volume "u-data"`,
			},
		},
		{
			name: "read-only existing volume mounted writable",
			docs: []config.Document{
				newExistingVolumeDoc("data", true),
				newVolumeMountContainerDoc("data"),
			},
			expectedErrs: []string{
				`container "app": mounts[0]: volume "data" is configured read-only, so it must be mounted with the ro option`,
			},
		},
		{
			name: "read-only external volume mounted rw",
			docs: []config.Document{
				newExternalVolumeDoc("data", true),
				newVolumeMountContainerDoc("data", "rw"),
			},
			expectedErrs: []string{
				`container "app": mounts[0]: volume "data" is configured read-only, so it must be mounted with the ro option`,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := validateCrossDocs(t, test.docs...)

			if len(test.expectedErrs) == 0 {
				if err != nil {
					// The bare volume documents fail their own validation, which is not what this
					// test is about.
					assert.NotContains(t, err.Error(), "mounts[0]")
				}

				return
			}

			require.Error(t, err)

			for _, expectedErr := range test.expectedErrs {
				assert.ErrorContains(t, err, expectedErr)
			}
		})
	}
}
