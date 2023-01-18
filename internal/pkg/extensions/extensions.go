// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package extensions provides function to manage system extensions.
package extensions

import (
	"path/filepath"

	"github.com/siderolabs/talos/pkg/machinery/extensions"
)

// Extension represents unpacked extension in the filesystem.
type Extension struct {
	Manifest extensions.Manifest

	directory  string
	rootfsPath string
}

func newExtension(rootfsPath, directory string) *Extension {
	extension := &Extension{
		rootfsPath: rootfsPath,
		directory:  directory,
	}

	if extension.directory == "" {
		extension.directory = filepath.Base(rootfsPath)
	}

	return extension
}
