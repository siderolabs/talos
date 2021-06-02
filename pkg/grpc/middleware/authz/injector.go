// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package authz

import (
	"context"
	"fmt"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"

	"github.com/talos-systems/talos/pkg/machinery/role"
)

// Injector sets roles to the context.
type Injector struct {
	// If true, trust roles in gRPC metadata, do not check certificate.
	TrustMetadata bool

	// Logger.
	Logger func(format string, v ...interface{})
}

func (i *Injector) logf(format string, v ...interface{}) {
	if i.Logger != nil {
		i.Logger(format, v...)
	}
}

// extractRoles returns roles extracted from the user's certificate (in case of the first apid instance),
// or from gRPC metadata (in case of subsequent apid instances or machined).
func (i *Injector) extractRoles(ctx context.Context) role.Set {
	// sanity check
	if rolesFromContext(ctx) != nil {
		panic("roles should not be present in the context at this point")
	}

	// check certificate first, if needed
	if !i.TrustMetadata {
		p, ok := peer.FromContext(ctx)
		if !ok {
			panic("can't get peer information")
		}

		tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
		if !ok {
			panic(fmt.Sprintf("expected credentials.TLSInfo, got %T", p.AuthInfo))
		}

		if len(tlsInfo.State.PeerCertificates) != 1 {
			panic(fmt.Sprintf("expected one certificate, got %d", len(tlsInfo.State.PeerCertificates)))
		}

		strings := tlsInfo.State.PeerCertificates[0].Subject.Organization

		// TODO validate cert.KeyUsage, cert.ExtKeyUsage, cert.Issuer.Organization, other fields there?

		roles, err := role.Parse(strings)
		i.logf("parsed peer's orgs %v as %v (err = %v)", strings, roles.Strings(), err)

		// not impersonator (not proxied request), return extracted roles
		if _, ok := roles[role.Impersonator]; !ok {
			return roles
		}
	}

	// trust gRPC metadata from clients with impersonator role (that's proxied request), or if configured
	return getFromMetadata(ctx, i.logf)
}

// UnaryInterceptor returns grpc UnaryServerInterceptor.
func (i *Injector) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ctx = ContextWithRoles(ctx, i.extractRoles(ctx))

		return handler(ctx, req)
	}
}

// StreamInterceptor returns grpc StreamServerInterceptor.
func (i *Injector) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := stream.Context()
		ctx = ContextWithRoles(ctx, i.extractRoles(ctx))

		wrapped := grpc_middleware.WrapServerStream(stream)
		wrapped.WrappedContext = ctx

		return handler(srv, wrapped)
	}
}
