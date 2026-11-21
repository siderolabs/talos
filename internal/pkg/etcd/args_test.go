// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/etcd"
	etcdresource "github.com/siderolabs/talos/pkg/machinery/resources/etcd"
)

func TestBuildArgs(t *testing.T) {
	t.Parallel()

	spec := func(extraArgs map[string]etcdresource.ArgValues) *etcdresource.SpecSpec {
		return &etcdresource.SpecSpec{
			Name:                  "node1",
			AdvertisedAddresses:   []netip.Addr{netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("fd00::1")},
			ListenPeerAddresses:   []netip.Addr{netip.MustParseAddr("10.0.0.1")},
			ListenClientAddresses: []netip.Addr{netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("10.0.0.1")},
			Image:                 "etcd:v1",
			ExtraArgs:             extraArgs,
		}
	}

	for _, test := range []struct {
		name string

		spec   *etcdresource.SpecSpec
		launch etcdresource.LaunchSpecSpec

		expected      []string
		notExpected   []string
		expectedError string
	}{
		{
			name: "bootstrap",
			spec: spec(nil),
			launch: etcdresource.LaunchSpecSpec{
				InitialClusterState: etcd.InitialClusterStateNew,
				InitialCluster:      "node1=https://10.0.0.1:2380",
			},
			expected: []string{
				"--name=node1",
				"--data-dir=/var/lib/etcd",
				"--initial-cluster-state=new",
				"--initial-cluster=node1=https://10.0.0.1:2380",
				"--listen-peer-urls=https://10.0.0.1:2380",
				"--listen-client-urls=https://127.0.0.1:2379,https://10.0.0.1:2379",
				"--initial-advertise-peer-urls=https://10.0.0.1:2380,https://[fd00::1]:2380",
				"--advertise-client-urls=https://10.0.0.1:2379,https://[fd00::1]:2379",
			},
		},
		{
			name: "fixed peer URLs",
			spec: spec(nil),
			launch: etcdresource.LaunchSpecSpec{
				InitialClusterState:      etcd.InitialClusterStateNew,
				InitialCluster:           "node1=https://10.0.0.1:2380",
				InitialAdvertisePeerURLs: []string{"https://10.0.0.1:2380"},
			},
			expected: []string{
				"--initial-cluster=node1=https://10.0.0.1:2380",
				// the peer URLs match the initial cluster, even though the advertised addresses have changed since
				"--initial-advertise-peer-urls=https://10.0.0.1:2380",
				"--advertise-client-urls=https://10.0.0.1:2379,https://[fd00::1]:2379",
			},
		},
		{
			name: "existing data",
			spec: spec(nil),
			launch: etcdresource.LaunchSpecSpec{
				InitialClusterState: etcd.InitialClusterStateExisting,
			},
			expected: []string{
				"--initial-cluster-state=existing",
			},
			notExpected: []string{
				"--initial-cluster=",
			},
		},
		{
			name: "extra args",
			spec: spec(map[string]etcdresource.ArgValues{
				"election-timeout":      {Values: []string{"5000"}},
				"initial-cluster":       {Values: []string{"a=https://1.2.3.4:2380"}},
				"initial-cluster-state": {Values: []string{"new"}},
				"advertise-client-urls": {Values: []string{"https://1.2.3.4:2379"}},
			}),
			launch: etcdresource.LaunchSpecSpec{
				InitialClusterState: etcd.InitialClusterStateExisting,
				InitialCluster:      "node1=https://10.0.0.1:2380",
			},
			expected: []string{
				"--election-timeout=5000",
				"--initial-cluster=a=https://1.2.3.4:2380",
				"--initial-cluster-state=new",
				"--advertise-client-urls=https://1.2.3.4:2379",
			},
			notExpected: []string{
				"--initial-cluster-state=existing",
				"--initial-cluster=node1=https://10.0.0.1:2380",
			},
		},
		{
			name: "denied extra args",
			spec: spec(map[string]etcdresource.ArgValues{
				"data-dir": {Values: []string{"/tmp"}},
			}),
			launch: etcdresource.LaunchSpecSpec{
				InitialClusterState: etcd.InitialClusterStateExisting,
			},
			expectedError: "data-dir",
		},
		{
			name: "protected args",
			spec: spec(map[string]etcdresource.ArgValues{
				"name": {Values: []string{"other"}},
			}),
			launch: etcdresource.LaunchSpecSpec{
				InitialClusterState: etcd.InitialClusterStateExisting,
			},
			expectedError: "name",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			args, err := etcd.BuildArgs(test.spec, &test.launch)
			if test.expectedError != "" {
				require.ErrorContains(t, err, test.expectedError)

				return
			}

			require.NoError(t, err)

			for _, expected := range test.expected {
				assert.Contains(t, args, expected)
			}

			for _, notExpected := range test.notExpected {
				for _, arg := range args {
					assert.NotContains(t, arg, notExpected)
				}
			}
		})
	}
}

func TestRestartArgs(t *testing.T) {
	t.Parallel()

	args := []string{
		"--name=node1",
		"--initial-advertise-peer-urls=https://10.0.0.1:2380",
		"--advertise-client-urls=https://10.0.0.1:2379",
	}

	// the advertised URLs which follow the machine addresses are not a reason to restart etcd
	assert.Equal(t, []string{"--name=node1"}, etcd.RestartArgs(&etcdresource.SpecSpec{}, args))

	// the advertised URLs set in the machine configuration are
	assert.Equal(t,
		[]string{"--name=node1", "--advertise-client-urls=https://10.0.0.1:2379"},
		etcd.RestartArgs(&etcdresource.SpecSpec{
			ExtraArgs: map[string]etcdresource.ArgValues{
				"advertise-client-urls": {Values: []string{"https://10.0.0.1:2379"}},
			},
		}, args),
	)
}
