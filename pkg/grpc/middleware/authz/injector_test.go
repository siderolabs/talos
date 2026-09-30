// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package authz_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// withPeerCert returns a context with gRPC peer info carrying a client TLS certificate with the given organizations.
func withPeerCert(ctx context.Context, orgs ...string) context.Context {
	return peer.NewContext(ctx, &peer.Peer{
		Addr: &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 12345},
		AuthInfo: credentials.TLSInfo{
			State: tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{
					{
						Subject: pkix.Name{
							Organization: orgs,
						},
					},
				},
			},
		},
	})
}

// withRoleMetadata returns a context with the impersonation header set in the incoming gRPC metadata.
func withRoleMetadata(ctx context.Context, roles ...string) context.Context {
	md, _ := metadata.FromIncomingContext(ctx)
	md = md.Copy()

	md.Set(constants.APIAuthzRoleMetadataKey, roles...)

	return metadata.NewIncomingContext(ctx, md)
}

// withEmptyMetadata returns a context with (empty) incoming gRPC metadata, as it would be for any real gRPC request.
func withEmptyMetadata(ctx context.Context) context.Context {
	return metadata.NewIncomingContext(ctx, metadata.MD{})
}

type fakeServerStream struct {
	grpc.ServerStream

	ctx context.Context //nolint:containedctx
}

func (s *fakeServerStream) Context() context.Context {
	return s.ctx
}

