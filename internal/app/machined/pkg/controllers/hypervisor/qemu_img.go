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

// qcow2Format is the qcow2 format name both qemu-img and libvirt use.
const qcow2Format = "qcow2"

// qemuImgCreateLinked materializes a thin qcow2 overlay backed by src at dst.
//
// Writes land in dst; reads for unmodified blocks are served from src. src must stay available
// and immutable for the lifetime of dst. Idempotent at the call site: callers must check for
// pre-existing dst before invoking.
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

// qemuImgConvert produces an independent qcow2 at dst from src. The result shares no state with
// src: dst's lifetime is independent of src.
func qemuImgConvert(ctx context.Context, src, dst string) error {
	args := []string{"convert", "-O", qcow2Format, src, dst}

	if out, err := exec.CommandContext(ctx, "qemu-img", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("qemu-img convert %q %q: %w: %s", src, dst, err, out)
	}

	return nil
}

// qemuImgResize grows the virtual size of dst to sizeBytes. For qcow2 overlays this changes only
// the overlay's virtual size without touching the backing file; the on-disk cost is proportional
// to subsequent writes, not to sizeBytes.
func qemuImgResize(ctx context.Context, dst string, sizeBytes uint64) error {
	args := []string{"resize", dst, strconv.FormatUint(sizeBytes, 10)}

	if out, err := exec.CommandContext(ctx, "qemu-img", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("qemu-img resize %q: %w: %s", dst, err, out)
	}

	return nil
}

// qemuImgInfo is the subset of `qemu-img info` output the controller reads.
type qemuImgInfo struct {
	Format      string `json:"format"`
	VirtualSize uint64 `json:"virtual-size"`
}

// qemuImgInspect reports what qemu-img sees for src: its format (e.g. "qcow2" or "raw") and the
// virtual size it presents to a guest.
func qemuImgInspect(ctx context.Context, src string) (qemuImgInfo, error) {
	out, err := exec.CommandContext(ctx, "qemu-img", "info", "--output=json", src).Output()
	if err != nil {
		return qemuImgInfo{}, fmt.Errorf("qemu-img info %q: %w", src, err)
	}

	var info qemuImgInfo

	if err := json.Unmarshal(out, &info); err != nil {
		return qemuImgInfo{}, fmt.Errorf("qemu-img info %q: parse: %w", src, err)
	}

	return info, nil
}

// qemuImgDetectFormat returns the format qemu-img sees for src, e.g. "qcow2" or "raw". A linked
// overlay needs this because qemu-img demands the backing format match the file; -F guessed wrong
// (or omitted) is a startup refusal, not a late failure.
func qemuImgDetectFormat(ctx context.Context, src string) (string, error) {
	info, err := qemuImgInspect(ctx, src)
	if err != nil {
		return "", err
	}

	if info.Format == "" {
		return "", fmt.Errorf("qemu-img info %q: no format reported", src)
	}

	return info.Format, nil
}
