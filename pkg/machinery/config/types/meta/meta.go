// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package meta provides common meta types for config documents.
package meta

// Meta is a shared meta information for config documents.
type Meta struct {
	Kind    string `yaml:"kind"`
	Version string `yaml:"version,omitempty"`
}
