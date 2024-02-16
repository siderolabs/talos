// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package authz

import (
	"context"

	"github.com/siderolabs/talos/pkg/machinery/role"
)

// ctxKey is used to store parsed roles in the context.
// Should be used only in this file.
type ctxKey struct{}

// GetRoles returns roles stored in the context by the Injector interceptor.
// May be used for additional checks in the API method handler.
func GetRoles(ctx context.Context) role.Set {
	set, ok := getFromContext(ctx)

	if !ok {
		panic("no roles in the context")
	}

	return set
}

// HasRole returns true if the context includes the given role.
func HasRole(ctx context.Context, r role.Role) bool {
	return GetRoles(ctx).Includes(r)
}

// getFromContext returns roles stored in the context.
func getFromContext(ctx context.Context) (role.Set, bool) {
	set, ok := ctx.Value(ctxKey{}).(role.Set)

	return set, ok
}

// ContextWithRoles returns derived context with roles set.
func ContextWithRoles(ctx context.Context, roles role.Set) context.Context {
	// sanity check
	if ctx.Value(ctxKey{}) != nil {
		panic("roles already stored in the context")
	}

	return context.WithValue(ctx, ctxKey{}, roles)
}
