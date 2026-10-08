// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

//nolint:containedctx // Supplies the context required by the gRPC server-stream interface.
type vncAuthStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *vncAuthStream) Context() context.Context { return s.ctx }

func TestVNCStreamAuthorization(t *testing.T) {
	authorizer := &authz.Authorizer{Rules: services.AuthorizationRules, FallbackRoles: role.All}

	for _, tt := range []struct {
		name  string
		roles role.Set
		code  codes.Code
	}{
		{"admin", role.MakeSet(role.Admin), codes.OK},
		{"operator", role.MakeSet(role.Operator), codes.PermissionDenied},
		{"reader", role.MakeSet(role.Reader), codes.PermissionDenied},
		{"none", role.Zero, codes.PermissionDenied},
	} {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			stream := &vncAuthStream{ctx: authz.ContextWithRoles(t.Context(), tt.roles)}
			info := &grpc.StreamServerInfo{
				FullMethod:     machine.HypervisorService_VNCStream_FullMethodName,
				IsClientStream: true,
				IsServerStream: true,
			}
			err := authorizer.StreamInterceptor()(nil, stream, info, func(any, grpc.ServerStream) error {
				called = true

				return nil
			})
			require.Equal(t, tt.code, status.Code(err))
			require.Equal(t, tt.code == codes.OK, called)
		})
	}
}
