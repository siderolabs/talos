// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package copyowner

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/qemuimg"
)

// Witness names the private ownership entry for name: a hard link for immutable
// copies, or a metadata marker for writable images.
func Witness(name string, image bool) string {
	prefix := ""
	if image {
		prefix = "image-"
	}

	return fmt.Sprintf(".copy-on-use/%s%x", prefix, sha256.Sum256([]byte(name)))
}

const OwnerAttribute = "user.talos.image-owner"

// ClaimImageOwner binds ownership to the file itself, not reusable inode numbers.
// Guest block writes preserve this attribute; removing and recreating the file does not.
func ClaimImageOwner(root *os.Root, name string) (string, error) {
	file, err := root.OpenFile(name, os.O_RDWR|unix.O_NOFOLLOW, 0)
	if err != nil {
		return "", err
	}
	defer file.Close() //nolint:errcheck

	info, err := file.Stat()
	if err != nil {
		return "", err
	}

	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("image ownership requires a regular file: %q", name)
	}

	owner, valid, err := readImageOwner(file)
	if err != nil {
		return "", err
	}

	if !valid {
		owner = uuid.NewString()
		if err := unix.Fsetxattr(int(file.Fd()), OwnerAttribute, []byte(owner), unix.XATTR_CREATE); err != nil {
			if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.ENOSYS) {
				return "", &qemuimg.ValidationError{Err: fmt.Errorf("writable image ownership requires user extended attributes: %w", err)}
			}

			return "", fmt.Errorf("set writable image ownership attribute: %w", err)
		}
	}

	// The ownership token must be durable before its marker or public name.
	if err := file.Sync(); err != nil {
		return "", err
	}

	return owner, nil
}

func validImageOwner(owner string) bool {
	parsed, err := uuid.Parse(owner)

	return err == nil && parsed.String() == owner
}

func readImageOwner(file *os.File) (string, bool, error) {
	var data [36]byte

	n, err := unix.Fgetxattr(int(file.Fd()), OwnerAttribute, data[:])
	if errors.Is(err, unix.ENODATA) || errors.Is(err, unix.ERANGE) {
		return "", false, nil
	}

	if err != nil {
		return "", false, classifyImageOwnerReadError(fmt.Errorf("read image ownership: %w", err))
	}

	owner := string(data[:n])

	return owner, validImageOwner(owner), nil
}

func classifyImageOwnerReadError(err error) error {
	if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.ENOSYS) {
		return &qemuimg.ValidationError{Err: fmt.Errorf("writable image ownership requires user extended attributes: %w", err)}
	}

	return err
}

// ImageFileMatchesMarker reports whether file carries the owner recorded by the marker at witness.
func ImageFileMatchesMarker(root *os.Root, witness string, file *os.File) (bool, error) {
	identity, format, valid, err := ReadImageMarker(root, witness)
	if err != nil || !valid {
		return false, err
	}

	return ImageFileMatches(identity, format, file)
}

// ImageFileMatches reports whether file carries the owner in marker identity for format.
func ImageFileMatches(identity, format string, file *os.File) (bool, error) {
	owner, valid, err := readImageOwner(file)

	return valid && identity == ImageMarker(format, owner), err
}
