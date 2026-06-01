// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package unix

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware/v2"
	"github.com/ryanuber/go-glob"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// ErrNotAuthorized should be returned to the client when they are not authorized.
var ErrNotAuthorized = status.Error(codes.PermissionDenied, "not authorized")

// PIDTimeout is the maximum time to wait for the runner state to be populated with the PID of the calling process.
const PIDTimeout = 5 * time.Second

// AllowedService describes a service that is allowed to connect to the gRPC server.
type AllowedService struct {
	// Pattern of service ID to match to.
	//
	// Might be literal, e.g. 'apid', or a pattern like 'ext-*'.
	Pattern string
	// AllowNamespaceMatch allows matching the service if the mount namespace matches, even if the PID doesn't match.
	//
	// This is useful to match forked processes from the original service process.
	AllowNamespaceMatch bool
	// AllowedRoles is an allowlist of roles to allow in the incoming request.
	AllowedRoles role.Set
}

// Authorizer checks that the calling process is authorized to call the gRPC service based on PID.
type Authorizer struct {
	// Resources is a link to COSI state.
	Resources state.State

	// AllowedServices is a list of globs matching service names allowed to connect.
	AllowedServices []AllowedService

	// Logger.
	Logger func(format string, v ...any)
}

func (a *Authorizer) logf(format string, v ...any) {
	if a.Logger != nil {
		a.Logger(format, v...)
	}
}

func (a *Authorizer) matchService(servicePID *runtime.ServicePID, pid int32, mountNamespace string) (role.Set, bool) {
	for _, allowedService := range a.AllowedServices {
		if ok := glob.Glob(allowedService.Pattern, servicePID.Metadata().ID()); ok {
			if servicePID.TypedSpec().PID == pid || (mountNamespace != "" && allowedService.AllowNamespaceMatch && servicePID.TypedSpec().MountNamespace == mountNamespace) {
				return allowedService.AllowedRoles, true
			}
		}
	}

	return role.Set{}, false
}

// authorize returns error if the calling process is not authorized (doesn't have a valid PID) to call the given gRPC method.
//
//nolint:gocyclo
func (a *Authorizer) authorize(ctx context.Context) (role.Set, error) {
	peerCreds, ok := GetPeerCredentials(ctx)
	if !ok {
		a.logf("no peer credentials found in context")

		return role.Set{}, ErrNotAuthorized
	}

	pid := peerCreds.PID
	mountNamespace := peerCreds.MountNamespace

	// allow up to 5 seconds for the runner state to be populated with the PID
	ctx, cancel := context.WithTimeout(ctx, PIDTimeout)
	defer cancel()

	// first, try to find a match via simple List query, fall back to Watch if no match if found
	servicePIDs, err := safe.StateListAll[*runtime.ServicePID](ctx, a.Resources)
	if err != nil {
		return role.Set{}, fmt.Errorf("failed to list service PIDs: %w", err)
	}

	for servicePID := range servicePIDs.All() {
		allowedRoles, matched := a.matchService(servicePID, pid, mountNamespace)
		if matched {
			a.logf("authorized based on PID (%d) match with service %q (allowed roles %s)", pid, servicePID.Metadata().ID(), allowedRoles.Strings())

			return allowedRoles, nil
		}
	}

	// perform a watch to wait for the PID to appear in the state, as it might not be there yet if the service is just starting
	eventCh := make(chan safe.WrappedStateEvent[*runtime.ServicePID])

	if err := safe.StateWatchKind(ctx, a.Resources, runtime.NewServicePID("").Metadata(), eventCh, state.WithBootstrapContents(true)); err != nil {
		return role.Set{}, fmt.Errorf("failed to establish watch: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			a.logf("timed out waiting for runner state to be populated with PID")

			return role.Set{}, ErrNotAuthorized
		case wrappedEvent := <-eventCh:
			switch wrappedEvent.Type() {
			case state.Created, state.Updated:
				servicePID, err := wrappedEvent.Resource()
				if err != nil {
					return role.Set{}, fmt.Errorf("failed to get resource from event: %w", err)
				}

				allowedRoles, matched := a.matchService(servicePID, pid, mountNamespace)
				if matched {
					a.logf("authorized based on PID (%d) match with service %q (allowed roles %s)", pid, servicePID.Metadata().ID(), allowedRoles.Strings())

					return allowedRoles, nil
				}
			case state.Destroyed, state.Bootstrapped, state.Noop:
				// ignore
			case state.Errored:
				return role.Set{}, fmt.Errorf("error watching runner state: %w", wrappedEvent.Error())
			}
		}
	}
}

// UnaryInterceptor returns grpc UnaryServerInterceptor.
func (a *Authorizer) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		allowedRoles, err := a.authorize(ctx)
		if err != nil {
			return nil, err
		}

		ctx = authz.ReplaceRoles(ctx, func(inRoles role.Set) role.Set {
			return inRoles.Intersect(allowedRoles)
		})

		return handler(ctx, req)
	}
}

// StreamInterceptor returns grpc StreamServerInterceptor.
func (a *Authorizer) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := stream.Context()

		allowedRoles, err := a.authorize(ctx)
		if err != nil {
			return err
		}

		ctx = authz.ReplaceRoles(ctx, func(inRoles role.Set) role.Set {
			return inRoles.Intersect(allowedRoles)
		})

		wrapped := grpc_middleware.WrapServerStream(stream)
		wrapped.WrappedContext = ctx

		stream = wrapped

		return handler(srv, stream)
	}
}
