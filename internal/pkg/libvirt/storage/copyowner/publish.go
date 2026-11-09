// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package copyowner keeps the on-disk ownership state of pool copies: hidden
// witnesses and markers, owner and binding records, in-flight stages, and the
// collection of private artifacts that no longer back a destination.
package copyowner

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Dir is the private ownership directory inside a pool target.
const Dir = ".copy-on-use"

// fileMode is the permission of every private ownership file.
const fileMode = 0o600

const rawFormat = "raw"

// ValidateName accepts a plain, non-hidden file name inside the pool directory.
func ValidateName(name string) error {
	if name == "" || filepath.Base(name) != name || name == "." || strings.HasPrefix(name, ".") {
		return fmt.Errorf("invalid copy volume name %q", name)
	}

	return nil
}

// Check reports whether name exists and is owned through witness. A present
// name without matching ownership is an error, never adopted.
func Check(root *os.Root, name, witness string) (bool, error) {
	file, err := root.Lstat(name)
	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	owned, err := root.Lstat(witness)
	if err != nil {
		return false, fmt.Errorf("copy volume %q has no ownership witness: %w", name, err)
	}

	if !file.Mode().IsRegular() || !owned.Mode().IsRegular() {
		return false, fmt.Errorf("copy volume %q exists without a matching ownership witness", name)
	}

	if strings.HasPrefix(witness, ".copy-on-use/image-") {
		if os.SameFile(file, owned) {
			return false, fmt.Errorf("copy volume %q has no writable ownership metadata", name)
		}

		return checkImageCopyOwnership(root, name, witness, file)
	}

	if !os.SameFile(file, owned) {
		return false, fmt.Errorf("copy volume %q exists without a matching ownership witness", name)
	}

	return true, nil
}

// Recover relinks an immutable copy's public name from its witness.
func Recover(root *os.Root, name, witness string) (bool, error) {
	owned, err := root.Lstat(witness)
	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if !owned.Mode().IsRegular() {
		return false, fmt.Errorf("copy volume %q has a nonregular ownership witness", name)
	}

	// A witness from an interrupted publication is not proven durable yet.
	if err := Sync(root, ".copy-on-use"); err != nil {
		return false, err
	}

	if err = root.Link(witness, name); err != nil {
		return false, fmt.Errorf("recover copy volume %q: %w", name, err)
	}

	return true, Sync(root, ".")
}

func checkImageCopyOwnership(root *os.Root, name, witness string, file os.FileInfo) (bool, error) {
	match, err := imageMarkerMatches(root, name, witness, file)
	if err != nil {
		return false, err
	}

	if !match {
		return false, fmt.Errorf("copy volume %q is not owned by its witness", name)
	}

	return true, nil
}

// ImageMarker binds the marker to the image's owner xattr, never to
// st_dev/st_ino: device numbers change across reboots (dm-crypt, LVM, enumeration).
func ImageMarker(format, owner string) string {
	return fmt.Sprintf("image-owner:%s:%s\n", format, owner)
}

// ReadImageMarker returns a writable marker's identity and format. A missing or
// unrecognized marker is reported as not valid.
func ReadImageMarker(root *os.Root, witness string) (string, string, bool, error) {
	before, err := root.Lstat(witness)
	if os.IsNotExist(err) {
		return "", "", false, nil
	}

	if err != nil {
		return "", "", false, err
	}

	if !before.Mode().IsRegular() || before.Size() > 128 {
		return "", "", false, nil
	}

	file, err := root.Open(witness)
	if err != nil {
		return "", "", false, err
	}

	opened, statErr := file.Stat()

	data, readErr := io.ReadAll(io.LimitReader(file, 129))
	if err := errors.Join(statErr, readErr, file.Close()); err != nil {
		return "", "", false, err
	}

	if !os.SameFile(before, opened) {
		return "", "", false, errors.New("image marker changed during read")
	}

	return parseImageMarker(data)
}

func parseImageMarker(data []byte) (string, string, bool, error) {
	representation, ok := strings.CutPrefix(string(data), "image-owner:")
	if !ok {
		return "", "", false, errors.New("decode image ownership: unknown marker")
	}

	format, owner, ok := strings.Cut(strings.TrimSuffix(representation, "\n"), ":")
	if !ok || !validImageOwner(owner) || (format != rawFormat && format != "qcow2") || string(data) != ImageMarker(format, owner) {
		return "", "", false, nil
	}

	return string(data), format, true, nil
}

