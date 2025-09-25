// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Copyright The Monogon Project Authors.
// SPDX-License-Identifier: Apache-2.0

package efivarfs

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"regexp"
	"strconv"

	"github.com/google/uuid"
)

func decodeString(varData []byte) (string, error) {
	efiStringRaw, err := Encoding.NewDecoder().Bytes(varData)
	if err != nil {
		// Pass the decoding error unwrapped.
		return "", err
	}
	// Remove the null suffix.
	return string(bytes.TrimSuffix(efiStringRaw, []byte{0})), nil
}

// ReadLoaderDevicePartUUID reads the ESP UUID from an EFI variable.
func ReadLoaderDevicePartUUID(rw ReadWriter) (uuid.UUID, error) {
	efiVar, _, err := rw.Read(ScopeSystemd, "LoaderDevicePartUUID")
	if err != nil {
		return uuid.Nil, err
	}

	strContent, err := decodeString(efiVar)
	if err != nil {
		return uuid.Nil, fmt.Errorf("decoding string failed: %w", err)
	}

	out, err := uuid.Parse(strContent)
	if err != nil {
		return uuid.Nil, fmt.Errorf("value in LoaderDevicePartUUID could not be parsed as UUID: %w", err)
	}

	return out, nil
}

// Technically UEFI mandates that only upper-case hex indices are valid, but in
// practice even vendors themselves ship firmware with lowercase hex indices,
// thus accept these here as well.
var bootVarRegexp = regexp.MustCompile(`^Boot([0-9A-Fa-f]{4})$`)

// ListBootEntries lists all EFI boot entries present in the system by their index.
func ListBootEntries(rw ReadWriter) (map[int]*LoadOption, error) {
	bootEntries := make(map[int]*LoadOption)

	varNames, err := rw.List(ScopeGlobal)
	if err != nil {
		return nil, fmt.Errorf("failed to list EFI variables at scope %s: %w", ScopeGlobal, err)
	}

	for _, varName := range varNames {
		s := bootVarRegexp.FindStringSubmatch(varName)
		if s == nil {
			continue
		}

		idx, err := strconv.ParseUint(s[1], 16, 16)
		if err != nil {
			// This cannot be hit as all regexp matches are parseable.
			// A quick fuzz run agrees.
			panic(err)
		}

		entry, err := GetBootEntry(rw, int(idx))
		if err != nil {
			return nil, fmt.Errorf("failed to get boot entry %s: %w", varName, err)
		}

		bootEntries[int(idx)] = entry
	}

	return bootEntries, nil
}

// AddBootEntry creates an new EFI boot entry variable and returns its
// non-negative index on success.
func AddBootEntry(rw ReadWriter, be *LoadOption) (int, error) {
	bootEntries, err := ListBootEntries(rw)
	if err != nil {
		return -1, fmt.Errorf("failed to list boot entries: %w", err)
	}

	idx := -1

	for i := range math.MaxUint16 {
		if _, ok := bootEntries[i]; !ok {
			idx = i

			break
		}
	}

	if idx == -1 {
		return -1, errors.New("all 2^16 boot entry variables are occupied")
	}

	err = SetBootEntry(rw, idx, be)
	if err != nil {
		return -1, fmt.Errorf("failed to set new boot entry: %w", err)
	}

	return idx, nil
}

// GetBootEntry returns the boot entry at the given index.
func GetBootEntry(rw ReadWriter, idx int) (*LoadOption, error) {
	raw, _, err := rw.Read(ScopeGlobal, fmt.Sprintf("Boot%04X", idx))
	if errors.Is(err, fs.ErrNotExist) {
		// Try non-spec-conforming lowercase entry
		raw, _, err = rw.Read(ScopeGlobal, fmt.Sprintf("Boot%04x", idx))
	}

	if err != nil {
		return nil, err
	}

	return UnmarshalLoadOption(raw)
}

// SetBootEntry writes the given boot entry to the given index.
func SetBootEntry(rw ReadWriter, idx int, be *LoadOption) error {
	bem, err := be.Marshal()
	if err != nil {
		return fmt.Errorf("while marshaling the EFI boot entry: %w", err)
	}

	return rw.Write(ScopeGlobal, fmt.Sprintf("Boot%04X", idx), AttrNonVolatile|AttrRuntimeAccess, bem)
}

// DeleteBootEntry deletes the boot entry at the given index.
func DeleteBootEntry(rw ReadWriter, idx int) error {
	err := rw.Delete(ScopeGlobal, fmt.Sprintf("Boot%04X", idx))
	if errors.Is(err, fs.ErrNotExist) {
		// Try non-spec-conforming lowercase entry
		err = rw.Delete(ScopeGlobal, fmt.Sprintf("Boot%04x", idx))
	}

	return err
}

// SetBootOrder replaces contents of the boot order variable with the order
// specified in ord.
func SetBootOrder(rw ReadWriter, ord BootOrder) error {
	return rw.Write(ScopeGlobal, "BootOrder", AttrNonVolatile|AttrRuntimeAccess, ord.Marshal())
}

// GetBootOrder returns the current boot order of the system.
func GetBootOrder(rw ReadWriter) (BootOrder, error) {
	raw, _, err := rw.Read(ScopeGlobal, "BootOrder")
	if err != nil {
		return nil, err
	}

	ord, err := UnmarshalBootOrder(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid boot order structure: %w", err)
	}

	return ord, nil
}

// SetBootNext sets the boot entry used for the next boot only. It automatically
// resets after the next boot.
func SetBootNext(rw ReadWriter, entryIdx uint16) error {
	data := make([]byte, 2)
	binary.LittleEndian.PutUint16(data, entryIdx)

	return rw.Write(ScopeGlobal, "BootNext", AttrNonVolatile|AttrRuntimeAccess, data)
}
