// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package adv provides common interfaces to access ADV data.
package adv

// ADV describes implementation which stores tag-value data.
type ADV interface {
	ReadTag(t uint8) (val string, ok bool)
	ReadTagBytes(t uint8) (val []byte, ok bool)
	SetTag(t uint8, val string) (ok bool)
	SetTagBytes(t uint8, val []byte) (ok bool)
	DeleteTag(t uint8) (ok bool)
	Bytes() ([]byte, error)
}

const (
	// End is the noop tag.
	End = iota
	// Bootonce is the bootonce tag.
	Bootonce
	// Menusave is the menusave tag.
	Menusave
	// Reserved1 is a reserved tag.
	Reserved1
	// Reserved2 is a reserved tag.
	Reserved2
	// Reserved3 is a reserved tag.
	Reserved3
	// Upgrade is the upgrade tag.
	Upgrade
	// StagedUpgradeImageRef stores image reference for staged upgrade.
	StagedUpgradeImageRef
	// StagedUpgradeInstallOptions stores JSON-serialized install.Options.
	StagedUpgradeInstallOptions
	// StateEncryptionConfig stores JSON-serialized v1alpha1.Encryption.
	StateEncryptionConfig
)
