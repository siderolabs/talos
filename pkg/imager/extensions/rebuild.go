// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extensions

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
)

// rebuildInitramfs rebuilds finalized initramfs with extensions.
//
// If uncompressedListing is not empty, contents will be prepended to the initramfs uncompressed.
// Contents from compressedListing will be appended to the initramfs compressed (xz/zstd) as a second block.
// Original initramfs.xz contents will stay without changes.
func (builder *Builder) rebuildInitramfs(ctx context.Context, tempDir string, quirks quirks.Quirks) error {
	compressedListing, uncompressedListing, err := buildInitramfsContents(tempDir)
	if err != nil {
		return err
	}

	if len(uncompressedListing) > 0 {
		if err = builder.prependUncompressedInitramfs(ctx, tempDir, uncompressedListing); err != nil {
			return fmt.Errorf("error prepending uncompressed initramfs: %w", err)
		}
	}

	if err = builder.appendCompressedInitramfs(ctx, tempDir, compressedListing, quirks); err != nil {
		return fmt.Errorf("error appending compressed initramfs: %w", err)
	}

	return nil
}

func (builder *Builder) appendCompressedInitramfs(ctx context.Context, tempDir string, compressedListing []byte, quirks quirks.Quirks) error {
	builder.Printf("creating system extensions initramfs archive and compressing it")

	// the code below runs the equivalent of:
	//   find $tempDir -print | cpio -H newc --create --reproducible | { xz -v -C crc32 -0 -e -T 0 -z || zstd -T0 -18 -c --quiet }

	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		return err
	}

	defer pipeR.Close() //nolint:errcheck
	defer pipeW.Close() //nolint:errcheck

	// build cpio image which contains .sqsh images and extensions.yaml
	cmd1 := exec.CommandContext(ctx, "cpio", "-H", "newc", "--create", "--reproducible", "--quiet", "-R", "+0:+0")
	cmd1.Dir = tempDir
	cmd1.Stdin = bytes.NewReader(compressedListing)
	cmd1.Stdout = pipeW
	cmd1.Stderr = os.Stderr

	if err = cmd1.Start(); err != nil {
		return err
	}

	if err = pipeW.Close(); err != nil {
		return err
	}

	destination, err := os.OpenFile(builder.InitramfsPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}

	defer destination.Close() //nolint:errcheck

	// append compressed initramfs.sysext to the original initramfs.xz, kernel can read such format
	var cmd2 *exec.Cmd

	if quirks.UseZSTDCompression() {
		cmd2 = exec.CommandContext(ctx, "zstd", "-T0", "-18", "-c", "--quiet")
	} else {
		cmd2 = exec.CommandContext(ctx, "xz", "-v", "-C", "crc32", "-0", "-e", "-T", "0", "-z", "--quiet")
	}

	cmd2.Dir = tempDir
	cmd2.Stdin = pipeR
	cmd2.Stdout = destination
	cmd2.Stderr = os.Stderr

	if err = cmd2.Start(); err != nil {
		return err
	}

	if err = pipeR.Close(); err != nil {
		return err
	}

	errCh := make(chan error, 1)

	go func() {
		errCh <- cmd1.Wait()
	}()

	go func() {
		errCh <- cmd2.Wait()
	}()

	for range 2 {
		if err = <-errCh; err != nil {
			return err
		}
	}

	return destination.Sync()
}

func (builder *Builder) prependUncompressedInitramfs(ctx context.Context, tempDir string, uncompressedListing []byte) error {
	builder.Printf("creating uncompressed initramfs archive")

	// the code below runs the equivalent of:
	//   mv initramfs.xz initramfs.xz-old
	//   find $tempDir -print | cpio -H newc --create --reproducible > initramfs.xz
	//   cat initramfs.xz-old >> initramfs.xz
	//   rm initramfs.xz-old

	initramfsOld := builder.InitramfsPath + "-old"

	if err := os.Rename(builder.InitramfsPath, initramfsOld); err != nil {
		return err
	}

	destination, err := os.OpenFile(builder.InitramfsPath, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	defer destination.Close() //nolint:errcheck

	cmd := exec.CommandContext(ctx, "cpio", "-H", "newc", "--create", "--reproducible", "--quiet", "-R", "+0:+0")
	cmd.Dir = tempDir
	cmd.Stdin = bytes.NewReader(uncompressedListing)
	cmd.Stdout = destination
	cmd.Stderr = os.Stderr

	if err = cmd.Run(); err != nil {
		return err
	}

	old, err := os.Open(initramfsOld)
	if err != nil {
		return err
	}

	defer old.Close() //nolint:errcheck

	if _, err = io.Copy(destination, old); err != nil {
		return err
	}

	if err = destination.Close(); err != nil {
		return err
	}

	if err = old.Close(); err != nil {
		return err
	}

	return os.Remove(initramfsOld)
}
