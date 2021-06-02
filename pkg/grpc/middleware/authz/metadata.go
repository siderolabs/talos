// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package authz

import (
	"context"

	"google.golang.org/grpc/metadata"

	"github.com/talos-systems/talos/pkg/machinery/role"
)

// mdKey is used to store roles in gRPC metadata.
// Should be used only in this file.
const mdKey = "talos-role"

// SetRolesToMetadata gets roles from the context (where they were previously set by the Injector interceptor)
// and sets them the metadata.
func SetRolesToMetadata(ctx context.Context, md metadata.MD) {
	md.Set(mdKey, GetRoles(ctx).Strings()...)
}

// getFromMetadata returns roles extracted from from gRPC metadata.
func getFromMetadata(ctx context.Context, logf func(format string, v ...interface{})) role.Set {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		panic("no request metadata")
	}

	strings := md.Get(mdKey)
	roles, err := role.Parse(strings)
	logf("parsed metadata %v as %v (err = %v)", strings, roles.Strings(), err)

	return roles
}
