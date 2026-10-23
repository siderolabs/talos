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
	"github.com/siderolabs/talos/pkg/machinery/redactable"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// AccessPolicy defines the access policy for resources accessed via the API.
//
// It decides whether the resource may be read at all, while [Redact] decides how much of the
// resource the requester gets to see. The two have to agree, so both consult the same three inputs:
// the sensitivity of the resource type, whether the type opted into redaction, and the redaction
// mode the requester asked for. See [redactable.Header] for the whole model and the decision table;
// the cases below refer back to it.
func AccessPolicy(st state.State) state.FilteringRule {
	return func(ctx context.Context, access state.Access) error {
		if !access.Verb.Readonly() {
			return status.Error(codes.PermissionDenied, "write access is not allowed")
		}

		mode, err := redactable.ModeFromIncomingContext(ctx)
		if err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}

		rd, err := safe.StateGet[*meta.ResourceDefinition](ctx, st, resource.NewMetadata(meta.NamespaceName, meta.ResourceDefinitionType, strings.ToLower(access.ResourceType), resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				return status.Error(codes.PermissionDenied, fmt.Sprintf("resource type %q is not supported", access.ResourceType))
			}

			return err
		}

		if err = checkSensitivity(rd.TypedSpec(), authz.GetRoles(ctx), mode); err != nil {
			return err
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

// checkSensitivity decides whether a resource type may be read at all, given the roles of the
// requester and the redaction mode it asked for.
//
// It is the access half of the table documented on [redactable.Header]; [Redact] implements the
// redaction half, and the two have to be read together.
func checkSensitivity(spec *meta.ResourceDefinitionSpec, roles role.Set, mode redactable.Mode) error {
	switch spec.Sensitivity {
	case meta.NonSensitive:
		// readable in full with any role in any mode: only a sensitive type can be redactable, so
		// there is nothing for [Redact] to do with it either
		return nil
	case meta.Sensitive:
		// a sensitive type is readable in the redacted form only if it was reviewed as safe to hand
		// out that way, which is what the registration records
		canRedact := redactable.IsRedactable(spec.Type)

		switch mode {
		case redactable.ModeDisabled:
			// the requester didn't opt into redaction, so nothing is redacted on the way out and a
			// sensitive resource stays admin-only, exactly as it was before redaction existed
			if !roles.Includes(role.Admin) {
				return authz.ErrNotAuthorized
			}

			return nil
		case redactable.ModeAccept:
			// the requester can handle a redacted resource, so a reviewed type is readable without
			// the admin role -- [Redact] redacts it on the way out
			if !roles.Includes(role.Admin) && !canRedact {
				return authz.ErrNotAuthorized
			}

			return nil
		case redactable.ModeEnforce:
			// the requester wants everything it reads redacted, so a type which cannot be redacted
			// is denied to it whatever roles it carries, an admin included: that denial is what
			// tells the requester to leave this type out of whatever it is collecting
			if !canRedact {
				return status.Error(codes.PermissionDenied,
					fmt.Sprintf("resource type %q is sensitive and cannot be redacted", spec.Type))
			}

			return nil
		default:
			return fmt.Errorf("unexpected redaction mode %q", mode)
		}
	default:
		return fmt.Errorf("unexpected sensitivity %q", spec.Sensitivity)
	}
}
