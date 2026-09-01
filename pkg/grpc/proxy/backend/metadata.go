// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package backend

import (
	"context"
	"slices"

	"github.com/siderolabs/gen/xslices"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
)

// forwardedMetadataKeys is the allowlist of gRPC metadata keys copied from the incoming
// request to the request proxied to a backend.
//
// Anything not listed here is dropped.
//
// Adding a key here hands the caller control over it, so only add keys which are purely
// informational.
//
// Keys must be canonical (lowercase) gRPC metadata keys.
var forwardedMetadataKeys = xslices.ToSet([]string{
	"context", // set by the Talos client to the client configuration context name, logged by the backend
	"runtime", // set by the Talos client, logged by the backend
})

// OutgoingMetadata builds the metadata for a request proxied to a backend.
//
// It contains the allowlisted keys of the incoming request metadata, plus the roles as
// resolved by this proxy instance for this request.
func OutgoingMetadata(ctx context.Context) metadata.MD {
	incomingMD, _ := metadata.FromIncomingContext(ctx)

	md := metadata.MD{}

	for key, values := range incomingMD {
		if _, ok := forwardedMetadataKeys[key]; ok {
			md[key] = slices.Clone(values)
		}
	}

	authz.SetMetadata(md, authz.GetRoles(ctx))

	return md
}
