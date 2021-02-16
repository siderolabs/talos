// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemuimg

import "github.com/talos-systems/go-cmd/pkg/cmd"

// Convert converts an image from one format to another.
func Convert(inputFmt, outputFmt, options, src, dest string) (err error) {
	if _, err = cmd.Run("qemu-img", "convert", "-f", inputFmt, "-O", outputFmt, "-o", options, src, dest); err != nil {
		return err
	}

	return nil
}
