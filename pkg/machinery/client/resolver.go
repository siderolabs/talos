// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"fmt"
	"math/rand"
	"strings"

	"google.golang.org/grpc/resolver"

	"github.com/talos-systems/net"

	"github.com/talos-systems/talos/pkg/machinery/constants"
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

	if err := r.start(); err != nil {
		return nil, err
	}

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

func (r *talosListResolver) start() error {
	var addrs []resolver.Address // nolint: prealloc

	for _, a := range strings.Split(r.target.Endpoint, ",") {
		addrs = append(addrs, resolver.Address{
			ServerName: a,
			Addr:       fmt.Sprintf("%s:%d", net.FormatAddress(a), constants.ApidPort),
		})
	}

	// shuffle the list in case client does just one request
	rand.Shuffle(len(addrs), func(i, j int) {
		addrs[i], addrs[j] = addrs[j], addrs[i]
	})

	serviceConfigJSON := `{
		"loadBalancingConfig": [{
			"round_robin": {}
		}]
	}`

	parsedServiceConfig := r.cc.ParseServiceConfig(serviceConfigJSON)

	if parsedServiceConfig.Err != nil {
		return parsedServiceConfig.Err
	}

	r.cc.UpdateState(resolver.State{
		Addresses:     addrs,
		ServiceConfig: parsedServiceConfig,
	})

	return nil
}

// ResolveNow implements resolver.Resolver.
func (r *talosListResolver) ResolveNow(o resolver.ResolveNowOptions) {}

// ResolveNow implements resolver.Resolver.
func (r *talosListResolver) Close() {}
