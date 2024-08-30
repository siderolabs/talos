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
	// MetalNetworkPlatformConfig stores serialized NetworkPlatformConfig for the `metal` platform.
	MetalNetworkPlatformConfig
	// DownloadURLCode stores the value of the `${code}` variable in the download URL for talos.config= URL.
	DownloadURLCode
	// UserReserved1 is reserved for user-defined metadata.
	UserReserved1
	// UserReserved2 is reserved for user-defined metadata.
	UserReserved2
	// UserReserved3 is reserved for user-defined metadata.
	UserReserved3
	// UUIDOverride stores the UUID that this machine will use instead of the one from the hardware.
	UUIDOverride
	// UniqueMachineToken store the unique token for this machine. It's useful because UUID may repeat or be filled with zeros.
	UniqueMachineToken
)
