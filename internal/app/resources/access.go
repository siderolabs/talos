// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package resources contains shared implementation of COSI resource API.
package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// AccessPolicy defines the access policy for resources accessed via the API.
func AccessPolicy(st state.State) state.FilteringRule {
	return func(ctx context.Context, access state.Access) error {
		if !access.Verb.Readonly() {
			return status.Error(codes.PermissionDenied, "write access is not allowed")
		}

		rd, err := safe.StateGet[*meta.ResourceDefinition](ctx, st, resource.NewMetadata(meta.NamespaceName, meta.ResourceDefinitionType, strings.ToLower(access.ResourceType), resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				return status.Error(codes.PermissionDenied, fmt.Sprintf("resource type %q is not supported", access.ResourceType))
			}

			return err
		}

		roles := authz.GetRoles(ctx)
		spec := rd.TypedSpec()

		switch spec.Sensitivity {
		case meta.Sensitive:
			if !roles.Includes(role.Admin) {
				return authz.ErrNotAuthorized
			}
		case meta.NonSensitive:
			// nothing
		default:
			return fmt.Errorf("unexpected sensitivity %q", spec.Sensitivity)
		}

		_, err = safe.StateGet[*meta.Namespace](ctx, st, resource.NewMetadata(meta.NamespaceName, meta.NamespaceType, access.ResourceNamespace, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				return status.Error(codes.PermissionDenied, fmt.Sprintf("namespace %q is not supported", access.ResourceNamespace))
			}

			return err
		}

		return nil
	}
}
