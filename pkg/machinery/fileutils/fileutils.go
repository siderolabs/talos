// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package fileutils provides helpers for handling files which contain secrets.
package fileutils

import (
	"errors"
	"io/fs"
	"os"
)

const (
	// SecretFileMode is the mode of the files which contain secrets: machine configuration,
	// talosconfig, kubeconfig, secrets bundle, private keys, etc.
	SecretFileMode os.FileMode = 0o600

	// SecretDirMode is the mode of the directories which contain files with secrets.
	SecretDirMode os.FileMode = 0o700
)

// WriteSecret writes the data to the file at path with SecretFileMode.
//
// Unlike os.WriteFile, it also restricts the mode of an already existing file.
func WriteSecret(path string, data []byte) error {
	if err := RestrictSecretMode(path); err != nil {
		return err
	}

	return os.WriteFile(path, data, SecretFileMode)
}

// RestrictSecretMode restricts the mode of an existing file at path to SecretFileMode,
// missing files are ignored.
//
// It should be called before writing the file, so that the contents are never exposed:
// os.WriteFile (and most other writers) keep the mode of an already existing file.
func RestrictSecretMode(path string) error {
	if err := os.Chmod(path, SecretFileMode); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	return nil
}
