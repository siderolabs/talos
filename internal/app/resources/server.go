// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package resources implements resources API server.
package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	yaml "gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/pkg/grpc/middleware/authz"
	resourceapi "github.com/talos-systems/talos/pkg/machinery/api/resource"
	"github.com/talos-systems/talos/pkg/machinery/role"
)

// Server implements ResourceService API.
type Server struct {
	resourceapi.UnimplementedResourceServiceServer

	Resources state.State
}

func marshalResource(r resource.Resource) (*resourceapi.Resource, error) {
	md := &resourceapi.Metadata{
		Namespace: r.Metadata().Namespace(),
		Type:      r.Metadata().Type(),
		Id:        r.Metadata().ID(),
		Version:   r.Metadata().Version().String(),
		Phase:     r.Metadata().Phase().String(),
		Owner:     r.Metadata().Owner(),
		Created:   timestamppb.New(r.Metadata().Created()),
		Updated:   timestamppb.New(r.Metadata().Updated()),
	}

	for _, fin := range *r.Metadata().Finalizers() {
		md.Finalizers = append(md.Finalizers, fin)
	}

	spec := &resourceapi.Spec{}

	if !resource.IsTombstone(r) && r.Spec() != nil {
		var err error

		spec.Yaml, err = yaml.Marshal(r.Spec())
		if err != nil {
			return nil, err
		}
	}

	return &resourceapi.Resource{
		Metadata: md,
		Spec:     spec,
	}, nil
}

type resourceKind struct {
	Namespace resource.Namespace
	Type      resource.Type
}

//nolint:gocyclo
func (s *Server) resolveResourceKind(ctx context.Context, kind *resourceKind) (*meta.ResourceDefinition, error) {
	registeredResources, err := s.Resources.List(ctx, resource.NewMetadata(meta.NamespaceName, meta.ResourceDefinitionType, "", resource.VersionUndefined))
	if err != nil {
		return nil, err
	}

	matched := []*meta.ResourceDefinition{}

	for _, item := range registeredResources.Items {
		rd, ok := item.(*meta.ResourceDefinition)
		if !ok {
			return nil, fmt.Errorf("unexpected resource definition type")
		}

		if strings.EqualFold(rd.Metadata().ID(), kind.Type) {
			matched = append(matched, rd)

			continue
		}

		spec := rd.Spec().(meta.ResourceDefinitionSpec) //nolint:errcheck,forcetypeassert

		for _, alias := range spec.Aliases {
			if strings.EqualFold(alias, kind.Type) {
				matched = append(matched, rd)

				break
			}
		}
	}

	switch {
	case len(matched) == 1:
		kind.Type = matched[0].Spec().(meta.ResourceDefinitionSpec).Type

		if kind.Namespace == "" {
			kind.Namespace = matched[0].Spec().(meta.ResourceDefinitionSpec).DefaultNamespace
		}

		return matched[0], nil
	case len(matched) > 1:
		matchedTypes := make([]string, len(matched))

		for i := range matched {
			matchedTypes[i] = matched[i].Metadata().ID()
		}

		return nil, status.Errorf(codes.InvalidArgument, fmt.Sprintf("resource type %q is ambiguous: %v", kind.Type, matchedTypes))
	default:
		return nil, status.Error(codes.NotFound, fmt.Sprintf("resource %q is not registered", kind.Type))
	}
}

