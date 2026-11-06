// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ipmi

import "github.com/olekukonko/tablewriter/pkg/twwidth"

func init() {
	// go-ipmi depends on tablewriter (to format the output for the CLI), which preallocates
	// a global string width cache (~1 MiB of live heap) on init.
	//
	// Talos never renders tables via go-ipmi, so disable the cache to release the memory.
	twwidth.SetCacheCapacity(0)
}
