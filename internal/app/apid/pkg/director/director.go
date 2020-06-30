// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package director provides proxy call routing facility
package director

import (
	"context"
	"regexp"

	"github.com/talos-systems/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Router wraps grpc-proxy StreamDirector.
type Router struct {
	localBackend         proxy.Backend
	remoteBackendFactory RemoteBackendFactory
	streamedMatchers     []*regexp.Regexp
}

// RemoteBackendFactory provides backend generation by address (target).
type RemoteBackendFactory func(target string) (proxy.Backend, error)

// NewRouter builds new Router.
func NewRouter(backendFactory RemoteBackendFactory, localBackend proxy.Backend) *Router {
	return &Router{
		localBackend:         localBackend,
		remoteBackendFactory: backendFactory,
	}
}

// Register is no-op to implement factory.Registrator interface.
//
// Actual proxy handler is installed via grpc.UnknownServiceHandler option.
func (r *Router) Register(srv *grpc.Server) {
}

// Director implements proxy.StreamDirector function.
func (r *Router) Director(ctx context.Context, fullMethodName string) (proxy.Mode, []proxy.Backend, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return proxy.One2One, []proxy.Backend{r.localBackend}, nil
	}

	if _, exists := md["proxyfrom"]; exists {
		return proxy.One2One, []proxy.Backend{r.localBackend}, nil
	}

	var targets []string

	if targets, ok = md["nodes"]; !ok {
		// send directly to local node, skips another layer of proxying
		return proxy.One2One, []proxy.Backend{r.localBackend}, nil
	}

	return r.aggregateDirector(targets)
}

// aggregateDirector sends request across set of remote instances and aggregates results.
func (r *Router) aggregateDirector(targets []string) (proxy.Mode, []proxy.Backend, error) {
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
	for _, re := range r.streamedMatchers {
		if re.MatchString(fullMethodName) {
			return true
		}
	}

	return false
}

// RegisterStreamedRegex register regex for streamed method.
//
// This could be exact literal match: /^\/serviceName\/methodName$/ or any
// suffix/prefix match.
func (r *Router) RegisterStreamedRegex(regex string) {
	r.streamedMatchers = append(r.streamedMatchers, regexp.MustCompile(regex))
}
