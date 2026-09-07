// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provision_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/provision"
)

func TestHTTPProbeRequestNormalize(t *testing.T) {
	request := provision.HTTPProbeRequest{
		IP:   netip.MustParseAddr("203.0.113.100"),
		Port: 80,
		Path: "/",
	}

	normalized, err := request.Normalize()
	require.NoError(t, err)
	require.Equal(t, provision.HTTPProbeDefaultTimeout, normalized.Timeout)

	request.Timeout = time.Millisecond
	normalized, err = request.Normalize()
	require.NoError(t, err)
	require.Equal(t, provision.HTTPProbeMinTimeout, normalized.Timeout)

	request.Timeout = time.Minute
	normalized, err = request.Normalize()
	require.NoError(t, err)
	require.Equal(t, provision.HTTPProbeMaxTimeout, normalized.Timeout)
}

func TestClusterRequestInstallDiskPath(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		disks    []*provision.Disk
		expected string
	}{
		{
			name:     "no disks",
			expected: "/dev/vda",
		},
		{
			name:     "virtio",
			disks:    []*provision.Disk{{Driver: "virtio"}, {Driver: "usb"}},
			expected: "/dev/vda",
		},
		{
			name:     "usb",
			disks:    []*provision.Disk{{Driver: "usb"}, {Driver: "virtio"}},
			expected: "/dev/sda",
		},
		{
			name:     "nvme",
			disks:    []*provision.Disk{{Driver: "nvme"}},
			expected: "/dev/nvme0n1",
		},
		{
			name:     "unknown driver",
			disks:    []*provision.Disk{{Driver: "virtiofs"}},
			expected: "/dev/vda",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			req := provision.ClusterRequest{
				Nodes: provision.NodeRequests{{Disks: test.disks}},
			}

			assert.Equal(t, test.expected, req.InstallDiskPath())
		})
	}
}
