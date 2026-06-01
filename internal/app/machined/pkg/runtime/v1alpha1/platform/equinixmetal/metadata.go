// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package equinixmetal

// MetadataConfig holds equinixmetal metadata info.
type MetadataConfig struct {
	ID             string        `json:"id"`
	Hostname       string        `json:"hostname"`
	Plan           string        `json:"plan"`
	Metro          string        `json:"metro"`
	Facility       string        `json:"facility"`
	Network        Network       `json:"network"`
	BGPNeighbors   []BGPNeighbor `json:"bgp_neighbors"`
	PrivateSubnets []string      `json:"private_subnets"`
}
