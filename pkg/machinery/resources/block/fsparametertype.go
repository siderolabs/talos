// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

// FSParameterType describes Filesystem Parameter type.
type FSParameterType int

// NFS Version types.
//
//structprotogen:gen_enum
const (
	FSParameterTypeStringValue  FSParameterType = iota // string
	FSParameterTypeBooleanValue                        // boolean
	FSParameterTypeBinaryValue                         // binary
)
