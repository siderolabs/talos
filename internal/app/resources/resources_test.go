// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resources_test

import (
	"context"
	"slices"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/registry"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/app/resources"
	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/redactable"
	networkres "github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

const (
	privateKey   = "GMFVMbEwHfSKgxRT6JQjGnUwAg7pTh1fbCsuJnrCiXo="
	presharedKey = "1lSlUlLIhRnvJdD9wLL/rMuHUCtBHnQTA/1kDmVYEQ4="
	publicKey    = "RcRXvGgWQFqOTfSlwOKY7CGwWvVAqBGYsjMEIcJ5vXQ="
)

// allRoles is every role which reaches the COSI API.
var allRoles = []role.Role{role.Admin, role.Operator, role.Reader}

// setup builds a state wrapped the same way the machined API wraps it, along with the raw state to
// seed the resources through.
func setup(t *testing.T) (raw, wrapped state.State) {
	t.Helper()

	ctx := t.Context()

	raw = state.WrapCore(namespaced.NewState(inmem.Build))

	namespaceRegistry := registry.NewNamespaceRegistry(raw)
	resourceRegistry := registry.NewResourceRegistry(raw)

	require.NoError(t, namespaceRegistry.RegisterDefault(ctx))
	require.NoError(t, resourceRegistry.RegisterDefault(ctx))
	require.NoError(t, namespaceRegistry.Register(ctx, networkres.NamespaceName, "network resources"))
	require.NoError(t, namespaceRegistry.Register(ctx, secrets.NamespaceName, "secrets"))

	for _, r := range []meta.ResourceWithRD{
		&networkres.LinkSpec{},
		&networkres.LinkStatus{},
		&secrets.OSRoot{},
	} {
		require.NoError(t, resourceRegistry.Register(ctx, r))
	}

	wrapped = state.WrapCore(state.Filter(resources.Redact(raw), resources.AccessPolicy(raw)))

	return raw, wrapped
}

// request builds a context carrying the roles and the redaction header of an incoming API request.
func request(ctx context.Context, r role.Role, mode redactable.Mode) context.Context {
	ctx = authz.ContextWithRoles(ctx, role.MakeSet(r))

	if mode == redactable.ModeDisabled {
		// a client predating redaction sends no header at all
		return ctx
	}

	return metadata.NewIncomingContext(ctx, metadata.Pairs(redactable.Header, string(mode)))
}

func wireguardLink() *networkres.LinkSpec {
	const id = "kubespan"

	link := networkres.NewLinkSpec(networkres.NamespaceName, id)
	link.TypedSpec().Name = id
	link.TypedSpec().Logical = true
	link.TypedSpec().Kind = "wireguard"
	link.TypedSpec().Wireguard = networkres.WireguardSpec{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		ListenPort: 51820,
		Peers: []networkres.WireguardPeer{
			{
				PublicKey:    publicKey,
				PresharedKey: presharedKey,
				Endpoint:     "10.0.0.1:51820",
			},
		},
	}

	return link
}

// requireRedacted asserts that the link had its secrets replaced and is marked as such, while the
// rest of the spec came through untouched.
func requireRedacted(t *testing.T, link *networkres.LinkSpec) {
	t.Helper()

	require.Equal(t, constants.Redacted, link.TypedSpec().Wireguard.PrivateKey)
	require.Equal(t, constants.Redacted, link.TypedSpec().Wireguard.Peers[0].PresharedKey)

	require.Equal(t, publicKey, link.TypedSpec().Wireguard.PublicKey)
	require.Equal(t, 51820, link.TypedSpec().Wireguard.ListenPort)
	require.Equal(t, "kubespan", link.TypedSpec().Name)

	value, ok := link.Metadata().Annotations().Get(constants.AnnotationRedacted)
	require.True(t, ok, "a redacted resource should be annotated")
	require.Equal(t, constants.AnnotationRedactedValue, value)
}

// requireInFull asserts that the link came through with its secrets and without the annotation.
func requireInFull(t *testing.T, link *networkres.LinkSpec) {
	t.Helper()

	require.Equal(t, privateKey, link.TypedSpec().Wireguard.PrivateKey)
	require.Equal(t, presharedKey, link.TypedSpec().Wireguard.Peers[0].PresharedKey)

	_, ok := link.Metadata().Annotations().Get(constants.AnnotationRedacted)
	require.False(t, ok, "a resource served in full should not be annotated")
}

// TestGetRedactionModes walks the redaction part of the table documented on
// redactable.Header for a sensitive, redactable resource.
func TestGetRedactionModes(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		role     role.Role
		mode     redactable.Mode
		redacted bool
	}{
		{
			name: "no header, admin: nothing is redacted",
			role: role.Admin,
			mode: redactable.ModeDisabled,
		},
		{
			name: "accept, admin: an admin is entitled to the secrets",
			role: role.Admin,
			mode: redactable.ModeAccept,
		},
		{
			name:     "accept, reader: served, but redacted",
			role:     role.Reader,
			mode:     redactable.ModeAccept,
			redacted: true,
		},
		{
			name:     "accept, operator: served, but redacted",
			role:     role.Operator,
			mode:     redactable.ModeAccept,
			redacted: true,
		},
		{
			name:     "enforce, admin: redacted even for an admin",
			role:     role.Admin,
			mode:     redactable.ModeEnforce,
			redacted: true,
		},
		{
			name:     "enforce, reader: redacted",
			role:     role.Reader,
			mode:     redactable.ModeEnforce,
			redacted: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			raw, wrapped := setup(t)

			require.NoError(t, raw.Create(t.Context(), wireguardLink()))

			link, err := safe.StateGetByID[*networkres.LinkSpec](request(t.Context(), test.role, test.mode), wrapped, "kubespan")
			require.NoError(t, err)

			if test.redacted {
				requireRedacted(t, link)
			} else {
				requireInFull(t, link)
			}

			// whatever happened on the way out, the stored resource is left alone
			stored, err := safe.StateGetByID[*networkres.LinkSpec](t.Context(), raw, "kubespan")
			require.NoError(t, err)
			requireInFull(t, stored)
		})
	}
}