//nolint:gocyclo
func TestInjector(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		injector authz.Injector
		ctx      context.Context //nolint:containedctx

		expectedRoles role.Set
		expectedCode  codes.Code
		expectedError string
	}{
		{
			name:          "disabled",
			injector:      authz.Injector{Mode: authz.Disabled},
			ctx:           withEmptyMetadata(context.Background()),
			expectedRoles: role.All,
		},
		{
			name:     "disabled ignores impersonation header",
			injector: authz.Injector{Mode: authz.Disabled},
			ctx:      withRoleMetadata(context.Background(), "os:reader"),
			// RBAC is off, so the header is meaningless: every role is granted anyway
			expectedRoles: role.All,
		},
		{
			name:          "read-only",
			injector:      authz.Injector{Mode: authz.ReadOnly},
			ctx:           withRoleMetadata(context.Background(), "os:admin"),
			expectedRoles: role.MakeSet(role.Reader),
		},
		{
			name: "read-only with admin on SideroLink: not a SideroLink peer",
			injector: authz.Injector{
				Mode: authz.ReadOnlyWithAdminOnSiderolink,
				SideroLinkPeerCheckFunc: func(context.Context) (netip.Addr, bool) {
					return netip.Addr{}, false
				},
			},
			ctx:           withRoleMetadata(context.Background(), "os:admin"),
			expectedRoles: role.MakeSet(role.Reader),
		},
		{
			name: "read-only with admin on SideroLink: SideroLink peer",
			injector: authz.Injector{
				Mode: authz.ReadOnlyWithAdminOnSiderolink,
				SideroLinkPeerCheckFunc: func(context.Context) (netip.Addr, bool) {
					return netip.MustParseAddr("fdae:41e4:649b:9303::1"), true
				},
			},
			ctx:           withEmptyMetadata(context.Background()),
			expectedRoles: role.MakeSet(role.Admin),
		},
		{
			name:          "metadata only",
			injector:      authz.Injector{Mode: authz.MetadataOnly},
			ctx:           withRoleMetadata(context.Background(), "os:operator", "os:reader"),
			expectedRoles: role.MakeSet(role.Operator, role.Reader),
		},
		{
			name:          "metadata only: no roles in metadata",
			injector:      authz.Injector{Mode: authz.MetadataOnly},
			ctx:           withEmptyMetadata(context.Background()),
			expectedRoles: role.Zero,
		},
		{
			name:          "metadata only: no metadata",
			injector:      authz.Injector{Mode: authz.MetadataOnly},
			ctx:           context.Background(),
			expectedCode:  codes.Unknown,
			expectedError: "no request metadata",
		},
		{
			name:          "enabled: roles from certificate",
			injector:      authz.Injector{Mode: authz.Enabled},
			ctx:           withPeerCert(withEmptyMetadata(context.Background()), "os:reader", "os:operator"),
			expectedRoles: role.MakeSet(role.Reader, role.Operator),
		},
		{
			name:          "enabled: impersonator without header keeps its own roles",
			injector:      authz.Injector{Mode: authz.Enabled},
			ctx:           withPeerCert(withEmptyMetadata(context.Background()), "os:impersonator", "os:reader"),
			expectedRoles: role.MakeSet(role.Impersonator, role.Reader),
		},
		{
			name:          "enabled: impersonator with header takes roles from header",
			injector:      authz.Injector{Mode: authz.Enabled},
			ctx:           withPeerCert(withRoleMetadata(context.Background(), "os:admin"), "os:impersonator"),
			expectedRoles: role.MakeSet(role.Admin),
		},
		{
			name:          "enabled: impersonator with header is not merged with own roles",
			injector:      authz.Injector{Mode: authz.Enabled},
			ctx:           withPeerCert(withRoleMetadata(context.Background(), "os:reader"), "os:impersonator", "os:admin"),
			expectedRoles: role.MakeSet(role.Reader),
		},
		{
			name:          "enabled: non-impersonator with header is rejected",
			injector:      authz.Injector{Mode: authz.Enabled},
			ctx:           withPeerCert(withRoleMetadata(context.Background(), "os:admin"), "os:reader"),
			expectedCode:  codes.PermissionDenied,
			expectedError: "client doesn't have impersonator role, but impersonation header is present",
		},
		{
			name:     "enabled: admin with header is rejected",
			injector: authz.Injector{Mode: authz.Enabled},
			ctx:      withPeerCert(withRoleMetadata(context.Background(), "os:reader"), "os:admin"),
			// even a "downgrade" is rejected: the header is only honored for impersonators
			expectedCode:  codes.PermissionDenied,
			expectedError: "client doesn't have impersonator role, but impersonation header is present",
		},
		{
			name:          "enabled: no peer",
			injector:      authz.Injector{Mode: authz.Enabled},
			ctx:           withEmptyMetadata(context.Background()),
			expectedCode:  codes.Unknown,
			expectedError: "can't get peer information",
		},
		{
			name:     "enabled: no TLS info",
			injector: authz.Injector{Mode: authz.Enabled},
			ctx: peer.NewContext(withEmptyMetadata(context.Background()), &peer.Peer{
				Addr: &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 12345},
			}),
			expectedCode:  codes.Unknown,
			expectedError: "expected credentials.TLSInfo, got <nil>",
		},
		{
			name:     "enabled: no peer certificates",
			injector: authz.Injector{Mode: authz.Enabled},
			ctx: peer.NewContext(withEmptyMetadata(context.Background()), &peer.Peer{
				Addr:     &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 12345},
				AuthInfo: credentials.TLSInfo{},
			}),
			expectedCode:  codes.Unknown,
			expectedError: "expected at least one certificate",
		},
		{
			name:          "roles already in the context",
			injector:      authz.Injector{Mode: authz.Disabled},
			ctx:           authz.ContextWithRoles(withEmptyMetadata(context.Background()), role.MakeSet(role.Reader)),
			expectedCode:  codes.Unknown,
			expectedError: "roles should not be present in the context at this point",
		},
		{
			name:          "unknown mode",
			injector:      authz.Injector{Mode: authz.InjectorMode(42)},
			ctx:           withEmptyMetadata(context.Background()),
			expectedCode:  codes.Unknown,
			expectedError: "unknown injector mode 42",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			t.Run("unary", func(t *testing.T) {
				t.Parallel()

				var (
					handlerCalled bool
					injectedRoles role.Set
				)

				handler := func(ctx context.Context, req any) (any, error) {
					handlerCalled = true
					injectedRoles = authz.GetRoles(ctx)

					return "response", nil
				}

				resp, err := test.injector.UnaryInterceptor()(test.ctx, "request", &grpc.UnaryServerInfo{FullMethod: "/some.Service/Method"}, handler)

				if test.expectedError != "" {
					require.Error(t, err)

					assert.False(t, handlerCalled, "handler should not be called on error")
					assert.Nil(t, resp)
					assert.Equal(t, test.expectedCode, status.Code(err))
					assert.ErrorContains(t, err, test.expectedError)

					return
				}

				require.NoError(t, err)

				assert.True(t, handlerCalled)
				assert.Equal(t, "response", resp)
				assert.Equal(t, test.expectedRoles.Strings(), injectedRoles.Strings())
			})

			t.Run("stream", func(t *testing.T) {
				t.Parallel()

				var (
					handlerCalled bool
					injectedRoles role.Set
				)

				handler := func(srv any, stream grpc.ServerStream) error {
					handlerCalled = true
					injectedRoles = authz.GetRoles(stream.Context())

					return nil
				}

				err := test.injector.StreamInterceptor()(nil, &fakeServerStream{ctx: test.ctx}, &grpc.StreamServerInfo{FullMethod: "/some.Service/Method"}, handler)

				if test.expectedError != "" {
					require.Error(t, err)

					assert.False(t, handlerCalled, "handler should not be called on error")
					assert.Equal(t, test.expectedCode, status.Code(err))
					assert.ErrorContains(t, err, test.expectedError)

					return
				}

				require.NoError(t, err)

				assert.True(t, handlerCalled)
				assert.Equal(t, test.expectedRoles.Strings(), injectedRoles.Strings())
			})
		})
	}
}
