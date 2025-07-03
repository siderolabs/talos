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
func ReadLoaderDevicePartUUID() (uuid.UUID, error) {
	efiVar, _, err := Read(ScopeSystemd, "LoaderDevicePartUUID")
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

// AddBootEntry creates an new EFI boot entry variable and returns its
// non-negative index on success.
func AddBootEntry(be *LoadOption) (int, error) {
	varNames, err := List(ScopeGlobal)
	if err != nil {
		return -1, fmt.Errorf("failed to list EFI variables: %w", err)
	}

	presentEntries := make(map[int]bool)
	// Technically these are sorted, but due to the lower/upper case issue
	// we cannot rely on this fact.
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

		presentEntries[int(idx)] = true
	}

	idx := -1

	for i := range math.MaxUint16 {
		if !presentEntries[i] {
			idx = i

			break
		}
	}

	if idx == -1 {
		return -1, errors.New("all 2^16 boot entry variables are occupied")
	}

	err = SetBootEntry(idx, be)
	if err != nil {
		return -1, fmt.Errorf("failed to set new boot entry: %w", err)
	}

	return idx, nil
}

// GetBootEntry returns the boot entry at the given index.
func GetBootEntry(idx int) (*LoadOption, error) {
	raw, _, err := Read(ScopeGlobal, fmt.Sprintf("Boot%04X", idx))
	if errors.Is(err, fs.ErrNotExist) {
		// Try non-spec-conforming lowercase entry
		raw, _, err = Read(ScopeGlobal, fmt.Sprintf("Boot%04x", idx))
	}

	if err != nil {
		return nil, err
	}

	return UnmarshalLoadOption(raw)
}

// SetBootEntry writes the given boot entry to the given index.
func SetBootEntry(idx int, be *LoadOption) error {
	bem, err := be.Marshal()
	if err != nil {
		return fmt.Errorf("while marshaling the EFI boot entry: %w", err)
	}

	return Write(ScopeGlobal, fmt.Sprintf("Boot%04X", idx), AttrNonVolatile|AttrRuntimeAccess, bem)
}

// DeleteBootEntry deletes the boot entry at the given index.
func DeleteBootEntry(idx int) error {
	err := Delete(ScopeGlobal, fmt.Sprintf("Boot%04X", idx))
	if errors.Is(err, fs.ErrNotExist) {
		// Try non-spec-conforming lowercase entry
		err = Delete(ScopeGlobal, fmt.Sprintf("Boot%04x", idx))
	}

	return err
}

// SetBootOrder replaces contents of the boot order variable with the order
// specified in ord.
func SetBootOrder(ord BootOrder) error {
	return Write(ScopeGlobal, "BootOrder", AttrNonVolatile|AttrRuntimeAccess, ord.Marshal())
}

// GetBootOrder returns the current boot order of the system.
func GetBootOrder() (BootOrder, error) {
	raw, _, err := Read(ScopeGlobal, "BootOrder")
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
func SetBootNext(entryIdx uint16) error {
	data := make([]byte, 2)
	binary.LittleEndian.PutUint16(data, entryIdx)

	return Write(ScopeGlobal, "BootNext", AttrNonVolatile|AttrRuntimeAccess, data)
}
