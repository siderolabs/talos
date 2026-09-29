// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package log_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	metadata "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"

	"github.com/siderolabs/talos/pkg/grpc/middleware/log"
)

func TestExtractMetadata(t *testing.T) {
	for _, test := range []struct {
		name     string
		md       metadata.MD
		peer     *peer.Peer
		noMD     bool
		expected string
	}{
		{
			name:     "empty",
			md:       metadata.MD{},
			expected: "",
		},
		{
			name:     "no metadata",
			noMD:     true,
			expected: "",
		},
		{
			name:     "no metadata with peer",
			noMD:     true,
			peer:     &peer.Peer{Addr: &net.TCPAddr{IP: net.IPv4(10, 5, 0, 2), Port: 34567}},
			expected: "peer=10.5.0.2:34567",
		},
		{
			name:     "peer without address",
			md:       metadata.Pairs("foo", "bar"),
			peer:     &peer.Peer{},
			expected: "foo=bar",
		},
		{
			name:     "peer address overrides client-supplied metadata",
			md:       metadata.Pairs("foo", "bar", "peer", "spoofed"),
			peer:     &peer.Peer{Addr: &net.TCPAddr{IP: net.IPv4(10, 5, 0, 2), Port: 34567}},
			expected: "foo=bar;peer=10.5.0.2:34567",
		},
		{
			name:     "client-supplied peer metadata is dropped",
			md:       metadata.Pairs("foo", "bar", "peer", "spoofed"),
			expected: "foo=bar",
		},
		{
			name:     "regular",
			md:       metadata.Pairs("foo", "bar", "one", "two", "a", "b"),
			expected: "a=b;foo=bar;one=two",
		},
		{
			name:     "sensitive",
			md:       metadata.Pairs("foo", "bar", "token", "secret"),
			expected: "foo=bar;token=<hidden>",
		},
	} {
		ctx := t.Context()

		if !test.noMD {
			ctx = metadata.NewIncomingContext(ctx, test.md)
		}

		if test.peer != nil {
			ctx = peer.NewContext(ctx, test.peer)
		}

		assert.Equal(t, test.expected, log.ExtractMetadata(ctx), test.name)
	}
}
