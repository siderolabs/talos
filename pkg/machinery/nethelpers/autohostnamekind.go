// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

// AutoHostnameKind is a kind of automatically generated hostname.
type AutoHostnameKind byte

// AutoHostnameKind constants.
//
// Note: AutoHostnameKindAddr is a legacy setting for backwards compatibility, and
// it is no longer exposed in network multi-doc configuration.
//
//structprotogen:gen_enum
const (
	AutoHostnameKindOff    AutoHostnameKind = iota // off
	AutoHostnameKindAddr                           // talos-addr
	AutoHostnameKindStable                         // stable
)
