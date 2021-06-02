// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package authz

import (
	"context"
	"strings"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/talos-systems/talos/pkg/machinery/role"
)

// Authorizer checks that the user is authorized (has a valid role) to call intercepted gRPC method.
// User roles should be set the Injector interceptor.
type Authorizer struct {
	// Maps full gRPC method names to roles. The user should have at least one of them.
	Rules map[string]role.Set

	// Defines roles for gRPC methods not present in Rules.
	FallbackRoles role.Set

	// If true, makes the authorizer never return authorization error.
	DontEnforce bool

	// Logger.
	Logger func(format string, v ...interface{})
}

// nextPrefix returns path's prefix, stopping on slashes and dots:
// /machine.MachineService/List -> /machine.MachineService -> /machine -> / -> / -> ...
// The chain ends with "/" no matter what.
func nextPrefix(path string) string {
	if path == "" || path[0] != '/' {
		return "/"
	}

	i := strings.LastIndexAny(path, "/.")
	if i <= 0 {
		return "/"
	}

	return path[:i]
}

func (a *Authorizer) logf(format string, v ...interface{}) {
	if a.Logger != nil {
		a.Logger(format, v...)
	}
}

// authorize returns error if the user is not authorized (doesn't have a valid role) to call the given gRPC method.
// User roles should be previously set the Injector interceptor.
func (a *Authorizer) authorize(ctx context.Context, method string) (context.Context, error) {
	clientRoles := GetRoles(ctx)

	var allowedRoles role.Set

	prefix := method
	for prefix != "/" {
		if allowedRoles = a.Rules[prefix]; allowedRoles != nil {
			break
		}

		prefix = nextPrefix(prefix)
	}

	if allowedRoles == nil {
		a.logf("no explicit rule found for %q, falling back to %v", method, a.FallbackRoles.Strings())
		allowedRoles = a.FallbackRoles
	}

	if allowedRoles.IncludesAny(clientRoles) {
		a.logf("authorized (%v includes %v)", allowedRoles.Strings(), clientRoles.Strings())

		return ctx, nil
	}

	if a.DontEnforce {
		a.logf("not authorized (%v doesn't include %v), but authorization wasn't enforced", allowedRoles.Strings(), clientRoles.Strings())

		return ctx, nil
	}

	a.logf("not authorized (%v doesn't include %v)", allowedRoles.Strings(), clientRoles.Strings())

	return nil, status.Error(codes.PermissionDenied, "not authorized")
}

// UnaryInterceptor returns grpc UnaryServerInterceptor.
func (a *Authorizer) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ctx, err := a.authorize(ctx, info.FullMethod)
		if err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}

// StreamInterceptor returns grpc StreamServerInterceptor.
func (a *Authorizer) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx, err := a.authorize(stream.Context(), info.FullMethod)
		if err != nil {
			return err
		}

		wrapped := grpc_middleware.WrapServerStream(stream)
		wrapped.WrappedContext = ctx

		return handler(srv, wrapped)
	}
}
