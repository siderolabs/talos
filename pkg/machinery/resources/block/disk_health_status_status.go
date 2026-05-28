// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

// DiskHealthStatusValue describes the normalized health status of a disk.
type DiskHealthStatusValue int

// Disk health status values.
//
//structprotogen:gen_enum
const (
	DiskHealthStatusValueUnknown  DiskHealthStatusValue = iota // unknown
	DiskHealthStatusValueHealthy                               // healthy
	DiskHealthStatusValueWarning                               // warning
	DiskHealthStatusValueCritical                              // critical
)
