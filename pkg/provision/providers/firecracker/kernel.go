// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package firecracker

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/talos-systems/talos/pkg/provision/internal/vmlinuz"
)

func uncompressKernel(srcKernelPath, dstKernelPath string) error {
	srcF, err := os.Open(srcKernelPath)
	if err != nil {
		return fmt.Errorf("failed to open kernel asset %q: %w", srcKernelPath, err)
	}

	defer srcF.Close() //nolint:errcheck

	kernelR, err := vmlinuz.Decompress(bufio.NewReader(srcF))
	if err != nil {
		return fmt.Errorf("error decompressing kernel: %w", err)
	}

	defer kernelR.Close() //nolint:errcheck

	dstF, err := os.Create(dstKernelPath)
	if err != nil {
		return fmt.Errorf("error creating temporary kernel image file: %w", err)
	}

	defer dstF.Close() //nolint:errcheck

	if _, err = io.Copy(dstF, kernelR); err != nil {
		return fmt.Errorf("error extracting kernel: %w", err)
	}

	return dstF.Close()
}
