// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resolver

import (
	"math/rand/v2"
	"net"
	"strconv"
	"strings"

	"github.com/siderolabs/gen/xslices"
	"google.golang.org/grpc/resolver"
)

// RoundRobinResolverScheme is a scheme to use in grpc.Dial for the round-robin gRPC resolver.
// This resolver requires that all endpoints have a port appended.
// To ensure this, use EnsureEndpointsHavePorts before constructing a connection string.
const RoundRobinResolverScheme = "talosroundrobin"

func init() {
	resolver.Register(&roundRobinResolverBuilder{
		scheme: RoundRobinResolverScheme,
	})
}

type roundRobinResolverBuilder struct {
	scheme string
}

// Build implements resolver.Builder.
func (b *roundRobinResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r := &roundRobinResolver{
		target: target,
		cc:     cc,
	}

	if err := r.start(); err != nil {
		return nil, err
	}

	return r, nil
}

// Build implements resolver.Builder.
func (b *roundRobinResolverBuilder) Scheme() string {
	return b.scheme
}

type roundRobinResolver struct {
	target resolver.Target
	cc     resolver.ClientConn
}

// EnsureEndpointsHavePorts returns the list of endpoints with default port appended to those addresses that don't have a port.
func EnsureEndpointsHavePorts(endpoints []string, defaultPort int) []string {
	return xslices.Map(endpoints, func(endpoint string) string {
		_, _, err := net.SplitHostPort(endpoint)
		if err != nil {
			return net.JoinHostPort(endpoint, strconv.Itoa(defaultPort))
		}

		return endpoint
	})
}

func (r *roundRobinResolver) start() error {
	var addrs []resolver.Address //nolint:prealloc

	endpoints := strings.Split(r.target.Endpoint(), ",")

	for _, addr := range endpoints {
		serverName := addr

		host, _, err := net.SplitHostPort(serverName)
		if err == nil {
			serverName = host
		}

		addrs = append(addrs, resolver.Address{
			ServerName: serverName,
			Addr:       addr,
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

	return r.cc.UpdateState(resolver.State{
		Addresses:     addrs,
		ServiceConfig: parsedServiceConfig,
	})
}

// ResolveNow implements resolver.Resolver.
func (r *roundRobinResolver) ResolveNow(o resolver.ResolveNowOptions) {}

// ResolveNow implements resolver.Resolver.
func (r *roundRobinResolver) Close() {}
