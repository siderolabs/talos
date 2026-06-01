// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package authz

import (
	"context"

	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// mdKey is used to store roles in gRPC metadata.
// Should be used only in this file.
const mdKey = constants.APIAuthzRoleMetadataKey

// SetMetadata sets given roles in gRPC metadata.
func SetMetadata(md metadata.MD, roles role.Set) {
	md.Set(mdKey, roles.Strings()...)
}

// getFromMetadata returns roles extracted from gRPC metadata.
func getFromMetadata(ctx context.Context, logf func(format string, v ...any)) (role.Set, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		panic("no request metadata")
	}

	strings := md.Get(mdKey)
	if len(strings) == 0 {
		if logf != nil {
			logf("no roles in metadata")
		}

		return role.Zero, false
	}

	roles, unknownRoles := role.Parse(strings)
	if logf != nil {
		logf("parsed metadata %v as %v (unknownRoles = %v)", strings, roles.Strings(), unknownRoles)
	}

	return roles, true
}
