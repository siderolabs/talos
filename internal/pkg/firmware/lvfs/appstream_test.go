// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lvfs_test

import (
	"bytes"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/firmware/lvfs"
)

//go:embed testdata/metadata.xml
var metadataXML []byte

func TestParseComponents(t *testing.T) {
	t.Parallel()

	var components []*lvfs.Component

	require.NoError(t, lvfs.ParseComponents(bytes.NewReader(metadataXML), func(c *lvfs.Component) error {
		components = append(components, c)

		return nil
	}))

	require.Len(t, components, 2)

	nvme := components[0]

	assert.Equal(t, "com.WDC.guid3099e006.firmware", nvme.ID)
	assert.Equal(t, "PC SN520 NVMe", nvme.Name)
	assert.Equal(t, []string{"3099e006-2dd5-5285-80ec-2845d613dc53"}, nvme.GUIDs())
	assert.Equal(t, "org.nvmexpress", nvme.Protocol())

	require.Len(t, nvme.Requires.Firmwares, 1)
	assert.Equal(t, "regex", nvme.Requires.Firmwares[0].Compare)
	assert.Equal(t, "NVME:0x1C58|NVME:0x101C|NVME:0x15B7", nvme.Requires.Firmwares[0].Version)
	assert.Equal(t, "vendor-id", nvme.Requires.Firmwares[0].Value)

	release, ok := nvme.LatestRelease()
	require.True(t, ok)

	assert.Equal(t, "20200012", release.Version)
	assert.Equal(t, "high", release.Urgency)

	url, sha256 := release.Cab()
	assert.Equal(t, "https://fwupd.org/downloads/b7514665fca835af4dbeb637893d0cee4407c613-20200012_v2.cab", url)
	assert.Equal(t, "d0e6101bb95ef165a0aed6c5857c5d9f4d08f355e08ca0a17eaa3de74790c017", sha256)
	assert.Contains(t, release.Description.InnerXML, "bug fix")

	uefi := components[1]

	assert.Equal(t, "org.uefi.capsule", uefi.Protocol())

	release, ok = uefi.LatestRelease()
	require.True(t, ok)

	// no artifacts: fall back to release location and container checksum
	url, sha256 = release.Cab()
	assert.Equal(t, "https://fwupd.org/downloads/example.cab", url)
	assert.Equal(t, "2222222222222222222222222222222222222222222222222222222222222222", sha256)
}

func TestCertPool(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, lvfs.CertPool())
}
