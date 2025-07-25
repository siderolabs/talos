// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Code generated by "deep-copy -type DeviceSpec -type DiscoveredVolumeSpec -type DiscoveryRefreshRequestSpec -type DiscoveryRefreshStatusSpec -type DiskSpec -type MountRequestSpec -type MountStatusSpec -type SwapStatusSpec -type SymlinkSpec -type SystemDiskSpec -type UserDiskConfigStatusSpec -type VolumeConfigSpec -type VolumeLifecycleSpec -type VolumeMountRequestSpec -type VolumeMountStatusSpec -type VolumeStatusSpec -type ZswapStatusSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go ."; DO NOT EDIT.

package block

// DeepCopy generates a deep copy of DeviceSpec.
func (o DeviceSpec) DeepCopy() DeviceSpec {
	var cp DeviceSpec = o
	if o.Secondaries != nil {
		cp.Secondaries = make([]string, len(o.Secondaries))
		copy(cp.Secondaries, o.Secondaries)
	}
	return cp
}

// DeepCopy generates a deep copy of DiscoveredVolumeSpec.
func (o DiscoveredVolumeSpec) DeepCopy() DiscoveredVolumeSpec {
	var cp DiscoveredVolumeSpec = o
	return cp
}

// DeepCopy generates a deep copy of DiscoveryRefreshRequestSpec.
func (o DiscoveryRefreshRequestSpec) DeepCopy() DiscoveryRefreshRequestSpec {
	var cp DiscoveryRefreshRequestSpec = o
	return cp
}

// DeepCopy generates a deep copy of DiscoveryRefreshStatusSpec.
func (o DiscoveryRefreshStatusSpec) DeepCopy() DiscoveryRefreshStatusSpec {
	var cp DiscoveryRefreshStatusSpec = o
	return cp
}

// DeepCopy generates a deep copy of DiskSpec.
func (o DiskSpec) DeepCopy() DiskSpec {
	var cp DiskSpec = o
	if o.SecondaryDisks != nil {
		cp.SecondaryDisks = make([]string, len(o.SecondaryDisks))
		copy(cp.SecondaryDisks, o.SecondaryDisks)
	}
	if o.Symlinks != nil {
		cp.Symlinks = make([]string, len(o.Symlinks))
		copy(cp.Symlinks, o.Symlinks)
	}
	return cp
}

// DeepCopy generates a deep copy of MountRequestSpec.
func (o MountRequestSpec) DeepCopy() MountRequestSpec {
	var cp MountRequestSpec = o
	if o.Requesters != nil {
		cp.Requesters = make([]string, len(o.Requesters))
		copy(cp.Requesters, o.Requesters)
	}
	if o.RequesterIDs != nil {
		cp.RequesterIDs = make([]string, len(o.RequesterIDs))
		copy(cp.RequesterIDs, o.RequesterIDs)
	}
	return cp
}

// DeepCopy generates a deep copy of MountStatusSpec.
func (o MountStatusSpec) DeepCopy() MountStatusSpec {
	var cp MountStatusSpec = o
	cp.Spec = o.Spec.DeepCopy()
	return cp
}

// DeepCopy generates a deep copy of SwapStatusSpec.
func (o SwapStatusSpec) DeepCopy() SwapStatusSpec {
	var cp SwapStatusSpec = o
	return cp
}

// DeepCopy generates a deep copy of SymlinkSpec.
func (o SymlinkSpec) DeepCopy() SymlinkSpec {
	var cp SymlinkSpec = o
	if o.Paths != nil {
		cp.Paths = make([]string, len(o.Paths))
		copy(cp.Paths, o.Paths)
	}
	return cp
}

// DeepCopy generates a deep copy of SystemDiskSpec.
func (o SystemDiskSpec) DeepCopy() SystemDiskSpec {
	var cp SystemDiskSpec = o
	return cp
}

// DeepCopy generates a deep copy of UserDiskConfigStatusSpec.
func (o UserDiskConfigStatusSpec) DeepCopy() UserDiskConfigStatusSpec {
	var cp UserDiskConfigStatusSpec = o
	return cp
}

// DeepCopy generates a deep copy of VolumeConfigSpec.
func (o VolumeConfigSpec) DeepCopy() VolumeConfigSpec {
	var cp VolumeConfigSpec = o
	if o.Encryption.Keys != nil {
		cp.Encryption.Keys = make([]EncryptionKey, len(o.Encryption.Keys))
		copy(cp.Encryption.Keys, o.Encryption.Keys)
		for i3 := range o.Encryption.Keys {
			if o.Encryption.Keys[i3].StaticPassphrase != nil {
				cp.Encryption.Keys[i3].StaticPassphrase = make([]byte, len(o.Encryption.Keys[i3].StaticPassphrase))
				copy(cp.Encryption.Keys[i3].StaticPassphrase, o.Encryption.Keys[i3].StaticPassphrase)
			}
		}
	}
	if o.Encryption.PerfOptions != nil {
		cp.Encryption.PerfOptions = make([]string, len(o.Encryption.PerfOptions))
		copy(cp.Encryption.PerfOptions, o.Encryption.PerfOptions)
	}
	return cp
}

// DeepCopy generates a deep copy of VolumeLifecycleSpec.
func (o VolumeLifecycleSpec) DeepCopy() VolumeLifecycleSpec {
	var cp VolumeLifecycleSpec = o
	return cp
}

// DeepCopy generates a deep copy of VolumeMountRequestSpec.
func (o VolumeMountRequestSpec) DeepCopy() VolumeMountRequestSpec {
	var cp VolumeMountRequestSpec = o
	return cp
}

// DeepCopy generates a deep copy of VolumeMountStatusSpec.
func (o VolumeMountStatusSpec) DeepCopy() VolumeMountStatusSpec {
	var cp VolumeMountStatusSpec = o
	return cp
}

// DeepCopy generates a deep copy of VolumeStatusSpec.
func (o VolumeStatusSpec) DeepCopy() VolumeStatusSpec {
	var cp VolumeStatusSpec = o
	if o.EncryptionFailedSyncs != nil {
		cp.EncryptionFailedSyncs = make([]string, len(o.EncryptionFailedSyncs))
		copy(cp.EncryptionFailedSyncs, o.EncryptionFailedSyncs)
	}
	if o.ConfiguredEncryptionKeys != nil {
		cp.ConfiguredEncryptionKeys = make([]string, len(o.ConfiguredEncryptionKeys))
		copy(cp.ConfiguredEncryptionKeys, o.ConfiguredEncryptionKeys)
	}
	return cp
}

// DeepCopy generates a deep copy of ZswapStatusSpec.
func (o ZswapStatusSpec) DeepCopy() ZswapStatusSpec {
	var cp ZswapStatusSpec = o
	return cp
}
