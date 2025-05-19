// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package vmdkconvert provides a wrapper around the vmdk-convert tool from open-vmdk.
package vmdkconvert

import (
	"os"

	"github.com/siderolabs/go-cmd/pkg/cmd"
)

// ConvertToStreamOptimizedVMDK converts a raw / flat / sparse disk image to a stream optimized VMDK.
func ConvertToStreamOptimizedVMDK(path string, printf func(string, ...any)) error {
	src := path + ".in"
	dest := path

	printf("converting disk image to stream optimized vmdk")

	if err := os.Rename(path, src); err != nil {
		return err
	}

	if _, err := cmd.Run("vmdk-convert", src, dest); err != nil {
		return err
	}

	return os.Remove(src)
}