func imageMarkerMatches(root *os.Root, name, witness string, info os.FileInfo) (bool, error) {
	file, err := root.Open(name)
	if err != nil {
		return false, err
	}
	defer file.Close() //nolint:errcheck

	opened, err := file.Stat()
	if err != nil {
		return false, err
	}

	if !os.SameFile(info, opened) {
		return false, errors.New("image changed during ownership check")
	}

	return ImageFileMatchesMarker(root, witness, file)
}

// Publish atomically links a staged copy under name after its ownership is durable.
// format is empty for immutable copies and the image format for writable ones.
func Publish(root *os.Root, staged, name, witness, format string) error {
	if format == "" {
		return publishCopy(root, staged, name, witness)
	}

	if _, err := root.Lstat(name); err == nil {
		return fmt.Errorf("publish copy volume %q: destination exists", name)
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := ClaimImageMarker(root, staged, name, witness, format); err != nil {
		return err
	}

	if err := Sync(root, ".copy-on-use"); err != nil {
		return err
	}

	if err := root.Link(staged, name); err != nil {
		return fmt.Errorf("publish image volume %q: %w", name, err)
	}

	return Sync(root, ".")
}

// ClaimImageMarker durably records staged's owner under witness, replacing only a known marker.
func ClaimImageMarker(root *os.Root, staged, name, witness, format string) error {
	// Only a known, unchanged metadata marker can be replaced. Keep the
	// previous marker through staging so a failed replacement remains collectible.
	previous, _, valid, err := ReadImageMarker(root, witness)
	if err != nil {
		return err
	}

	var previousInfo os.FileInfo
	if valid {
		previousInfo, err = root.Lstat(witness)
		if err != nil {
			return err
		}
	}

	owner, err := ClaimImageOwner(root, staged)
	if err != nil {
		return err
	}
	// Publish only a complete, synced metadata file under the ownership name.
	// A failed write or interrupted process must not leave an empty final marker
	// which prevents a later attempt after capacity or service recovery.
	markerStage := filepath.Join(".copy-on-use", filepath.Base(staged)+".owner")

	marker, err := root.OpenFile(markerStage, os.O_CREATE|os.O_EXCL|os.O_WRONLY, fileMode)
	if err != nil {
		return fmt.Errorf("stage image ownership for %q: %w", name, err)
	}

	_, writeErr := marker.WriteString(ImageMarker(format, owner))
	if writeErr == nil {
		writeErr = marker.Sync()
	}

	if err := errors.Join(writeErr, marker.Close()); err != nil {
		return errors.Join(err, root.Remove(markerStage))
	}

	if valid {
		return replaceImageMarker(root, witness, markerStage, previous, previousInfo)
	}

	linkErr := root.Link(markerStage, witness)
	if err := errors.Join(linkErr, root.Remove(markerStage)); err != nil {
		return fmt.Errorf("claim image volume %q: %w", name, err)
	}

	return nil
}

func replaceImageMarker(root *os.Root, witness, markerStage, previous string, previousInfo os.FileInfo) error {
	current, _, valid, err := ReadImageMarker(root, witness)
	if err != nil || !valid || current != previous {
		return errors.Join(err, root.Remove(markerStage), fmt.Errorf("image ownership marker %q changed during replacement", witness))
	}

	currentInfo, err := root.Lstat(witness)
	if err != nil || !os.SameFile(previousInfo, currentInfo) {
		return errors.Join(err, root.Remove(markerStage), fmt.Errorf("image ownership marker %q changed during replacement", witness))
	}

	if err := root.Rename(markerStage, witness); err != nil {
		return errors.Join(err, root.Remove(markerStage))
	}

	return nil
}

func publishCopy(root *os.Root, staged, name, witness string) error {
	if err := root.Link(staged, witness); err != nil {
		return fmt.Errorf("claim copy volume %q: %w", name, err)
	}

	if err := Sync(root, ".copy-on-use"); err != nil {
		return err
	}

	if err := root.Link(staged, name); err != nil {
		return fmt.Errorf("publish copy volume %q: %w", name, err)
	}

	return Sync(root, ".")
}

// Sync makes ownership and publication durable before reporting a usable copy.
func Sync(root *os.Root, name string) error {
	directory, err := root.Open(name)
	if err != nil {
		return err
	}

	return errors.Join(directory.Sync(), directory.Close())
}
