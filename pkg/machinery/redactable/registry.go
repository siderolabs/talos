// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package redactable

import (
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
)

// redactableTypes is the registry of the resource types which might be returned in the redacted form.
var redactableTypes sync.Map

// Register marks the resource type as safe to be returned in the redacted form.
//
// A resource type is marked sensitive when it might carry secrets, which makes it readable only with
// the os:admin role. Registering the type here relaxes that: the requesters without the os:admin role
// are allowed to read the resource, but the spec they get back has its sensitive values redacted.
//
// Only register a type once it has been reviewed field by field: redaction covers the spec fields
// marked with the `redact:` tag and nothing else, so the rest of the spec and the whole of the
// metadata (IDs, labels, annotations, owner) are returned as is. A type which carries no useful
// information once redacted should stay unregistered instead.
//
// The type parameter ties the registration to the spec type, so that only the types which actually
// implement [Spec] can be registered:
//
//	redactable.Register[LinkSpecSpec](LinkSpecType)
func Register[T any, PT interface {
	*T
	Spec
}](resourceType resource.Type) {
	redactableTypes.Store(resourceType, struct{}{})
}

// IsRedactable reports whether the resource type was registered with [Register].
func IsRedactable(resourceType resource.Type) bool {
	_, ok := redactableTypes.Load(resourceType)

	return ok
}
