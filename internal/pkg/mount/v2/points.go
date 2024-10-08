// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"errors"
	"path/filepath"
	"slices"
)

// Points is a list of mount points.
type Points []*Point

// Mount all mount points.
func (points Points) Mount(opts ...OperationOption) (unmounter func() error, err error) {
	unmounters := make([]func() error, 0, len(points))

	for _, point := range points {
		unmounter, err := point.Mount(opts...)
		if err != nil {
			// unmount what got already mounted
			slices.Reverse(unmounters)

			for _, unmounter := range unmounters {
				_ = unmounter() //nolint:errcheck
			}

			return nil, err
		}

		unmounters = append(unmounters, unmounter)
	}

	slices.Reverse(unmounters)

	return func() error {
		var unmountErr error

		for _, unmounter := range unmounters {
			unmountErr = errors.Join(unmounter())
		}

		return unmountErr
	}, nil
}

// Unmount all mount points.
func (points Points) Unmount() error {
	for i := len(points) - 1; i >= 0; i-- {
		if err := points[i].Unmount(); err != nil {
			return err
		}
	}

	return nil
}

// Move all mount points to a new prefix.
func (points Points) Move(prefix string) error {
	for _, point := range points {
		if err := point.Move(filepath.Join(prefix, point.target)); err != nil {
			return err
		}
	}

	return nil
}
