// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package authz

import (
	"context"

	"github.com/talos-systems/talos/pkg/machinery/role"
)

// ctxKey is used to store parsed roles in the context.
// Should be used only in this file.
type ctxKey struct{}

// GetRoles returns roles stored in the context by the Injector interceptor.
// May be used for additional checks in the API method handler.
func GetRoles(ctx context.Context) role.Set {
	roles := rolesFromContext(ctx)

	if roles == nil {
		panic("no roles in the context")
	}

	return roles
}

// rolesFromContext returns roles stored in the context, or nil.
func rolesFromContext(ctx context.Context) role.Set {
	roles, _ := ctx.Value(ctxKey{}).(role.Set) //nolint:errcheck

	return roles
}

// ContextWithRoles returns derived context with roles set.
func ContextWithRoles(ctx context.Context, roles role.Set) context.Context {
	// sanity check
	if ctx.Value(ctxKey{}) != nil {
		panic("roles already stored in the context")
	}

	return context.WithValue(ctx, ctxKey{}, roles)
}
