// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nfs_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/nfs"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

type resolver struct {
	addrs []netip.Addr
}

func (r resolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return r.addrs, nil
}

func TestResolveMountParameters(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name          string
		source        string
		parameters    []block.ParameterSpec
		resolver      resolver
		expected      []block.ParameterSpec
		expectedError string
	}{
		{
			name:       "IPv4 literal",
			source:     "192.0.2.10:/export",
			parameters: []block.ParameterSpec{block.NewStringParameter("proto", "tcp")},
			expected: []block.ParameterSpec{
				block.NewStringParameter("proto", "tcp"),
				block.NewStringParameter("addr", "192.0.2.10"),
			},
		},
		{
			name:       "IPv6 literal",
			source:     "[2001:db8::10]:/export",
			parameters: []block.ParameterSpec{block.NewStringParameter("proto", "tcp6")},
			expected: []block.ParameterSpec{
				block.NewStringParameter("proto", "tcp6"),
				block.NewStringParameter("addr", "2001:db8::10"),
			},
		},
		{
			name:       "IPv4 transport pins the hostname to an IPv4 address",
			source:     "nfs.example.test:/export",
			parameters: []block.ParameterSpec{block.NewStringParameter("proto", "tcp")},
			resolver: resolver{addrs: []netip.Addr{
				netip.MustParseAddr("2001:db8::10"),
				netip.MustParseAddr("192.0.2.10"),
			}},
			expected: []block.ParameterSpec{
				block.NewStringParameter("proto", "tcp"),
				block.NewStringParameter("addr", "192.0.2.10"),
			},
		},
		{
			name:       "IPv6 transport pins the hostname to an IPv6 address",
			source:     "nfs.example.test:/export",
			parameters: []block.ParameterSpec{block.NewStringParameter("proto", "tcp6")},
			resolver: resolver{addrs: []netip.Addr{
				netip.MustParseAddr("192.0.2.10"),
				netip.MustParseAddr("2001:db8::10"),
			}},
			expected: []block.ParameterSpec{
				block.NewStringParameter("proto", "tcp6"),
				block.NewStringParameter("addr", "2001:db8::10"),
			},
		},
		{
			name:       "mount transport pins the address family on its own",
			source:     "nfs.example.test:/export",
			parameters: []block.ParameterSpec{block.NewStringParameter("mountproto", "tcp6")},
			resolver: resolver{addrs: []netip.Addr{
				netip.MustParseAddr("192.0.2.10"),
				netip.MustParseAddr("2001:db8::10"),
			}},
			expected: []block.ParameterSpec{
				block.NewStringParameter("mountproto", "tcp6"),
				block.NewStringParameter("proto", "tcp6"),
				block.NewStringParameter("addr", "2001:db8::10"),
			},
		},
		{
			name:       "IPv4 transport rejects an IPv6-only server",
			source:     "nfs.example.test:/export",
			parameters: []block.ParameterSpec{block.NewStringParameter("proto", "tcp")},
			resolver: resolver{addrs: []netip.Addr{
				netip.MustParseAddr("2001:db8::10"),
			}},
			expectedError: `NFS server "nfs.example.test" resolved to no IPv4 addresses`,
		},
		{
			name:          "IPv6 transport rejects an IPv4 literal",
			source:        "192.0.2.10:/export",
			parameters:    []block.ParameterSpec{block.NewStringParameter("proto", "tcp6")},
			expectedError: `NFS server "192.0.2.10" is not an IPv6 address`,
		},
		{
			name:       "unset transport is derived from the resolved address",
			source:     "nfs.example.test:/export",
			parameters: []block.ParameterSpec{block.NewStringParameter("port", "2049")},
			resolver: resolver{addrs: []netip.Addr{
				netip.MustParseAddr("192.0.2.10"),
			}},
			expected: []block.ParameterSpec{
				block.NewStringParameter("port", "2049"),
				block.NewStringParameter("proto", "tcp"),
				block.NewStringParameter("addr", "192.0.2.10"),
			},
		},
		{
			name:       "unset transport is derived from the first resolved address (IPv4)",
			source:     "nfs.example.test:/export",
			parameters: []block.ParameterSpec{block.NewStringParameter("port", "2049")},
			resolver: resolver{addrs: []netip.Addr{
				netip.MustParseAddr("192.0.2.10"),
				netip.MustParseAddr("2001:db8::10"),
			}},
			expected: []block.ParameterSpec{
				block.NewStringParameter("port", "2049"),
				block.NewStringParameter("proto", "tcp"),
				block.NewStringParameter("addr", "192.0.2.10"),
			},
		},
		{
			name:       "unset transport is derived from the first resolved address (IPv6)",
			source:     "nfs.example.test:/export",
			parameters: []block.ParameterSpec{block.NewStringParameter("port", "2049")},
			resolver: resolver{addrs: []netip.Addr{
				netip.MustParseAddr("2001:db8::10"),
				netip.MustParseAddr("192.0.2.10"),
			}},
			expected: []block.ParameterSpec{
				block.NewStringParameter("port", "2049"),
				block.NewStringParameter("proto", "tcp6"),
				block.NewStringParameter("addr", "2001:db8::10"),
			},
		},
		{
			name:       "unset transport is derived as IPv6 for an IPv6 literal",
			source:     "[2001:db8::10]:/export",
			parameters: nil,
			expected: []block.ParameterSpec{
				block.NewStringParameter("proto", "tcp6"),
				block.NewStringParameter("addr", "2001:db8::10"),
			},
		},
		{
			name:       "hostname-valued address parameter is resolved",
			source:     "nfs.example.test:/export",
			parameters: []block.ParameterSpec{block.NewStringParameter("addr", "backend.example.test")},
			resolver: resolver{addrs: []netip.Addr{
				netip.MustParseAddr("192.0.2.20"),
			}},
			expected: []block.ParameterSpec{
				block.NewStringParameter("proto", "tcp"),
				block.NewStringParameter("addr", "192.0.2.20"),
			},
		},
		{
			name:   "duplicate address parameters use the last value",
			source: "nfs.example.test:/export",
			parameters: []block.ParameterSpec{
				block.NewStringParameter("proto", "tcp"),
				block.NewStringParameter("addr", "192.0.2.10"),
				block.NewStringParameter("addr", "192.0.2.20"),
			},
			expected: []block.ParameterSpec{
				block.NewStringParameter("proto", "tcp"),
				block.NewStringParameter("addr", "192.0.2.20"),
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual, err := nfs.ResolveMountParameters(t.Context(), test.source, test.parameters, test.resolver)

			if test.expectedError != "" {
				require.EqualError(t, err, test.expectedError)

				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expected, actual)
		})
	}
}
