// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package efivarfs

import (
	"io/fs"
	"maps"
	"slices"

	"github.com/google/uuid"
)

// Mock is a mock implementation of ReaderWriter interface for testing purposes.
type Mock struct {
	Variables map[uuid.UUID]map[string]MockVariable
}

// MockVariable represents a mock EFI variable with its attributes and data.
type MockVariable struct {
	Attrs Attribute
	Data  []byte
}

// Write writes a variable to the given scope.
func (mock *Mock) Write(scope uuid.UUID, varName string, attrs Attribute, value []byte) error {
	if mock.Variables == nil {
		mock.Variables = make(map[uuid.UUID]map[string]MockVariable)
	}

	if mock.Variables[scope] == nil {
		mock.Variables[scope] = make(map[string]MockVariable)
	}

	mock.Variables[scope][varName] = MockVariable{
		Attrs: attrs,
		Data:  value,
	}

	return nil
}

// Delete deletes a variable from the given scope.
func (mock *Mock) Delete(scope uuid.UUID, varName string) error {
	if mock.Variables == nil || mock.Variables[scope] == nil {
		return fs.ErrNotExist
	}

	if _, exists := mock.Variables[scope][varName]; !exists {
		return fs.ErrNotExist
	}

	delete(mock.Variables[scope], varName)

	return nil
}

// Read reads a variable from the given scope.
func (mock *Mock) Read(scope uuid.UUID, varName string) ([]byte, Attribute, error) {
	if mock.Variables == nil || mock.Variables[scope] == nil {
		return nil, 0, fs.ErrNotExist
	}

	variable, exists := mock.Variables[scope][varName]
	if !exists {
		return nil, 0, fs.ErrNotExist
	}

	return variable.Data, variable.Attrs, nil
}

// List lists all variable names in the given scope.
func (mock *Mock) List(scope uuid.UUID) ([]string, error) {
	if mock.Variables == nil || mock.Variables[scope] == nil {
		return nil, nil
	}

	return slices.Collect(maps.Keys(mock.Variables[scope])), nil
}
