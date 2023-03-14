// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package meta

const (
	// Upgrade is the upgrade tag.
	Upgrade = iota + 6
	// StagedUpgradeImageRef stores image reference for staged upgrade.
	StagedUpgradeImageRef
	// StagedUpgradeInstallOptions stores JSON-serialized install.Options.
	StagedUpgradeInstallOptions
	// StateEncryptionConfig stores JSON-serialized v1alpha1.Encryption.
	StateEncryptionConfig
)
