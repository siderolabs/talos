// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

// ContentLibraryConfig defines the interface to access content library configuration.
type ContentLibraryConfig interface {
	NamedDocument
	ContentLibraryConfigSignal()

	// BackingVolumeName is the name of the volume storing the library's contents, as declared by
	// the user, existing or external volume document backing it.
	BackingVolumeName() string
}
