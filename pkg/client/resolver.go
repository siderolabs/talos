// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"fmt"
	"strings"

	"google.golang.org/grpc/resolver"

	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/net"
)

func init() {
	resolver.Register(&talosListResolverBuilder{})
}

const talosListResolverScheme = "taloslist"

type talosListResolverBuilder struct{}

// Build implements resolver.Builder.
func (b *talosListResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r := &talosListResolver{
		target: target,
		cc:     cc,
	}
	r.start()

	return r, nil
}

// Build implements resolver.Builder.
func (b *talosListResolverBuilder) Scheme() string {
	return talosListResolverScheme
}

type talosListResolver struct {
	target resolver.Target
	cc     resolver.ClientConn
}

func (r *talosListResolver) start() {
	var addrs []resolver.Address // nolint: prealloc

	for _, a := range strings.Split(r.target.Endpoint, ",") {
		addrs = append(addrs, resolver.Address{
			ServerName: a,
			Addr:       fmt.Sprintf("%s:%d", net.FormatAddress(a), constants.ApidPort),
		})
	}

	r.cc.UpdateState(resolver.State{
		Addresses: addrs,
	})
}

// ResolveNow implements resolver.Resolver.
func (r *talosListResolver) ResolveNow(o resolver.ResolveNowOptions) {}

// ResolveNow implements resolver.Resolver.
func (r *talosListResolver) Close() {}
