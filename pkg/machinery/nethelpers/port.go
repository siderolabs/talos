// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import (
	"fmt"
	"strconv"

	"github.com/mdlayher/ethtool"
)

// Port wraps ethtool.Port for YAML marshaling.
type Port ethtool.Port

// MarshalYAML implements yaml.Marshaler interface.
func (port Port) MarshalYAML() (interface{}, error) {
	return ethtool.Port(port).String(), nil
}

// PortRangeSingle returns a PortRange composed of just a single port.
func PortRangeSingle(port uint32) PortRange {
	return PortRange{
		From: port,
		To:   port,
	}
}

// PortRange defines a TCP/UDP port range.
type PortRange struct {
	From uint32 `yaml:"from"`
	To   uint32 `yaml:"to"`
}

func (pr *PortRange) String() string {
	if pr.From == 0 && pr.To == 0 {
		return ""
	}

	if pr.From == pr.To {
		return strconv.Itoa(int(pr.From))
	}

	return fmt.Sprintf("%d-%d", pr.From, pr.To)
}
