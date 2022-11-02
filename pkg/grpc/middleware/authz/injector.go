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

	"github.com/siderolabs/talos/pkg/machinery/role"
)

// InjectorMode specifies how roles are extracted.
type InjectorMode int

const (
	// Disabled is used when RBAC is disabled in the machine configuration. All roles are assumed.
	Disabled InjectorMode = iota

	// ReadOnly is used to inject only Reader role.
	ReadOnly

	// MetadataOnly is used internally. Checks only metadata.
	MetadataOnly

	// Enabled is used when RBAC is enabled in the machine configuration. Roles are extracted normally.
	Enabled
)

// Injector sets roles to the context.
type Injector struct {
	// Mode.
	Mode InjectorMode

	// Logger.
	Logger func(format string, v ...interface{})
}

func (i *Injector) logf(format string, v ...interface{}) {
	if i.Logger != nil {
		i.Logger(format, v...)
	}
}

// extractRoles returns roles extracted from the user's certificate (in case of the first apid instance),
// or from gRPC metadata (in case of subsequent apid instances, machined, or user with impersonator role).
//
//nolint:gocyclo
func (i *Injector) extractRoles(ctx context.Context) role.Set {
	// sanity check
	if _, ok := getFromContext(ctx); ok {
		panic("roles should not be present in the context at this point")
	}

	switch i.Mode {
	case Disabled:
		i.logf("RBAC is disabled, injecting all roles")

		return role.All

	case ReadOnly:
		return role.MakeSet(role.Reader)

	case MetadataOnly:
		roles, _ := getFromMetadata(ctx, i.logf)

		return roles

	case Enabled:
		p, ok := peer.FromContext(ctx)
		if !ok {
			panic("can't get peer information")
		}

		tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
		if !ok {
			panic(fmt.Sprintf("expected credentials.TLSInfo, got %T", p.AuthInfo))
		}

		if len(tlsInfo.State.PeerCertificates) == 0 {
			panic("expected at least one certificate")
		}

		// PeerCertificates[0] is the leaf certificate the connection was verified against, so this
		// is the client cert. Other certificates in the chain might be CAs or intermediates.
		strings := tlsInfo.State.PeerCertificates[0].Subject.Organization

		// TODO validate cert.KeyUsage, cert.ExtKeyUsage, cert.Issuer.Organization, other fields there?

		roles, unknownRoles := role.Parse(strings)
		i.logf("parsed peer's certificate orgs %v as %v (unknownRoles = %v)", strings, roles.Strings(), unknownRoles)

		// trust gRPC metadata from clients with impersonator role if present
		// (including requests proxied from other apid instances)
		if roles.Includes(role.Impersonator) {
			metadataRoles, ok := getFromMetadata(ctx, i.logf)
			if ok {
				return metadataRoles
			}

			// that's a real user with impersonator role then
			i.logf("no roles in metadadata, returning parsed roles")
		}

		return roles
	}

	panic("unreachable")
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
