// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package assets provides bootloader assets.
package assets

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

// Assets is a list of assets.
type Assets []Asset

// Asset represents a file required by a target.
type Asset struct {
	Source      string
	Destination string
}

// Install copies the assets to the bootloader partition.
func (assets Assets) Install() error {
	for _, asset := range assets {
		asset := asset

		if assetErr := func() error {
			var (
				sourceFile *os.File
				destFile   *os.File
			)

			sourceFile, err := os.Open(asset.Source)
			if err != nil {
				return err
			}
			//nolint:errcheck
			defer sourceFile.Close()

			if err = os.MkdirAll(filepath.Dir(asset.Destination), os.ModeDir); err != nil {
				return err
			}

			if destFile, err = os.Create(asset.Destination); err != nil {
				return err
			}

			//nolint:errcheck
			defer destFile.Close()

			log.Printf("copying %s to %s\n", sourceFile.Name(), destFile.Name())

			if _, err = io.Copy(destFile, sourceFile); err != nil {
				log.Printf("failed to copy %s to %s\n", sourceFile.Name(), destFile.Name())

				return err
			}

			if err = destFile.Close(); err != nil {
				log.Printf("failed to close %s", destFile.Name())

				return err
			}

			if err = sourceFile.Close(); err != nil {
				log.Printf("failed to close %s", sourceFile.Name())

				return err
			}

			return nil
		}(); assetErr != nil {
			return assetErr
		}
	}

	return nil
}
