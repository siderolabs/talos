// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package controllers provides common methods for controller operations.
package controllers

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"reflect"

	yaml "gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/pkg/xfs"
)

// LoadOrNewFromFile either loads value from file.yaml or generates new values and saves as file.yaml.
//
//nolint:gocyclo
func LoadOrNewFromFile[T any](root xfs.Root, path string, empty T, generate func(T) error) error {
	f, err := xfs.OpenFile(root, path, os.O_RDONLY, 0)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("error reading state file %q: %w", path, err)
	}

	// file doesn't exist yet, generate new value and save it
	if f == nil || errors.Is(err, fs.ErrNotExist) {
		if err = generate(empty); err != nil {
			return err
		}

		f, err = xfs.OpenFile(root, path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
		if err != nil {
			return fmt.Errorf("error creating state file %q: %w", path, err)
		}

		defer f.Close() //nolint:errcheck

		encoder := yaml.NewEncoder(f)
		if err = encoder.Encode(empty); err != nil {
			return fmt.Errorf("error marshaling %q: %w", path, err)
		}

		if err = encoder.Close(); err != nil {
			return err
		}

		return f.Close()
	}

	// read existing cached value
	defer f.Close() //nolint:errcheck

	if err = yaml.NewDecoder(f).Decode(empty); err != nil {
		return fmt.Errorf("error unmarshaling %q: %w", path, err)
	}

	if reflect.ValueOf(empty).Elem().IsZero() {
		return fmt.Errorf("value of %q is still zero after unmarshaling", path)
	}

	return f.Close()
}
