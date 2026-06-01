// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"context"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xslices"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ResolveResourceKind resolves potentially aliased 'resourceType' and replaces empty 'resourceNamespace' with the default namespace for the resource.
func (c *Client) ResolveResourceKind(ctx context.Context, resourceNamespace *resource.Namespace, resourceType resource.Type) (*meta.ResourceDefinition, error) {
	registeredResources, err := safe.StateListAll[*meta.ResourceDefinition](ctx, c.COSI)
	if err != nil {
		return nil, err
	}

	var matched []*meta.ResourceDefinition

	for rd := range registeredResources.All() {
		if strings.EqualFold(rd.Metadata().ID(), resourceType) {
			matched = append(matched, rd)

			continue
		}

		spec := rd.TypedSpec()

		for _, alias := range spec.AllAliases {
			if strings.EqualFold(alias, resourceType) {
				matched = append(matched, rd)

				break
			}
		}
	}

	switch {
	case len(matched) == 1:
		if *resourceNamespace == "" {
			*resourceNamespace = matched[0].TypedSpec().DefaultNamespace
		}

		return matched[0], nil
	case len(matched) > 1:
		matchedTypes := xslices.Map(matched, func(rd *meta.ResourceDefinition) string { return rd.Metadata().ID() })

		return nil, status.Errorf(codes.InvalidArgument, "resource type %q is ambiguous: %v", resourceType, matchedTypes)
	default:
		return nil, status.Errorf(codes.NotFound, "resource %q is not registered", resourceType)
	}
}
