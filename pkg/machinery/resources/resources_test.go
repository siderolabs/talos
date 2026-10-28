// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resources_test

import (
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/redactable"
	"github.com/siderolabs/talos/pkg/machinery/resources"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// redactTag is the struct tag redactgen uses to mark a sensitive field.
const redactTag = "redact"

// nonSensitiveExceptions lists the redact-tagged fields a non-sensitive resource is allowed to have
// in its spec, by the path TestNonSensitiveResourcesCarryNoSecrets reports them under.
//
// Every entry is a promise that some code elsewhere keeps the field empty, and only review enforces
// it, so keep this list as short as it is and say in the comment what makes the field safe.
var nonSensitiveExceptions = map[resource.Type][]string{
	// LinkStatusSpec reuses WireguardSpec, which carries the values used to configure a link. The
	// status decoder never fills those in: it skips the private key, and reports a pre-shared key
	// as the PresharedKeyConfigured boolean rather than copying it. See Decode(dev, isStatus=true)
	// in internal/app/machined/pkg/adapters/network/wireguard_spec.go, and the field comments on
	// network.WireguardPeer ("present only in the LinkSpec").
	network.LinkStatusType: {
		"LinkStatusSpec.Wireguard.PrivateKey",
		"LinkStatusSpec.Wireguard.Peers[].PresharedKey",
	},
}

// redactionExceptions lists the sensitive resource types which carry redact-tagged fields and are
// still not registered as redactable.
//
// A type belongs here only when there would be nothing left to read once it is redacted, which is
// the one good reason to keep a sensitive resource admin-only, and the comment has to say so.
var redactionExceptions = map[resource.Type][]string{}

// secretKeywords is the case-insensitive substring set used to flag a field which names a secret,
// the same vocabulary the machine configuration audit uses, see TestRegistryRedactAudit in
// pkg/machinery/config/types.
//
// Deliberately narrow: bare "auth" and "key" would match authorizationConfig, publicKey or
// encryptionKeySize. "userdata" is the odd one out: it names an opaque operator-supplied blob rather
// than a kind of secret, and such a blob carries credentials in practice.
var secretKeywords = regexp.MustCompile(`(?i)(password|passphrase|token|secret|privatekey|presharedkey|apikey|userdata)`)

// secretLookingExceptions lists the fields which match [secretKeywords] without naming a secret, by
// the path TestSecretLookingFieldsAreTagged reports them under.
//
// Only a false positive of the keyword match belongs here, and the comment has to say what the field
// actually holds.
var secretLookingExceptions = map[resource.Type][]string{}

// TestNonSensitiveResourcesCarryNoSecrets asserts that a resource type readable with any role has no
// field marked with the redact tag.
//
// Redaction exists to widen access to a sensitive resource, not to paper over a misclassified one:
// only a sensitive type may be redactable. A non-sensitive type is readable with any role, and a
// client only gets it redacted if it asks for that (see redactable.Header), so a secret in its spec
// reaches an os:reader on any client which doesn't ask. If this check fires, the resource is either
// sensitive and should say so, or it should not be carrying the value in the first place.
func TestNonSensitiveResourcesCarryNoSecrets(t *testing.T) {
	t.Parallel()

	for _, r := range resources.All() {
		rd := r.ResourceDefinition()

		if rd.Sensitivity != meta.NonSensitive {
			continue
		}

		t.Run(rd.Type, func(t *testing.T) {
			t.Parallel()

			found := redactTaggedFields(r)
			exempt := nonSensitiveExceptions[rd.Type]

			assert.Empty(t, without(found, exempt),
				"%s is readable with any role, but its spec has fields marked with the %q tag: "+
					"mark the resource sensitive, or stop putting the value in the spec",
				rd.Type, redactTag)

			// a stale exemption is as dangerous as a missing one: it would keep covering a field
			// which has since been renamed into something the walk no longer recognizes
			assert.Empty(t, without(exempt, found),
				"%s has exemptions in nonSensitiveExceptions which match no field of its spec any more",
				rd.Type)
		})
	}
}

// TestSensitiveResourcesWithSecretsAreRedactable is the other half of the same rule: a sensitive
// resource which says, field by field, what in it is a secret has everything it takes to be served
// in the redacted form, so not registering it only costs the reader access for no gain.
//
// This is what flags a resource type whose redaction was forgotten: marking the fields is the work,
// and [redactable.Register] is the one line which puts it to use.
func TestSensitiveResourcesWithSecretsAreRedactable(t *testing.T) {
	t.Parallel()

	for _, r := range resources.All() {
		rd := r.ResourceDefinition()

		if rd.Sensitivity != meta.Sensitive || redactable.IsRedactable(rd.Type) {
			continue
		}

		t.Run(rd.Type, func(t *testing.T) {
			t.Parallel()

			found := redactTaggedFields(r)
			exempt := redactionExceptions[rd.Type]

			assert.Empty(t, without(found, exempt),
				"%s marks fields of its spec with the %q tag, but is not registered as redactable: "+
					"call redactable.Register for it, or say in redactionExceptions what makes the "+
					"redacted resource not worth reading",
				rd.Type, redactTag)

			assert.Empty(t, without(exempt, found),
				"%s has exemptions in redactionExceptions which match no field of its spec any more",
				rd.Type)
		})
	}
}

// TestSecretLookingFieldsAreTagged is the check which doesn't rely on anyone having remembered to
// mark the field: it reads the field names instead, and asserts that a field which names a secret is
// one redaction takes out.
//
// The two tests above work off the `redact:` tags, so they say nothing about a secret nobody tagged.
// This one flags such a field as soon as it is added to a resource which is served to a non-admin --
// either a non-sensitive one, which anyone reads in full, or a sensitive one which is handed out in
// the redacted form.
//
// A resource which is sensitive and not redactable is skipped: it is served to an os:admin alone and
// always in full, so there is nothing in it to take out. Registering such a type as redactable brings
// it under this check, which is the point at which its fields have to be marked.
func TestSecretLookingFieldsAreTagged(t *testing.T) {
	t.Parallel()

	for _, r := range resources.All() {
		rd := r.ResourceDefinition()

		if rd.Sensitivity == meta.Sensitive && !redactable.IsRedactable(rd.Type) {
			continue
		}

		t.Run(rd.Type, func(t *testing.T) {
			t.Parallel()

			specType := specStructType(r.Spec())
			found := secretLookingFields(specType, specType.Name(), false, map[reflect.Type]struct{}{})
			exempt := secretLookingExceptions[rd.Type]

			assert.Empty(t, without(found, exempt),
				"%s names fields which look like secrets, and redaction leaves them in place: "+
					"mark them with the %q tag, or say in secretLookingExceptions what they hold",
				rd.Type, redactTag)

			assert.Empty(t, without(exempt, found),
				"%s has exemptions in secretLookingExceptions which match no field of its spec any more",
				rd.Type)
		})
	}
}

// TestRedactTaggedFieldsWalk is a sanity check on the walk the tests above are built on: it has to
// actually reach the nested fields, so make sure it finds the ones which are known to be there.
func TestRedactTaggedFieldsWalk(t *testing.T) {
	t.Parallel()

	idx := slices.IndexFunc(resources.All(), func(r meta.ResourceWithRD) bool {
		return r.ResourceDefinition().Type == network.LinkSpecType
	})
	require.NotEqual(t, -1, idx, "LinkSpec is not registered")

	require.ElementsMatch(t, []string{
		"LinkSpecSpec.Wireguard.PrivateKey",
		"LinkSpecSpec.Wireguard.Peers[].PresharedKey",
	}, redactTaggedFields(resources.All()[idx]))
}

// redactTaggedFields returns the paths of the spec fields of the resource which are marked with the
// redact tag.
func redactTaggedFields(r meta.ResourceWithRD) []string {
	specType := specStructType(r.Spec())

	return sensitiveFields(specType, specType.Name(), map[reflect.Type]struct{}{})
}

// specStructType returns the type the paths of a resource spec are reported under.
//
// Resource.Spec() usually hands out a pointer to the spec, but not always, so unwrap as far as it
// takes to name the type.
func specStructType(spec any) reflect.Type {
	typ := reflect.TypeOf(spec)

	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	return typ
}

// without returns the elements of paths which are not in exclude.
func without(paths, exclude []string) []string {
	return xslices.Filter(paths, func(path string) bool {
		return !slices.Contains(exclude, path)
	})
}

// sensitiveFields walks the type and returns the paths of the fields marked with the redact tag.
func sensitiveFields(typ reflect.Type, path string, seen map[reflect.Type]struct{}) []string {
	switch typ.Kind() { //nolint:exhaustive // the rest of the kinds can't carry a struct tag
	case reflect.Pointer:
		return sensitiveFields(typ.Elem(), path, seen)
	case reflect.Slice, reflect.Array, reflect.Map:
		return sensitiveFields(typ.Elem(), path+"[]", seen)
	case reflect.Struct:
		// a recursive type: going around the cycle can't reach any new field
		if _, ok := seen[typ]; ok {
			return nil
		}

		seen[typ] = struct{}{}
		defer delete(seen, typ)

		var result []string

		for field := range typ.Fields() {
			fieldPath := path + "." + field.Name

			if _, ok := field.Tag.Lookup(redactTag); ok {
				result = append(result, fieldPath)

				continue
			}

			result = append(result, sensitiveFields(field.Type, fieldPath, seen)...)
		}

		return result
	default:
		return nil
	}
}

// secretLookingFields walks the type and returns the paths of the string and []byte fields whose name
// or yaml tag matches [secretKeywords] and which redaction does not reach.
//
// A field under a tagged one is reached by the redaction of its parent, so the whole subtree counts as
// covered. The walk stays inside the Talos packages: a field of a foreign type is not something this
// repository can tag, and descending into one (url.URL, netip.Addr) only finds its internals.
func secretLookingFields(typ reflect.Type, path string, covered bool, seen map[reflect.Type]struct{}) []string {
	switch typ.Kind() { //nolint:exhaustive // the rest of the kinds name no field
	case reflect.Pointer:
		return secretLookingFields(typ.Elem(), path, covered, seen)
	case reflect.Slice, reflect.Array, reflect.Map:
		return secretLookingFields(typ.Elem(), path+"[]", covered, seen)
	case reflect.Struct:
		// a recursive type: going around the cycle can't reach any new field
		if _, ok := seen[typ]; ok {
			return nil
		}

		// a field of a foreign type is not something this repository can tag, and descending into
		// one (url.URL, netip.Addr) only finds its internals
		if !strings.Contains(typ.PkgPath(), "siderolabs/talos") {
			return nil
		}

		seen[typ] = struct{}{}
		defer delete(seen, typ)

		var result []string

		for field := range typ.Fields() {
			result = append(result, secretLookingField(field, path, covered, seen)...)
		}

		return result
	default:
		return nil
	}
}

// secretLookingField is secretLookingFields for a single field of a struct.
func secretLookingField(field reflect.StructField, path string, covered bool, seen map[reflect.Type]struct{}) []string {
	if !field.IsExported() {
		return nil
	}

	fieldPath := path + "." + field.Name

	_, tagged := field.Tag.Lookup(redactTag)

	if !covered && !tagged && namesASecret(field) && holdsAValue(field.Type) {
		return []string{fieldPath}
	}

	return secretLookingFields(field.Type, fieldPath, covered || tagged, seen)
}

// namesASecret reports whether the name or the yaml tag of the field matches [secretKeywords].
func namesASecret(field reflect.StructField) bool {
	yamlTag, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")

	return secretKeywords.MatchString(field.Name) || secretKeywords.MatchString(yamlTag)
}

// holdsAValue reports whether the type can carry a secret as read by a human, which is what the
// keyword match is about: a boolean saying that a secret is configured is not one, see
// WireguardPeer.PresharedKeyConfigured.
func holdsAValue(typ reflect.Type) bool {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	switch typ.Kind() { //nolint:exhaustive // everything else is reported by the walk, not here
	case reflect.String:
		return true
	case reflect.Slice, reflect.Array:
		return typ.Elem().Kind() == reflect.Uint8
	default:
		return false
	}
}
