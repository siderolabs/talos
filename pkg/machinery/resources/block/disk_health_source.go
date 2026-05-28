// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

// DiskHealthSource describes the source of disk health information.
type DiskHealthSource int

// Disk health sources.
//
//structprotogen:gen_enum
const (
	DiskHealthSourceUnknown     DiskHealthSource = iota // unknown
	DiskHealthSourceNVMe                                // nvme
	DiskHealthSourceATA                                 // ata
	DiskHealthSourceUnsupported                         // unsupported
)
