// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

// Document is a configuration document.
type Document interface {
	// Clone returns a deep copy of the document.
	Clone() Document
}

// SecretDocument is a configuration document that contains secrets.
type SecretDocument interface {
	// Redact does in-place replacement of secrets with the given string.
	Redact(replacement string)
}
