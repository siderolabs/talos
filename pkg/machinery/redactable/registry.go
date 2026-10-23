// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package redactable

import (
	"fmt"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// redactableTypes is the registry of the resource types which might be returned in the redacted form.
var redactableTypes sync.Map

// Register marks the resource type as safe to be returned in the redacted form.
//
// A resource type is marked sensitive when it might carry secrets, which makes it readable only with
// the os:admin role. Registering the type here relaxes that: the requesters without the os:admin role
// are allowed to read the resource, but the spec they get back has its sensitive values redacted.
//
// Only a sensitive type can be registered, and Register panics otherwise: a non-sensitive type is
// readable in full by anyone, so redacting it in some of the modes would only protect the secrets
// from the clients which asked for it. A type carrying secrets has to be marked sensitive first.
//
// Only register a type once it has been reviewed field by field: redaction covers the spec fields
// marked with the `redact:` tag and nothing else, so the rest of the spec and the whole of the
// metadata (IDs, labels, annotations, owner) are returned as is. A type which carries no useful
// information once redacted should stay unregistered instead.
//
// The type parameters are the ones of the typed resource, which ties the registration to the spec
// type, so that only the types which actually implement [Spec] can be registered, and to the
// resource definition, which supplies the resource type and its sensitivity:
//
//	redactable.Register[LinkSpecSpec, LinkSpecExtension]()
func Register[T typed.DeepCopyable[T], E typed.Extension, PT SpecPtr[T]]() {
	var extension E

	rd := extension.ResourceDefinition()

	if rd.Sensitivity != meta.Sensitive {
		panic(fmt.Sprintf("resource type %q is registered as redactable, but it is not sensitive", rd.Type))
	}

	redactableTypes.Store(rd.Type, struct{}{})
}

// IsRedactable reports whether the resource type was registered with [Register].
//
// A redactable type is always sensitive.
func IsRedactable(resourceType resource.Type) bool {
	_, ok := redactableTypes.Load(resourceType)

	return ok
}
