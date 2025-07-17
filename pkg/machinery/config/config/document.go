// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

// Document is a configuration document.
type Document interface {
	// Clone returns a deep copy of the document.
	Clone() Document
	// Kind returns the kind of the document.
	Kind() string
	// APIVersion returns the API version of the document.
	APIVersion() string
}

// NamedDocument is a configuration document which has a name.
type NamedDocument interface {
	// Name of the document.
	Name() string
}

// ConflictingDocument is a configuration document which conflicts with other document.
//
// If the document is named, it conflicts by name, otherwise it conflicts by kind.
type ConflictingDocument interface {
	ConflictsWithKinds() []string
}

// SecretDocument is a configuration document that contains secrets.
type SecretDocument interface {
	// Redact does in-place replacement of secrets with the given string.
	Redact(replacement string)
}
