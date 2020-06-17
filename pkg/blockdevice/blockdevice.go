// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package blockdevice provides a library for working with block devices.
package blockdevice

import "errors"

// ErrMissingPartitionTable indicates that the the block device does not have a
// partition table.
var ErrMissingPartitionTable = errors.New("missing partition table")
