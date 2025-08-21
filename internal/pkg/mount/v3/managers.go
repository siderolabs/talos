// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"errors"
	"path/filepath"
	"slices"
)

// Managers is a list of mount managers.
type Managers []*Manager

// Mount all mount points managed by managers.
func (managers Managers) Mount() (func() error, error) {
	unmounters := make([]func() error, 0, len(managers))

	for _, manager := range managers {
		if _, err := manager.Mount(); err != nil {
			// unmount what got already mounted
			slices.Reverse(unmounters)

			for _, unmounter := range unmounters {
				if unmountErr := unmounter(); unmountErr != nil {
					err = errors.Join(err, unmountErr)
				}
			}

			return nil, err
		}

		unmounters = append(unmounters, manager.Unmount)
	}

	// unmount last to first
	slices.Reverse(unmounters)

	return func() error {
		var unmountErr error

		for _, unmounter := range unmounters {
			if err := unmounter(); err != nil {
				unmountErr = errors.Join(unmountErr, err)
			}
		}

		return unmountErr
	}, nil
}

// Unmount all mount points managed by managers.
func (managers Managers) Unmount() error {
	for i := len(managers) - 1; i >= 0; i-- {
		if err := managers[i].Unmount(); err != nil {
			return err
		}
	}

	return nil
}

// Move all mount managers to a new prefix.
func (managers Managers) Move(prefix string) error {
	for _, manager := range managers {
		if err := manager.Move(filepath.Join(prefix, manager.target)); err != nil {
			return err
		}
	}

	return nil
}
