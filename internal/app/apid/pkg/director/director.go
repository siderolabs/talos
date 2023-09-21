// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package director provides proxy call routing facility
package director

import (
	"context"
	"regexp"
	"slices"
	"strings"

	"github.com/siderolabs/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Router wraps grpc-proxy StreamDirector.
type Router struct {
	localBackend         proxy.Backend
	remoteBackendFactory RemoteBackendFactory
	localAddressProvider LocalAddressProvider
	streamedMatchers     []*regexp.Regexp
}

// RemoteBackendFactory provides backend generation by address (target).
type RemoteBackendFactory func(target string) (proxy.Backend, error)

// NewRouter builds new Router.
func NewRouter(backendFactory RemoteBackendFactory, localBackend proxy.Backend, localAddressProvider LocalAddressProvider) *Router {
	return &Router{
		localBackend:         localBackend,
		remoteBackendFactory: backendFactory,
		localAddressProvider: localAddressProvider,
	}
}

// Register is no-op to implement factory.Registrator interface.
//
// Actual proxy handler is installed via grpc.UnknownServiceHandler option.
func (r *Router) Register(srv *grpc.Server) {
}

// Director implements proxy.StreamDirector function.
//
//nolint:gocyclo
func (r *Router) Director(ctx context.Context, fullMethodName string) (proxy.Mode, []proxy.Backend, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return proxy.One2One, []proxy.Backend{r.localBackend}, nil
	}

	if _, exists := md["proxyfrom"]; exists {
		return proxy.One2One, []proxy.Backend{r.localBackend}, nil
	}

	nodes, okNodes := md["nodes"]
	node, okNode := md["node"]

	if okNode && len(node) != 1 {
		return proxy.One2One, nil, status.Error(codes.InvalidArgument, "node metadata must be single-valued")
	}

	// special handling for cases when a single node is requested, but forwarding is disabled
	//
	// if there's a single destination, and that destination is local node, skip forwarding and send a request to the same node
	if r.remoteBackendFactory == nil {
		if okNode && r.localAddressProvider.IsLocalTarget(node[0]) {
			okNode = false
		}

		if okNodes && len(nodes) == 1 && r.localAddressProvider.IsLocalTarget(nodes[0]) {
			okNodes = false
		}
	}

	switch {
	case okNodes:
		// COSI methods do not support one-2-many proxying.
		if strings.HasPrefix(fullMethodName, "/cosi.") {
			return proxy.One2One, nil, status.Error(codes.InvalidArgument, "one-2-many proxying is not supported for COSI methods")
		}

		return r.aggregateDirector(nodes)
	case okNode:
		return r.singleDirector(node[0])
	default:
		// send directly to local node, skips another layer of proxying
		return proxy.One2One, []proxy.Backend{r.localBackend}, nil
	}
}

// singleDirector sends request to a single instance in one-2-one mode.
func (r *Router) singleDirector(target string) (proxy.Mode, []proxy.Backend, error) {
	if r.remoteBackendFactory == nil {
		return proxy.One2One, nil, status.Error(codes.PermissionDenied, "no request forwarding")
	}

	backend, err := r.remoteBackendFactory(target)
	if err != nil {
		return proxy.One2One, nil, status.Error(codes.Internal, err.Error())
	}

	return proxy.One2One, []proxy.Backend{backend}, nil
}

// aggregateDirector sends request across set of remote instances and aggregates results.
func (r *Router) aggregateDirector(targets []string) (proxy.Mode, []proxy.Backend, error) {
	if r.remoteBackendFactory == nil {
		return proxy.One2One, nil, status.Error(codes.PermissionDenied, "no request forwarding")
	}

	var err error

	backends := make([]proxy.Backend, len(targets))

	for i, target := range targets {
		backends[i], err = r.remoteBackendFactory(target)
		if err != nil {
			return proxy.One2Many, nil, status.Error(codes.Internal, err.Error())
		}
	}

	return proxy.One2Many, backends, nil
}

// StreamedDetector implements proxy.StreamedDetector.
func (r *Router) StreamedDetector(fullMethodName string) bool {
	return slices.ContainsFunc(r.streamedMatchers, func(regex *regexp.Regexp) bool { return regex.MatchString(fullMethodName) })
}

// RegisterStreamedRegex register regex for streamed method.
//
// This could be exact literal match: /^\/serviceName\/methodName$/ or any
// suffix/prefix match.
func (r *Router) RegisterStreamedRegex(regex string) {
	r.streamedMatchers = append(r.streamedMatchers, regexp.MustCompile(regex))
}
