// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Copyright The Monogon Project Authors.
// SPDX-License-Identifier: Apache-2.0

// Package efivarfs provides functions to read and manipulate UEFI runtime
// variables. It uses Linux's efivarfs [1] to access the variables and all
// functions generally require that this is mounted at
// "/sys/firmware/efi/efivars".
//
// [1] https://www.kernel.org/doc/html/latest/filesystems/efivarfs.html
package efivarfs

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/g0rbe/go-chattr"
	"github.com/google/uuid"
	"golang.org/x/sys/unix"
	"golang.org/x/text/encoding/unicode"

	"github.com/siderolabs/talos/internal/pkg/mount/v3"
	"github.com/siderolabs/talos/pkg/xfs"
)

const (
	// Path is the path to the efivarfs mount point.
	Path = "/sys/firmware/efi/efivars"
)

var (
	// ScopeGlobal is the scope of variables defined by the EFI specification
	// itself.
	ScopeGlobal = uuid.MustParse("8be4df61-93ca-11d2-aa0d-00e098032b8c")
	// ScopeSystemd is the scope of variables defined by Systemd/bootspec.
	ScopeSystemd = uuid.MustParse("4a67b082-0a4c-41cf-b6c7-440b29bb8c4f")
)

// Encoding defines the Unicode encoding used by UEFI, which is UCS-2 Little
// Endian. For BMP characters UTF-16 is equivalent to UCS-2. See the UEFI
// Spec 2.9, Sections 33.2.6 and 1.8.1.
var Encoding = unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)

// Attribute contains a bitset of EFI variable attributes.
type Attribute uint32

const (
	// AttrNonVolatile is the attribute for non-volatile variables.
	// If set the value of the variable is is persistent across resets and
	// power cycles. Variables without this set cannot be created or modified
	// after UEFI boot services are terminated.
	AttrNonVolatile Attribute = 1 << iota
	// AttrBootserviceAccess is the attribute for variables that can be
	// accessed from UEFI boot services.
	AttrBootserviceAccess
	// AttrRuntimeAccess is the attribute for variables that can be accessed from
	// an operating system after UEFI boot services are terminated. Variables
	// setting this must also set AttrBootserviceAccess. This is automatically
	// taken care of by Write in this package.
	AttrRuntimeAccess
	// AttrHardwareErrorRecord is the attribute for variables that are used to
	// mark a variable as being a hardware error record. See UEFI 2.10 section
	// 8.2.8 for more information about this.
	AttrHardwareErrorRecord
	// AttrAuthenticatedWriteAccess is the attribute for variables that require
	// authenticated access to write.
	// Deprecated: should not be used for new variables.
	AttrAuthenticatedWriteAccess
	// AttrTimeBasedAuthenticatedWriteAccess is the attribute for variables
	// that require special authentication to write. These variables
	// cannot be written with this package.
	AttrTimeBasedAuthenticatedWriteAccess
	// AttrAppendWrite is the attribute for variables that can be appended to.
	// If set in a Write() call, tries to append the data instead of replacing
	// it completely.
	AttrAppendWrite
	// AttrEnhancedAuthenticatedAccess is the attribute for variables that
	// require special authentication to access and write. These variables
	// cannot be accessed with this package.
	AttrEnhancedAuthenticatedAccess
)

func varPath(scope uuid.UUID, varName string) string {
	return fmt.Sprintf("%s-%s", varName, scope.String())
}

// ReadWriter is an interface for reading and writing EFI variables.
type ReadWriter interface {
	Write(scope uuid.UUID, varName string, attrs Attribute, value []byte) error
	Delete(scope uuid.UUID, varName string) error
	Read(scope uuid.UUID, varName string) ([]byte, Attribute, error)
	List(scope uuid.UUID) ([]string, error)
}

// FilesystemReaderWriter implements ReaderWriter using the efivars Linux filesystem.
type FilesystemReaderWriter struct {
	write bool

	point *mount.Point
}

// NewFilesystemReaderWriter creates a new FilesystemReaderWriter.
func NewFilesystemReaderWriter(write bool) (*FilesystemReaderWriter, error) {
	fsReaderWriter := &FilesystemReaderWriter{
		write: write,
	}

	if write {
		point, err := mount.NewManager(
			mount.WithDetached(),
			mount.WithMountAttributes(unix.MOUNT_ATTR_NOSUID|unix.MOUNT_ATTR_NOEXEC|unix.MOUNT_ATTR_NODEV|unix.MOUNT_ATTR_RELATIME),
			mount.WithFsopen("efivarfs"),
		).Mount()
		if err != nil {
			return nil, fmt.Errorf("remounting efivarfs read-write: %w", err)
		}

		fsReaderWriter.point = point
	}

	return fsReaderWriter, nil
}

