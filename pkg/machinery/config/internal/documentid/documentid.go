// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package documentid provides methods to create and extract document IDs.
package documentid

import (
	"cmp"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

const (
	// ManifestAPIVersionKey is the string indicating a manifest's version.
	ManifestAPIVersionKey = "apiVersion"
	// ManifestKindKey is the string indicating a manifest's kind.
	ManifestKindKey = "kind"
	// ManifestNameKey is the string indicating a manifest's name.
	ManifestNameKey = "name"
	// ManifestVersionKey is the string indicating a manifest's version, used only in v1alpha1 document.
	ManifestVersionKey = "version"
)

// DocumentID uniquely identifies a configuration document.
type DocumentID struct {
	APIVersion string
	Kind       string
	Name       string
}

// Meta returns a map representation of the DocumentID suitable for use as metadata in configuration documents.
func (id DocumentID) Meta() map[string]any {
	meta := map[string]any{}

	if id.Kind == "v1alpha1" {
		meta[ManifestVersionKey] = "v1alpha1"

		return meta
	}

	meta[ManifestAPIVersionKey] = id.APIVersion
	meta[ManifestKindKey] = id.Kind

	if id.Name != "" {
		meta[ManifestNameKey] = id.Name
	}

	return meta
}

// Extract extracts a DocumentID from the given configuration document.
func Extract(doc config.Document) DocumentID {
	var apiVersion, kind, name string

	if doc.APIVersion() != "" && doc.Kind() != "" {
		apiVersion = doc.APIVersion()
		kind = doc.Kind()
	}

	if named, ok := doc.(config.NamedDocument); ok {
		name = named.Name()
	}

	return DocumentID{
		APIVersion: apiVersion,
		Kind:       cmp.Or(kind, "v1alpha1"),
		Name:       name,
	}
}
