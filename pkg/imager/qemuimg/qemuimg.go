// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package qemuimg provides a wrapper around qemu-img.
package qemuimg

import (
	"context"
	"os"

	"github.com/siderolabs/go-cmd/pkg/cmd"
)

// Convert converts an image from one format to another.
func Convert(ctx context.Context, inputFmt, outputFmt, options, path string, printf func(string, ...any)) error {
	src := path + ".in"
	dest := path

	printf("converting %s to %s", inputFmt, outputFmt)

	if err := os.Rename(path, src); err != nil {
		return err
	}

	if _, err := cmd.RunWithOptions(ctx, "qemu-img", []string{"convert", "-f", inputFmt, "-O", outputFmt, "-o", options, src, dest}); err != nil {
		return err
	}

	return os.Remove(src)
}

// Resize an image.
func Resize(ctx context.Context, file, size string) error {
	if _, err := cmd.RunWithOptions(ctx, "qemu-img", []string{"resize", file, size}); err != nil {
		return err
	}

	return nil
}
