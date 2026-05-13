// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package types_test

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	_ "github.com/siderolabs/talos/pkg/machinery/config/types" // register all document kinds
)

// TestRegistryRedactAudit walks every registered document kind, looks for
// fields whose yaml tag name suggests a secret, and asserts that:
//
//  1. The document implements config.SecretDocument.
//  2. Calling Redact actually replaces the field's value.
//
// Detection is a pure case-insensitive substring match on the yaml tag against
// a small keyword list. Tags in this codebase are camelCase, so no separator
// handling is needed. New documents added in the future will be picked up
// automatically without an allowlist to maintain.
//
// The frozen v1alpha1 Config is registered too and gets walked here, but in
// practice it never grows new fields, so its Redact is also covered by
// v1alpha1_redact_test.go for direct coverage of its existing surface.
func TestRegistryRedactAudit(t *testing.T) {
	t.Parallel()

	kinds := registry.Kinds()
	require.NotEmpty(t, kinds, "no document kinds registered, blank imports in types.go are missing")

	for _, kind := range kinds {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			doc, err := registry.New(kind, "v1alpha1")
			require.NoError(t, err)

			findings := findSecretFields(reflect.ValueOf(doc), "")
			if len(findings) == 0 {
				return
			}

			// Seed each finding with a unique sentinel so we can tell which one
			// (if any) failed to be redacted.
			const replacement = "__REDACTED__"

			for i := range findings {
				findings[i].set("sentinel-" + findings[i].path)
			}

			secretDoc, ok := doc.(config.SecretDocument)
			if !ok {
				paths := make([]string, len(findings))
				for i, f := range findings {
					paths[i] = f.path
				}

				t.Fatalf("kind %q has secret-looking fields but does not implement config.SecretDocument:\n  %s",
					kind, strings.Join(paths, "\n  "))
			}

			secretDoc.Redact(replacement)

			for _, f := range findings {
				assert.Equal(t, replacement, f.get(),
					"field %s.%s was not redacted by Redact", kind, f.path)
			}
		})
	}
}

// secretKeywords is the case-insensitive substring set used to flag suspect yaml tags.
// Deliberately narrow: bare "auth" and "key" would generate false positives
// (authorizationConfig, publicKey, encryptionKeySize, ...).
var secretKeywords = regexp.MustCompile(`(?i)(password|passphrase|token|secret|privatekey|presharedkey|apikey)`)

// finding holds a single leaf field flagged by the heuristic, with closures to
// read and write its value through whatever pointer/slice chain leads to it.
type finding struct {
	path string
	set  func(string)
	get  func() string
}

// findSecretFields walks v (expected to be a pointer to a document struct or
// any struct/pointer/slice/map reachable from one) and returns every string or
// []byte leaf whose yaml tag matches the heuristic.
//
// The walk instantiates nil pointers, adds one element to empty slices of
// structs, and inserts one entry into empty maps so nested secrets are
// reachable. It skips:
//   - non-Talos packages (e.g., url.URL, time.Time) to avoid runaway recursion.
//   - cycles, via a depth cap.
//
//nolint:gocyclo,cyclop
func findSecretFields(v reflect.Value, path string) []finding {
	const maxDepth = 8

	if strings.Count(path, ".")+strings.Count(path, "[0]") > maxDepth {
		return nil
	}

	//nolint:exhaustive // intentionally only handles pointer/slice/map/struct, everything else (scalars, channels, etc.) returns nil via the default.
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			if !v.CanSet() {
				return nil
			}

			v.Set(reflect.New(v.Type().Elem()))
		}

		return findSecretFields(v.Elem(), path)

	case reflect.Slice:
		// []byte leaf is handled at the field level in the Struct case below.
		// Here we only descend into slices of struct / *struct, adding one
		// element so nested fields become reachable.
		elemType := v.Type().Elem()
		if elemType.Kind() != reflect.Struct && !(elemType.Kind() == reflect.Pointer && elemType.Elem().Kind() == reflect.Struct) {
			return nil
		}

		if v.Len() == 0 {
			if !v.CanSet() {
				return nil
			}

			v.Set(reflect.Append(v, reflect.New(elemType).Elem()))
		}

		return findSecretFields(v.Index(0), path+"[0]")

	case reflect.Map:
		// Only descend into maps whose value type is *struct. With a pointer
		// element, the inserted map entry and our walk target share the same
		// underlying struct, so reflect-based mutations and later Redact calls
		// see each other. map[K]struct (value, not pointer) would need a more
		// complex round-trip and is not used by any registered kind today.
		elemType := v.Type().Elem()
		if elemType.Kind() != reflect.Pointer || elemType.Elem().Kind() != reflect.Struct {
			return nil
		}

		if v.IsNil() {
			if !v.CanSet() {
				return nil
			}

			v.Set(reflect.MakeMap(v.Type()))
		}

		key := reflect.New(v.Type().Key()).Elem()
		if key.Kind() != reflect.String {
			// No registered kind uses non-string map keys for secret-bearing structs.
			return nil
		}

		key.SetString("audit")

		entry := reflect.New(elemType.Elem())
		v.SetMapIndex(key, entry)

		return findSecretFields(entry, path+`["audit"]`)

	case reflect.Struct:
		if !isTalosType(v.Type()) {
			return nil
		}

		var out []finding

		for i := range v.NumField() {
			field := v.Type().Field(i)
			if !field.IsExported() {
				continue
			}

			tagName := yamlTagName(field.Tag.Get("yaml"))
			fieldPath := path

			switch {
			case field.Anonymous:
				// Embedded struct (e.g., meta.Meta), recurse without extending path.
			case tagName == "" || tagName == "-":
				continue
			default:
				if fieldPath != "" {
					fieldPath += "."
				}

				fieldPath += tagName
			}

			fv := v.Field(i)

			// Leaf string field with a secret-looking tag.
			if fv.Kind() == reflect.String && secretKeywords.MatchString(tagName) {
				out = append(out, finding{
					path: fieldPath,
					set:  func(s string) { fv.SetString(s) },
					get:  fv.String,
				})

				continue
			}

			// Leaf []byte field with a secret-looking tag.
			if fv.Kind() == reflect.Slice && fv.Type().Elem().Kind() == reflect.Uint8 && secretKeywords.MatchString(tagName) {
				out = append(out, finding{
					path: fieldPath,
					set:  func(s string) { fv.SetBytes([]byte(s)) },
					get:  func() string { return string(fv.Bytes()) },
				})

				continue
			}

			out = append(out, findSecretFields(fv, fieldPath)...)
		}

		return out

	default:
		return nil
	}
}

func yamlTagName(tag string) string {
	name, _, _ := strings.Cut(tag, ",")

	return name
}

func isTalosType(t reflect.Type) bool {
	return strings.HasPrefix(t.PkgPath(), "github.com/siderolabs/talos/")
}