func (s *Server) checkReadAccess(ctx context.Context, kind *resourceKind, rd *meta.ResourceDefinition) error {
	roles := authz.GetRoles(ctx)
	spec := rd.Spec().(meta.ResourceDefinitionSpec) //nolint:errcheck,forcetypeassert

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

	registeredNamespaces, err := s.Resources.List(ctx, resource.NewMetadata(meta.NamespaceName, meta.NamespaceType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	for _, ns := range registeredNamespaces.Items {
		if ns.Metadata().ID() == kind.Namespace {
			return nil
		}
	}

	return status.Error(codes.NotFound, fmt.Sprintf("namespace %q is not registered", kind.Namespace))
}

// Get implements resource.ResourceServiceServer interface.
func (s *Server) Get(ctx context.Context, in *resourceapi.GetRequest) (*resourceapi.GetResponse, error) {
	kind := &resourceKind{
		Namespace: in.GetNamespace(),
		Type:      in.GetType(),
	}

	rd, err := s.resolveResourceKind(ctx, kind)
	if err != nil {
		return nil, err
	}

	if err = s.checkReadAccess(ctx, kind, rd); err != nil {
		return nil, err
	}

	r, err := s.Resources.Get(ctx, resource.NewMetadata(kind.Namespace, kind.Type, in.GetId(), resource.VersionUndefined))
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, status.Error(codes.NotFound, err.Error())
		}

		return nil, err
	}

	protoD, err := marshalResource(rd)
	if err != nil {
		return nil, err
	}

	protoR, err := marshalResource(r)
	if err != nil {
		return nil, err
	}

	return &resourceapi.GetResponse{
		Messages: []*resourceapi.Get{
			{
				Definition: protoD,
				Resource:   protoR,
			},
		},
	}, nil
}

// List implements resource.ResourceServiceServer interface.
func (s *Server) List(in *resourceapi.ListRequest, srv resourceapi.ResourceService_ListServer) error {
	kind := &resourceKind{
		Namespace: in.GetNamespace(),
		Type:      in.GetType(),
	}

	rd, err := s.resolveResourceKind(srv.Context(), kind)
	if err != nil {
		return err
	}

	if err = s.checkReadAccess(srv.Context(), kind, rd); err != nil {
		return err
	}

	list, err := s.Resources.List(srv.Context(), resource.NewMetadata(kind.Namespace, kind.Type, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	protoD, err := marshalResource(rd)
	if err != nil {
		return err
	}

	if err = srv.Send(&resourceapi.ListResponse{
		Definition: protoD,
	}); err != nil {
		return err
	}

	for _, r := range list.Items {
		protoR, err := marshalResource(r)
		if err != nil {
			return err
		}

		if err = srv.Send(&resourceapi.ListResponse{
			Resource: protoR,
		}); err != nil {
			return err
		}
	}

	return nil
}

// Watch implements resource.ResourceServiceServer interface.
//
//nolint:gocyclo
func (s *Server) Watch(in *resourceapi.WatchRequest, srv resourceapi.ResourceService_WatchServer) error {
	kind := &resourceKind{
		Namespace: in.GetNamespace(),
		Type:      in.GetType(),
	}

	rd, err := s.resolveResourceKind(srv.Context(), kind)
	if err != nil {
		return err
	}

	if err = s.checkReadAccess(srv.Context(), kind, rd); err != nil {
		return err
	}

	protoD, err := marshalResource(rd)
	if err != nil {
		return err
	}

	if err = srv.Send(&resourceapi.WatchResponse{
		Definition: protoD,
	}); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(srv.Context())
	defer cancel()

	eventCh := make(chan state.Event)

	md := resource.NewMetadata(kind.Namespace, kind.Type, in.GetId(), resource.VersionUndefined)

	if in.GetId() == "" {
		opts := []state.WatchKindOption{}

		if in.TailEvents > 0 {
			opts = append(opts, state.WithKindTailEvents(int(in.TailEvents)))
		} else {
			opts = append(opts, state.WithBootstrapContents(true))
		}

		err = s.Resources.WatchKind(ctx, md, eventCh, opts...)
	} else {
		opts := []state.WatchOption{}

		if in.TailEvents > 0 {
			opts = append(opts, state.WithTailEvents(int(in.TailEvents)))
		}

		err = s.Resources.Watch(ctx, md, eventCh, opts...)
	}

	if err != nil {
		return fmt.Errorf("error setting up watch: %w", err)
	}

	for event := range eventCh {
		protoR, err := marshalResource(event.Resource)
		if err != nil {
			return err
		}

		resp := &resourceapi.WatchResponse{
			Resource: protoR,
		}

		switch event.Type {
		case state.Created:
			resp.EventType = resourceapi.EventType_CREATED
		case state.Updated:
			resp.EventType = resourceapi.EventType_UPDATED
		case state.Destroyed:
			resp.EventType = resourceapi.EventType_DESTROYED
		}

		if err = srv.Send(resp); err != nil {
			return err
		}
	}

	return nil
}
