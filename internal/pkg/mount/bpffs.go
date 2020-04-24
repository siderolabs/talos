// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

// BPFMountPoints returns the bpf mount points.
func BPFMountPoints() (mountpoints *Points, err error) {
	base := "/sys/fs/bpf"
	bpf := NewMountPoints()
	bpf.Set("bpf", NewMountPoint("bpffs", base, "bpf", 0, ""))

	return bpf, nil
}
