// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

// MacvlanMode is a MACVLAN operating mode.
type MacvlanMode uint32

// MacvlanMode constants.
//
// See linux/if_link.h.
//
//structprotogen:gen_enum
const (
	MacvlanModePrivate  MacvlanMode = 0x1  // private
	MacvlanModeVEPA     MacvlanMode = 0x2  // vepa
	MacvlanModeBridge   MacvlanMode = 0x4  // bridge
	MacvlanModePassthru MacvlanMode = 0x8  // passthru
	MacvlanModeSource   MacvlanMode = 0x10 // source
)
