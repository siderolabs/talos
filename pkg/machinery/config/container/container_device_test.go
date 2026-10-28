// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	containercfg "github.com/siderolabs/talos/pkg/machinery/config/types/container"
)

// newRawVolumeDoc builds a RawVolumeConfig document.
func newRawVolumeDoc(name string) *blockcfg.RawVolumeConfigV1Alpha1 {
	doc := blockcfg.NewRawVolumeConfigV1Alpha1()
	doc.MetaName = name

	return doc
}

// newRawVolumeDeviceContainerDoc builds a ContainerConfig document handing it the named raw volumes.
func newRawVolumeDeviceContainerDoc(containerName string, volumeNames ...string) *containercfg.ContainerConfigV1Alpha1 {
	doc := containercfg.NewContainerConfigV1Alpha1()
	doc.MetaName = containerName
	doc.ContainerImage = "docker.io/library/nginx:1.27"

	for _, volumeName := range volumeNames {
		doc.DevicesConfig = append(doc.DevicesConfig, containercfg.ContainerDevice{
			RawVolumeDevice: &containercfg.RawVolumeDevice{
				VolumeName:        volumeName,
				DeviceDestination: "/dev/" + volumeName,
			},
		})
	}

	return doc
}

// validateRawVolumeDeviceDocs runs container-level validation over the given documents.
//
// The raw volume documents are left bare, so their own validation has plenty to say about them; only
// what the raw volume device check reports is of interest here.
func validateRawVolumeDeviceDocs(t *testing.T, docs ...config.Document) error {
	t.Helper()

	ctr, err := container.New(docs...)
	require.NoError(t, err)

	_, err = ctr.ValidateAsClient(validationMode{})

	return err
}

func TestContainerRawVolumeDevices(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name         string
		docs         []config.Document
		expectedErrs []string
	}{
		{
			name: "raw volume",
			docs: []config.Document{
				newRawVolumeDoc("data"),
				newRawVolumeDeviceContainerDoc("app", "data"),
			},
		},
		{
			name: "raw volumes for different containers",
			docs: []config.Document{
				newRawVolumeDoc("data-1"),
				newRawVolumeDoc("data-2"),
				newRawVolumeDeviceContainerDoc("app-1", "data-1"),
				newRawVolumeDeviceContainerDoc("app-2", "data-2"),
			},
		},
		{
			name: "raw volume not configured",
			docs: []config.Document{
				newRawVolumeDeviceContainerDoc("app", "nowhere"),
			},
			expectedErrs: []string{
				`container "app": devices[0]: no RawVolumeConfig declares raw volume "nowhere"`,
			},
		},
		{
			// A raw volume is resolved by its document kind, so a user volume of the same name is no match.
			name: "user volume named as a raw volume",
			docs: []config.Document{
				newUserVolumeDoc("data"),
				newRawVolumeDeviceContainerDoc("app", "data"),
			},
			expectedErrs: []string{
				`container "app": devices[0]: no RawVolumeConfig declares raw volume "data"`,
			},
		},
		{
			name: "raw volume named by its ID",
			docs: []config.Document{
				newRawVolumeDoc("data"),
				newRawVolumeDeviceContainerDoc("app", "r-data"),
			},
			expectedErrs: []string{
				`container "app": devices[0]: no RawVolumeConfig declares raw volume "r-data"`,
			},
		},
		{
			// Containers are visited by name, so the first one by name owns the raw volume.
			name: "raw volume handed to two containers",
			docs: []config.Document{
				newRawVolumeDoc("data"),
				newRawVolumeDeviceContainerDoc("app-2", "data"),
				newRawVolumeDeviceContainerDoc("app-1", "data"),
			},
			expectedErrs: []string{
				`container "app-2": devices[0]: raw volume "data" is already handed to container "app-1"`,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := validateRawVolumeDeviceDocs(t, test.docs...)

			if len(test.expectedErrs) == 0 {
				if err != nil {
					// The bare volume documents fail their own validation, which is not what this
					// test is about.
					assert.NotContains(t, err.Error(), "devices[")
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
