// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package redactable

import (
	"context"
	"fmt"

	"google.golang.org/grpc/metadata"
)

// Header is the gRPC metadata header which carries the redaction intent of the requester.
//
// Redaction is strictly opt-in: a request which doesn't carry this header is served exactly the way
// it was served before redaction existed. An old client therefore never receives a resource with
// some of its values quietly swapped for a placeholder it has no way to recognize, and it never sees
// a resource which the access policy would have denied.
//
// # The access model
//
// Whether a resource is served, and whether it is served in full, is decided by four inputs:
//
//   - the sensitivity of the resource type, declared by its ResourceDefinition;
//   - whether the type opted into redaction via [Register];
//   - the roles the requester carries (only os:admin is relevant here);
//   - the redaction mode the requester asked for with this header.
//
// A "redactable" type below is one registered with [Register]: its spec knows how to redact the
// values it carries, and it has been reviewed as safe to hand out in that form. Only a sensitive type
// can be registered, so a non-sensitive type is never redacted. A sensitive type which is not
// registered is never served to anyone but an os:admin, because there is nothing meaningful left of
// it once redacted, or because it has not been reviewed yet.
//
//	                             | (no header)     | accept            | enforce
//	-----------------------------+-----------------+-------------------+------------------
//	non-sensitive                | in full         | in full           | in full
//	-----------------------------+-----------------+-------------------+------------------
//	sensitive, redactable        | admin: in full  | admin: in full    | redacted
//	                             | other: denied   | other: redacted   |
//	-----------------------------+-----------------+-------------------+------------------
//	sensitive, not redactable    | admin: in full  | admin: in full    | denied
//	                             | other: denied   | other: denied     |
//
// "redacted" above means the sensitive values of the spec are replaced, and the resource is marked
// with the constants.AnnotationRedacted annotation. A resource of a redactable type which carries
// nothing sensitive at the moment is served as it is and is not annotated, whatever the mode.
//
// Note that redaction only reaches the spec fields marked with the `redact:` struct tag. Resource
// metadata -- IDs, labels, annotations, owner -- is never redacted, which is why opting a sensitive
// type into redaction is a deliberate, per-type review and not something a client can ask for.
const Header = "x-talos-dev-redact"

// Mode is the redaction intent a requester declares with the [Header].
//
// See [Header] for the access model the modes take part in.
type Mode string

const (
	// ModeDisabled is the mode of a request which doesn't carry the [Header].
	//
	// Nothing is ever redacted, and a sensitive resource is served to an os:admin only. This is the
	// behavior of any client which predates redaction.
	ModeDisabled Mode = ""

	// ModeAccept tells the server that the requester is able to handle a redacted resource.
	//
	// A sensitive but redactable resource is then served to a requester without the os:admin role,
	// with its sensitive values redacted. An os:admin is entitled to see everything, so its results
	// are not redacted. Use this to widen what a reader or an operator can look at.
	ModeAccept Mode = "accept"

	// ModeEnforce tells the server to redact whatever it can, whoever is asking.
	//
	// Every sensitive resource which knows how to redact itself is served redacted, including to an
	// os:admin, and a sensitive resource which cannot be redacted is denied to everyone. Use this to
	// collect resources into an artifact which is going to leave the cluster, such as a support
	// bundle: the PermissionDenied then tells the collector that this type has to be left out
	// entirely.
	ModeEnforce Mode = "enforce"
)

// WithMode returns a context which asks the server for the given redaction mode.
//
// [ModeDisabled] clears the header, so the request is served without any redaction.
func WithMode(ctx context.Context, mode Mode) context.Context {
	md, _ := metadata.FromOutgoingContext(ctx)

	md = md.Copy()

	if mode == ModeDisabled {
		md.Delete(Header)
	} else {
		md.Set(Header, string(mode))
	}

	return metadata.NewOutgoingContext(ctx, md)
}

// ModeFromIncomingContext returns the redaction mode requested by the incoming gRPC request.
//
// A request without the header asks for [ModeDisabled]. An unparseable header is an error, as
// silently falling back to no redaction would hand out unredacted values to a client which asked for
// the opposite.
func ModeFromIncomingContext(ctx context.Context) (Mode, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ModeDisabled, nil
	}

	values := md.Get(Header)

	switch len(values) {
	case 0:
		return ModeDisabled, nil
	case 1:
		// handled below
	default:
		return ModeDisabled, fmt.Errorf("multiple %q headers in the request", Header)
	}

	switch mode := Mode(values[0]); mode {
	case ModeAccept, ModeEnforce:
		return mode, nil
	case ModeDisabled:
		return ModeDisabled, nil
	default:
		return ModeDisabled, fmt.Errorf("unsupported %q value %q (supported: %q, %q)",
			Header, values[0], ModeAccept, ModeEnforce)
	}
}
