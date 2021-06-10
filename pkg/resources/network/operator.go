// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//go:generate stringer -type=Operator -linecomment

// Operator enumerates Talos network operators.
type Operator int

// Operator list.
const (
	OperatorDHCP4 Operator = iota // dhcp4
	OperatorDHCP6                 // dhcp6
	OperatorVIP                   // vip
	OperatorWgLAN                 // wglan
)

// MarshalYAML implements yaml.Marshaler.
func (operator Operator) MarshalYAML() (interface{}, error) {
	return operator.String(), nil
}
