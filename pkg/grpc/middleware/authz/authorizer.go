// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package authz

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/machinery/role"
)

// ErrNotAuthorized should be returned to the client when they are not authorized.
var ErrNotAuthorized = status.Error(codes.PermissionDenied, "not authorized")

// Authorizer checks that the user is authorized (has a valid role) to call intercepted gRPC method.
// User roles should be set the Injector interceptor.
type Authorizer struct {
	// Maps full gRPC method names to roles. The user should have at least one of them.
	Rules map[string]role.Set

	// Defines roles for gRPC methods not present in Rules.
	FallbackRoles role.Set

	// Logger.
	Logger func(format string, v ...any)
}

func (a *Authorizer) logf(format string, v ...any) {
	if a.Logger != nil {
		a.Logger(format, v...)
	}
}

// authorize returns error if the user is not authorized (doesn't have a valid role) to call the given gRPC method.
// User roles should be previously set the Injector interceptor.
func (a *Authorizer) authorize(ctx context.Context, method string) error {
	allowedRoles, found := a.Rules[method]
	if !found {
		a.logf("no explicit rule found for %q, falling back to %v", method, a.FallbackRoles.Strings())
		allowedRoles = a.FallbackRoles
	}

	clientRoles := GetRoles(ctx)
	if allowedRoles.IncludesAny(clientRoles) {
		a.logf("authorized (%v includes %v)", allowedRoles.Strings(), clientRoles.Strings())

		return nil
	}

	a.logf("not authorized (%v doesn't include %v)", allowedRoles.Strings(), clientRoles.Strings())

	return ErrNotAuthorized
}

// UnaryInterceptor returns grpc UnaryServerInterceptor.
func (a *Authorizer) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := a.authorize(ctx, info.FullMethod); err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}

// StreamInterceptor returns grpc StreamServerInterceptor.
func (a *Authorizer) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := a.authorize(stream.Context(), info.FullMethod); err != nil {
			return err
		}

		return handler(srv, stream)
	}
}
