// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package controllers provides common methods for controller operations.
package controllers

import (
	"errors"
	"fmt"
	"os"
	"reflect"

	yaml "gopkg.in/yaml.v3"
)

// LoadOrNewFromFile either loads value from file.yaml or generates new values and saves as file.yaml.
func LoadOrNewFromFile(path string, empty any, generate func(any) error) error {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error reading state file: %w", err)
	}

	// file doesn't exist yet, generate new value and save it
	if f == nil {
		if err = generate(empty); err != nil {
			return err
		}

		f, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
		if err != nil {
			return fmt.Errorf("error creating state file: %w", err)
		}

		defer f.Close() //nolint:errcheck

		encoder := yaml.NewEncoder(f)
		if err = encoder.Encode(empty); err != nil {
			return fmt.Errorf("error marshaling: %w", err)
		}

		if err = encoder.Close(); err != nil {
			return err
		}

		return f.Close()
	}

	// read existing cached value
	defer f.Close() //nolint:errcheck

	if err = yaml.NewDecoder(f).Decode(empty); err != nil {
		return fmt.Errorf("error unmarshaling: %w", err)
	}

	if reflect.ValueOf(empty).Elem().IsZero() {
		return errors.New("value is still zero after unmarshaling")
	}

	return f.Close()
}
