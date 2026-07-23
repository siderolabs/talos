// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

// MDArrayPhase describes the provisioning/sync state of an MD array.
type MDArrayPhase int

// MD array phases.
//
//structprotogen:gen_enum
const (
	MDArrayPhaseUnknown    MDArrayPhase = iota // unknown
	MDArrayPhaseWaiting                        // waiting
	MDArrayPhaseRebuilding                     // rebuilding
	MDArrayPhaseReady                          // ready
	MDArrayPhaseError                          // error
)
