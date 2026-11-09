// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/digitalocean/go-libvirt"
	"github.com/opencontainers/go-digest"

	"github.com/siderolabs/talos/internal/pkg/libvirt/storage/copyowner"
	"github.com/siderolabs/talos/internal/pkg/qemuimg"
)

func (op *CopyOperation) validateVolume(volume Volume) error {
	if op.imageFormat == "" {
		if volume.Format != rawFormat {
			return fmt.Errorf("%w: %q", ErrCopyFormatUnsupported, volume.Format)
		}

		return nil
	}

	if volume.Capacity < op.imageCapacity {
		return &qemuimg.ValidationError{Err: errors.New("staged image capacity is below request")}
	}

	if err := validateImageFile(op.root, op.staged, volume.Format); err != nil {
		return err
	}

	return nil
}

// Raw is an ownership decision made at conversion time, not a content probe:
// guest sectors can contain any image signature and libvirt may report its
// nested virtual size instead of the length of the actual raw destination.
func (c *Client) copiedImageVolume(p libvirt.StoragePool, root *os.Root, name, format string) (Volume, error) {
	if format != rawFormat {
		volume, err := c.copiedVolume(p, name)
		if err != nil {
			return Volume{}, err
		}

		if volume.Format != format {
			return Volume{}, &qemuimg.ValidationError{Err: fmt.Errorf("owned image format %q differs from observed format %q", format, volume.Format)}
		}

		return volume, nil
	}

	volume, found, err := c.volume(p, name, true)
	if err != nil {
		return Volume{}, err
	}

	if !found {
		return Volume{}, fmt.Errorf("copy volume %q has no libvirt observation", name)
	}

	file, err := root.Lstat(name)
	if err != nil {
		return Volume{}, err
	}

	if !file.Mode().IsRegular() || file.Size() < 0 {
		return Volume{}, fmt.Errorf("copy volume %q is not a regular raw image", name)
	}

	volume.Format = rawFormat
	volume.Capacity = uint64(file.Size())

	return volume, nil
}

// Import verifies an opened source inode once against its content pin, converts
// from that inode and checks its identity before durable publication.
func (op *CopyOperation) Import(ctx context.Context, tool *qemuimg.Client, format string, capacity uint64) error {
	if op == nil || op.root == nil {
		return errors.New("copy stage is closed")
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	input, original, err := openCopySource(op.Source)
	if err != nil {
		if errors.Is(err, ErrCopySourceNotRegular) {
			return &qemuimg.ValidationError{Err: err}
		}

		return err
	}
	defer input.Close() //nolint:errcheck

	if err := tool.Preflight(ctx, input, format, capacity); err != nil {
		return err
	}

	return op.importOpenedImage(ctx, tool, input, original, format, capacity)
}

func (op *CopyOperation) importOpenedImage(ctx context.Context, tool *qemuimg.Client, input *os.File, original os.FileInfo, format string, capacity uint64) error {
	_, err := op.verifyImageSource(ctx, input, original, op.Expected)
	if err != nil {
		return err
	}

	output, err := op.root.OpenFile(op.staged, os.O_CREATE|os.O_EXCL|os.O_RDWR, volumeMode)
	if err != nil {
		return err
	}
	defer output.Close() //nolint:errcheck

	op.progress.BytesCopied = 0
	op.observe(CopyPhaseConvert)

	if err := tool.Convert(ctx, qemuimg.ConvertSpec{
		Source:      input,
		Destination: output,
		Format:      format,
		Capacity:    capacity,

		Progress: func(percent float64) {
			if op.observer == nil {
				return
			}

			// QEMU reports a percentage, so these are source-equivalent byte
			// estimates rather than measurements of physical destination I/O.
			completed := uint64(float64(original.Size()) * percent / 100)
			if completed > op.progress.BytesCopied {
				op.progress.BytesCopied = completed
				op.observe(CopyPhaseConvert)
			}
		},
	}); err != nil {
		return err
	}

	if err := op.checkImageSource(input, original); err != nil {
		return err
	}

	op.observe(CopyPhaseSync)

	if err := output.Sync(); err != nil {
		return err
	}

	op.imageFormat = format
	op.imageCapacity = capacity
	op.witness = copyowner.Witness(op.Name, true)

	return ctx.Err()
}

func validateImageFile(root *os.Root, name, format string) error {
	file, err := root.Open(name)
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck

	if format == rawFormat {
		return nil
	} // Guest bytes may themselves start with qcow2 magic.

	actual, err := qemuimg.Inspect(file)
	if err != nil {
		return err
	}

	if actual != format {
		return &qemuimg.ValidationError{Err: fmt.Errorf("observed image format %q differs from file format %q", format, actual)}
	}

	return nil
}

func (op *CopyOperation) verifyImageSource(ctx context.Context, input *os.File, original os.FileInfo, expected string) (string, error) {
	if _, err := input.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	digester := digest.SHA256.Digester()
	op.progress.TotalBytes = uint64(original.Size())
	op.progress.TotalKnown = true
	op.progress.BytesCopied = 0
	op.observe(CopyPhaseVerify)

	reader := copyReadFunc(func(buffer []byte) (int, error) {
		if err := ctx.Err(); err != nil {
			return 0, err
		}

		n, readErr := input.Read(buffer)
		if op.observer != nil && n > 0 {
			op.progress.BytesCopied += uint64(n)
			op.observe(CopyPhaseVerify)
		}

		return n, readErr
	})
	if _, err := io.CopyBuffer(digester.Hash(), reader, make([]byte, 128<<10)); err != nil {
		return "", err
	}

	if err := op.checkImageSource(input, original); err != nil {
		return "", err
	}

	actual := digester.Digest().String()
	if expected != "" && actual != expected {
		return "", &qemuimg.ValidationError{Err: fmt.Errorf("copy source digest mismatch: expected %s", expected)}
	}

	return actual, ctx.Err()
}

func (op *CopyOperation) checkImageSource(input *os.File, original os.FileInfo) error {
	opened, openedErr := input.Stat()

	current, pathErr := os.Lstat(op.Source)
	if err := errors.Join(openedErr, pathErr); err != nil {
		return err
	}

	if !os.SameFile(original, current) || !os.SameFile(original, opened) ||
		current.Size() != original.Size() || opened.Size() != original.Size() ||
		!opened.ModTime().Equal(original.ModTime()) ||
		opened.Sys().(*syscall.Stat_t).Ctim != original.Sys().(*syscall.Stat_t).Ctim {
		return fmt.Errorf("%w: %q changed during conversion", ErrCopySourceChanged, op.Source)
	}

	return nil
}
