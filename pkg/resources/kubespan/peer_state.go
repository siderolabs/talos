// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

//go:generate stringer -type=PeerState -linecomment

// PeerState is KubeSpan peer current state.
type PeerState int

// MarshalText implements encoding.TextMarshaler.
func (v PeerState) MarshalText() ([]byte, error) {
	return []byte(v.String()), nil
}

// PeerState constants.
const (
	PeerStateUnknown PeerState = iota // unknown
	PeerStateUp                       // up
	PeerStateDown                     // down
)
