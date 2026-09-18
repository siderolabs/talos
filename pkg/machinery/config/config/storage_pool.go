// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

// StoragePoolConfig exposes a directory storage pool backed by a configured volume.
type StoragePoolConfig interface {
	NamedDocument
	StoragePoolConfigSignal()
	// VolumeName returns the runtime volume ID, including its u-, e-, or x- prefix.
	VolumeName() string
}