// TestAccessPolicyModes walks the access part of the table documented on
// redactable.Header.
func TestAccessPolicyModes(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name         string
		resourceType resource.Type
		namespace    resource.Namespace
		id           resource.ID
		// allowed lists the roles which may read the resource in each mode.
		allowed map[redactable.Mode][]role.Role
	}{
		{
			name:         "non-sensitive",
			resourceType: networkres.LinkStatusType,
			namespace:    networkres.NamespaceName,
			id:           "eth0",
			allowed: map[redactable.Mode][]role.Role{
				redactable.ModeDisabled: allRoles,
				redactable.ModeAccept:   allRoles,
				redactable.ModeEnforce:  allRoles,
			},
		},
		{
			name:         "sensitive, redactable",
			resourceType: networkres.LinkSpecType,
			namespace:    networkres.NamespaceName,
			id:           "eth0",
			allowed: map[redactable.Mode][]role.Role{
				// a client which didn't opt in gets the pre-redaction behavior
				redactable.ModeDisabled: {role.Admin},
				redactable.ModeAccept:   allRoles,
				redactable.ModeEnforce:  allRoles,
			},
		},
		{
			name:         "sensitive, not redactable",
			resourceType: secrets.OSRootType,
			namespace:    secrets.NamespaceName,
			id:           secrets.OSRootID,
			allowed: map[redactable.Mode][]role.Role{
				redactable.ModeDisabled: {role.Admin},
				redactable.ModeAccept:   {role.Admin},
				// there is no way to serve this one redacted, so it is denied to everyone: that
				// denial is what tells the caller to leave the type out of what it is collecting
				redactable.ModeEnforce: nil,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, wrapped := setup(t)

			for mode, allowed := range test.allowed {
				for _, r := range allRoles {
					ctx := request(t.Context(), r, mode)
					md := resource.NewMetadata(test.namespace, test.resourceType, test.id, resource.VersionUndefined)

					_, err := wrapped.Get(ctx, md)

					if slices.Contains(allowed, r) {
						// the resource itself is not there, but the access is allowed
						require.True(t, state.IsNotFoundError(err), "mode %q, %s: expected a not found error, got %v", mode, r, err)
					} else {
						require.Equal(t, codes.PermissionDenied, status.Code(err), "mode %q, %s: expected permission denied, got %v", mode, r, err)
					}
				}
			}
		})
	}
}

func TestUnsupportedRedactionMode(t *testing.T) {
	t.Parallel()

	_, wrapped := setup(t)

	ctx := request(t.Context(), role.Admin, "redact-everything-please")

	_, err := wrapped.Get(ctx, networkres.NewLinkSpec(networkres.NamespaceName, "eth0").Metadata())

	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Contains(t, status.Convert(err).Message(), redactable.Header)
}

func TestListRedacted(t *testing.T) {
	t.Parallel()

	raw, wrapped := setup(t)

	require.NoError(t, raw.Create(t.Context(), wireguardLink()))
	require.NoError(t, raw.Create(t.Context(), networkres.NewLinkSpec(networkres.NamespaceName, "eth0")))

	links, err := safe.StateListAll[*networkres.LinkSpec](
		request(t.Context(), role.Reader, redactable.ModeAccept), wrapped,
	)
	require.NoError(t, err)
	require.Equal(t, 2, links.Len())

	for link := range links.All() {
		_, redacted := link.Metadata().Annotations().Get(constants.AnnotationRedacted)

		switch link.Metadata().ID() {
		case "kubespan":
			require.Equal(t, constants.Redacted, link.TypedSpec().Wireguard.PrivateKey)
			require.True(t, redacted, "the link carrying a key should be marked as redacted")
		case "eth0":
			require.Empty(t, link.TypedSpec().Wireguard.PrivateKey)
			require.False(t, redacted, "the link carrying no key should not be marked as redacted")
		}
	}
}