// Close unmounts efivarfs if the FilesystemReaderWriter was created with write
// access.
func (rw *FilesystemReaderWriter) Close() error {
	if rw.write {
		return rw.point.Unmount(mount.UnmountOptions{})
	}

	return nil
}

// Write writes the value of the named variable in the given scope.
func (rw *FilesystemReaderWriter) Write(scope uuid.UUID, varName string, attrs Attribute, value []byte) error {
	if !rw.write {
		return errors.New("efivarfs was opened read-only")
	}

	// Ref: https://docs.kernel.org/filesystems/efivarfs.html
	// Remove immutable attribute from the efivarfs file if it exists

	xfsFile, err := xfs.OpenFile(rw.point.Root(), varPath(scope, varName), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to open efivarfs file %q: %w", varPath(scope, varName), err)
	}

	defer xfsFile.Close() //nolint:errcheck

	f, err := xfs.AsOSFile(xfsFile, varPath(scope, varName))
	if err != nil {
		return fmt.Errorf("failed to convert efivarfs file %q to os.File: %w", varPath(scope, varName), err)
	}

	defer f.Close() //nolint:errcheck

	if err := chattr.UnsetAttr(xfsFile.(*os.File), chattr.FS_IMMUTABLE_FL); err != nil {
		return fmt.Errorf("failed to clear immutable attribute from efivarfs file %q: %w", varPath(scope, varName), err)
	}

	defer chattr.SetAttr(f, chattr.FS_IMMUTABLE_FL) //nolint:errcheck

	// Write attributes, see @linux//Documentation/filesystems:efivarfs.rst for format

	// Required by UEFI 2.10 Section 8.2.3:
	// Runtime access to a data variable implies boot service access. Attributes
	// that have EFI_VARIABLE_RUNTIME_ACCESS set must also have
	// EFI_VARIABLE_BOOTSERVICE_ACCESS set. The caller is responsible for
	// following this rule.
	if attrs&AttrRuntimeAccess != 0 {
		attrs |= AttrBootserviceAccess
	}
	// Linux wants everything in on write, so assemble an intermediate buffer
	buf := make([]byte, len(value)+4)
	binary.LittleEndian.PutUint32(buf[:4], uint32(attrs))
	copy(buf[4:], value)

	_, err = xfsFile.Write(buf)
	if err1 := xfsFile.Close(); err1 != nil && err == nil {
		err = err1
	}

	return err
}

// Read reads the value of the named variable in the given scope.
func (rw *FilesystemReaderWriter) Read(scope uuid.UUID, varName string) ([]byte, Attribute, error) {
	val, err := xfs.ReadFile(rw.point.Root(), varPath(scope, varName))
	if err != nil {
		e := err
		// Unwrap PathError here as we wrap our own parameter message around it
		var perr *fs.PathError
		if errors.As(err, &perr) {
			e = perr.Err
		}

		return nil, Attribute(0), fmt.Errorf("reading %q in scope %s: %w", varName, scope, e)
	}

	if len(val) < 4 {
		return nil, Attribute(0), fmt.Errorf("reading %q in scope %s: malformed, less than 4 bytes long", varName, scope)
	}

	return val[4:], Attribute(binary.LittleEndian.Uint32(val[:4])), nil
}

// List lists all variable names present for a given scope sorted by their names
// in Go's "native" string sort order.
func (rw *FilesystemReaderWriter) List(scope uuid.UUID) ([]string, error) {
	vars, err := os.ReadDir(Path)
	if err != nil {
		return nil, fmt.Errorf("failed to list variable directory: %w", err)
	}

	var outVarNames []string //nolint:prealloc

	suffix := fmt.Sprintf("-%v", scope)

	for _, v := range vars {
		if v.IsDir() {
			continue
		}

		if !strings.HasSuffix(v.Name(), suffix) {
			continue
		}

		outVarNames = append(outVarNames, strings.TrimSuffix(v.Name(), suffix))
	}

	return outVarNames, nil
}

// Delete deletes the given variable name in the given scope. Use with care,
// some firmware fails to boot if variables it uses are deleted.
func (rw *FilesystemReaderWriter) Delete(scope uuid.UUID, varName string) error {
	if !rw.write {
		return errors.New("efivarfs was opened read-only")
	}

	return os.Remove(varPath(scope, varName))
}

// UniqueBootOrder returns a copy of the given BootOrder with duplicate entries
// removed, preserving the order of first appearance.
func UniqueBootOrder(bootOrder BootOrder) BootOrder {
	seen := make(map[uint16]struct{}, len(bootOrder))
	j := 0

	for _, v := range bootOrder {
		if _, ok := seen[v]; ok {
			continue
		}

		seen[v] = struct{}{}
		bootOrder[j] = v
		j++
	}

	return bootOrder[:j]
}
