// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
)

// qcow2Format is the qcow2 format name used by both qemu-img and libvirt.
const qcow2Format = "qcow2"

// qemuImgCreateLinked creates a thin qcow2 overlay at dst backed by src, which must stay unchanged while dst exists.
func qemuImgCreateLinked(ctx context.Context, src, dst string, sizeBytes uint64) error {
	srcFormat, err := qemuImgDetectFormat(ctx, src)
	if err != nil {
		return err
	}

	args := []string{
		"create",
		"-f", qcow2Format,
		"-F", srcFormat,
		"-b", src,
		dst,
	}

	if sizeBytes > 0 {
		args = append(args, strconv.FormatUint(sizeBytes, 10))
	}

	if out, err := exec.CommandContext(ctx, "qemu-img", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("qemu-img create -b %q %q: %w: %s", src, dst, err, out)
	}

	return nil
}

// qemuImgConvert copies src into an independent qcow2 at dst.
func qemuImgConvert(ctx context.Context, src, dst string) error {
	args := []string{"convert", "-O", qcow2Format, src, dst}

	if out, err := exec.CommandContext(ctx, "qemu-img", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("qemu-img convert %q %q: %w: %s", src, dst, err, out)
	}

	return nil
}

// qemuImgResize grows the virtual size of dst to sizeBytes.
func qemuImgResize(ctx context.Context, dst string, sizeBytes uint64) error {
	args := []string{"resize", dst, strconv.FormatUint(sizeBytes, 10)}

	if out, err := exec.CommandContext(ctx, "qemu-img", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("qemu-img resize %q: %w: %s", dst, err, out)
	}

	return nil
}

// qemuImgDetectFormat returns the format of src, which a linked overlay has to declare for its backing file.
func qemuImgDetectFormat(ctx context.Context, src string) (string, error) {
	out, err := exec.CommandContext(ctx, "qemu-img", "info", "--output=json", src).Output()
	if err != nil {
		return "", fmt.Errorf("qemu-img info %q: %w", src, err)
	}

	var info struct {
		Format string `json:"format"`
	}

	if err := json.Unmarshal(out, &info); err != nil {
		return "", fmt.Errorf("qemu-img info %q: parse: %w", src, err)
	}

	if info.Format == "" {
		return "", fmt.Errorf("qemu-img info %q: no format reported", src)
	}

	return info.Format, nil
}
