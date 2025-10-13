// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makers_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops/configmaker/internal/makers"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
)

func TestQemuMaker_MachineConfig(t *testing.T) {
	cOps := clusterops.GetCommon()
	qOps := clusterops.GetQemu()

	m, err := makers.NewQemu(makers.MakerOptions[clusterops.Qemu]{
		ExtraOps:    qOps,
		CommonOps:   cOps,
		Provisioner: testProvisioner{}, // use test provisioner to simplify the test case.
	})
	require.NoError(t, err)

	desiredExtraGenOps := []generate.Option{}

	assertConfigDefaultness(t, cOps, *m.Maker, desiredExtraGenOps...)
}

func TestQemuMaker_ParseDNSConfig(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		doc          string
		fileContents string
		expected     []string
	}{
		{
			doc:          "properly configured",
			fileContents: `nameserver 10.10.1.2`,
			expected:     []string{"10.10.1.2"},
		},
		{
			doc: "other options",
			fileContents: `nameserver 10.10.1.2
options edns0
search .`,
			expected: []string{"10.10.1.2"},
		},
		{
			doc: "other options first",
			fileContents: `options edns0
search .
nameserver 10.10.1.2`,
			expected: []string{"10.10.1.2"},
		},
		{
			doc: "multiple nameservers",
			fileContents: `options edns0
search .
nameserver 10.10.1.2
nameserver 10.10.1.3
`,
			expected: []string{"10.10.1.2", "10.10.1.3"},
		},
		{
			doc: ">3 nameservers",
			fileContents: `options edns0
search .
nameserver 10.10.1.2
nameserver 10.10.1.3
nameserver 10.10.1.4
nameserver 10.10.1.5
nameserver 10.10.1.6
`,
			expected: []string{
				"10.10.1.2",
				"10.10.1.3",
				"10.10.1.4",
			},
		},
		{
			doc: "no nameserver",
			fileContents: `options edns0
search .`,
			expected: nil,
		},

		{
			doc: "ipv6",
			fileContents: `search localdomain
nameserver 2001:4860:4860::8888
nameserver 2404:1a8:7f01:b::3
`,
			expected: []string{
				"2001:4860:4860::8888",
				"2404:1a8:7f01:b::3",
			},
		},
		{
			doc: "mixed ipv4/6",
			fileContents: `options edns0
search .
nameserver 10.10.1.5
nameserver 2001:4860:4860::8888`,
			expected: []string{
				"10.10.1.5",
				"2001:4860:4860::8888",
			},
		},

		{
			doc: "comments",
			fileContents: `options edns0
search .
nameserver 10.10.1.2	# a comment after a real value
#nameserver 10.10.1.3
nameserver 10.10.1.4
;nameserver 10.10.1.5
nameserver 10.10.1.6
`,
			expected: []string{
				"10.10.1.2",
				"10.10.1.4",
				"10.10.1.6",
			},
		},
		{
			doc: "excludes loopback",
			fileContents: `options edns0
search .
nameserver 127.0.0.53
nameserver 10.96.0.10
nameserver 127.0.0.1
nameserver 192.168.1.1
`,
			expected: []string{
				"10.96.0.10",
				"192.168.1.1",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.doc, func(t *testing.T) {
			t.Parallel()

			testDir := t.TempDir()
			testResolvPath := filepath.Join(testDir, "test_resolv.conf")

			f, err := os.Create(testResolvPath)
			assert.NoError(t, err, "create test file")
			defer f.Close() //nolint:errcheck,wsl_v5

			_, err = f.WriteString(tc.fileContents)
			assert.NoError(t, err, "write test file")

			actual, err := makers.ParseHostDNSConfig(testResolvPath)
			assert.NoError(t, err)

			require.EqualValues(t, tc.expected, actual)
		})
	}
}