func TestWatchRedacted(t *testing.T) {
	t.Parallel()

	raw, wrapped := setup(t)

	ctx, cancel := context.WithCancel(request(t.Context(), role.Reader, redactable.ModeAccept))
	t.Cleanup(cancel)

	ch := make(chan state.Event)

	require.NoError(t, wrapped.WatchKind(ctx, networkres.NewLinkSpec(networkres.NamespaceName, "").Metadata(), ch))

	link := wireguardLink()
	require.NoError(t, raw.Create(t.Context(), link))

	event := <-ch
	require.Equal(t, state.Created, event.Type)
	require.Equal(t, constants.Redacted, event.Resource.(*networkres.LinkSpec).TypedSpec().Wireguard.PrivateKey) //nolint:forcetypeassert

	// an update carries the old version of the resource as well, it has to be redacted too
	link.TypedSpec().MTU = 1400
	require.NoError(t, raw.Update(t.Context(), link))

	event = <-ch
	require.Equal(t, state.Updated, event.Type)
	require.Equal(t, constants.Redacted, event.Resource.(*networkres.LinkSpec).TypedSpec().Wireguard.PrivateKey) //nolint:forcetypeassert
	require.NotNil(t, event.Old)
	require.Equal(t, constants.Redacted, event.Old.(*networkres.LinkSpec).TypedSpec().Wireguard.PrivateKey) //nolint:forcetypeassert
}

// TestWatchTombstones covers the events which carry a tombstone instead of a resource: a tombstone
// holds the metadata alone and panics when asked for a spec, so redaction has to leave it alone.
//
// The watch bootstrap marker is the one which bit in practice: every kind watch the dashboard opens
// ends its bootstrap with one.
func TestWatchTombstones(t *testing.T) {
	t.Parallel()

	t.Run("bootstrapped marker", func(t *testing.T) {
		t.Parallel()

		raw, wrapped := setup(t)

		require.NoError(t, raw.Create(t.Context(), wireguardLink()))

		ctx, cancel := context.WithCancel(request(t.Context(), role.Reader, redactable.ModeAccept))
		t.Cleanup(cancel)

		ch := make(chan state.Event)

		require.NoError(t, wrapped.WatchKind(ctx, networkres.NewLinkSpec(networkres.NamespaceName, "").Metadata(), ch,
			state.WithBootstrapContents(true)))

		event := <-ch
		require.Equal(t, state.Created, event.Type)
		require.Equal(t, constants.Redacted, event.Resource.(*networkres.LinkSpec).TypedSpec().Wireguard.PrivateKey) //nolint:forcetypeassert

		event = <-ch
		require.Equal(t, state.Bootstrapped, event.Type)
		require.True(t, resource.IsTombstone(event.Resource), "the bootstrap marker should carry a tombstone")
	})

	t.Run("watch of a missing resource", func(t *testing.T) {
		t.Parallel()

		_, wrapped := setup(t)

		ctx, cancel := context.WithCancel(request(t.Context(), role.Reader, redactable.ModeAccept))
		t.Cleanup(cancel)

		ch := make(chan state.Event)

		require.NoError(t, wrapped.Watch(ctx, networkres.NewLinkSpec(networkres.NamespaceName, "eth0").Metadata(), ch))

		event := <-ch
		require.Equal(t, state.Destroyed, event.Type)
		require.True(t, resource.IsTombstone(event.Resource), "a watch of a resource which is not there should carry a tombstone")
	})
}

// TestWatchNotRedactedWithoutHeader makes sure the opt-in reaches the watch path too.
func TestWatchNotRedactedWithoutHeader(t *testing.T) {
	t.Parallel()

	raw, wrapped := setup(t)

	ctx, cancel := context.WithCancel(request(t.Context(), role.Admin, redactable.ModeDisabled))
	t.Cleanup(cancel)

	ch := make(chan state.Event)

	require.NoError(t, wrapped.WatchKind(ctx, networkres.NewLinkSpec(networkres.NamespaceName, "").Metadata(), ch))

	require.NoError(t, raw.Create(t.Context(), wireguardLink()))

	event := <-ch
	require.Equal(t, state.Created, event.Type)
	requireInFull(t, event.Resource.(*networkres.LinkSpec)) //nolint:forcetypeassert
}
