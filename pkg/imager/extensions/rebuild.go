// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extensions

import (
	"os"
	"os/exec"
)

func (builder *Builder) rebuildInitramfs(tempDir string) error {
	builder.Printf("creating system extensions initramfs archive and compressing it")

	// the code below runs the equivalent of:
	//   find $tempDir -print | cpio -H newc --create --reproducible | xz -v -C crc32 -0 -e -T 0 -z

	listing, err := buildContents(tempDir)
	if err != nil {
		return err
	}

	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		return err
	}

	defer pipeR.Close() //nolint:errcheck
	defer pipeW.Close() //nolint:errcheck

	// build cpio image which contains .sqsh images and extensions.yaml
	cmd1 := exec.Command("cpio", "-H", "newc", "--create", "--reproducible")
	cmd1.Dir = tempDir
	cmd1.Stdin = listing
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
	cmd2 := exec.Command("xz", "-v", "-C", "crc32", "-0", "-e", "-T", "0", "-z")
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

	for i := 0; i < 2; i++ {
		if err = <-errCh; err != nil {
			return err
		}
	}

	return destination.Sync()
}
