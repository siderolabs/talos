// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

import "fmt"

//go:generate go tool golang.org/x/tools/cmd/stringer -type=PeerState -linecomment

// PeerState is KubeSpan peer current state.
type PeerState int

// MarshalText implements encoding.TextMarshaler.
func (v PeerState) MarshalText() ([]byte, error) {
	return []byte(v.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (v *PeerState) UnmarshalText(b []byte) error {
	switch string(b) {
	case "unknown":
		*v = PeerStateUnknown
	case "up":
		*v = PeerStateUp
	case "down":
		*v = PeerStateDown
	default:
		return fmt.Errorf("unsupported value for PeerState: %q", string(b))
	}

	return nil
}

// PeerState constants.
//
//structprotogen:gen_enum
const (
	PeerStateUnknown PeerState = iota // unknown
	PeerStateUp                       // up
	PeerStateDown                     // down
)
