// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package director provides proxy call routing facility
package director

import (
	"context"
	"fmt"
	"strings"

	"github.com/talos-systems/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Router wraps grpc-proxy StreamDirector.
type Router struct {
	localBackends map[string]proxy.Backend
}

// NewRouter builds new Router.
func NewRouter() *Router {
	return &Router{
		localBackends: map[string]proxy.Backend{},
	}
}

// Register is no-op to implement factory.Registrator interface.
//
// Actual proxy handler is installed via grpc.UnknownServiceHandler option.
func (r *Router) Register(srv *grpc.Server) {
}

// Director implements proxy.StreamDirector function.
func (r *Router) Director(ctx context.Context, fullMethodName string) (proxy.Mode, []proxy.Backend, error) {
	parts := strings.SplitN(fullMethodName, "/", 3)
	serviceName := parts[1]

	if backend, ok := r.localBackends[serviceName]; ok {
		return proxy.One2One, []proxy.Backend{backend}, nil
	}

	return proxy.One2One, nil, status.Errorf(codes.Unknown, "service %v is not defined", serviceName)
}

// RegisterLocalBackend registers local backend by service name.
func (r *Router) RegisterLocalBackend(serviceName string, backend proxy.Backend) {
	if _, exists := r.localBackends[serviceName]; exists {
		panic(fmt.Sprintf("local backend %v already registered", serviceName))
	}

	r.localBackends[serviceName] = backend
}
