// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imager

import (
	"os"
	"path/filepath"

	"github.com/siderolabs/go-cmd/pkg/cmd"
)

func (i *Imager) postProcessTar(filename string) error {
	dir := filepath.Dir(filename)
	src := "disk.raw"

	if err := os.Rename(filename, filepath.Join(dir, src)); err != nil {
		return err
	}

	outPath := filename + ".tar.gz"

	if _, err := cmd.Run("tar", "-cvf", outPath, "-C", dir, "--sparse", "--use-compress-program=pigz -6", src); err != nil {
		return err
	}

	return os.Remove(filepath.Join(dir, src))
}

func (i *Imager) postProcessGz(filename string) error {
	if _, err := cmd.Run("pigz", "-6", filename); err != nil {
		return err
	}

	return nil
}

func (i *Imager) postProcessXz(filename string) error {
	if _, err := cmd.Run("xz", "-0", "-T", "0", filename); err != nil {
		return err
	}

	return nil
}
