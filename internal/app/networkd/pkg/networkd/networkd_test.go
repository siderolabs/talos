/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package networkd

import (
	"log"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/talos/pkg/userdata"
)

type NetworkdSuite struct {
	suite.Suite
}

func TestNetworkdSuite(t *testing.T) {
	// Hide all our state transition messages
	//log.SetOutput(ioutil.Discard)
	suite.Run(t, new(NetworkdSuite))
}

func (suite *NetworkdSuite) TestParse() {
	tests := []struct {
		UserData *userdata.UserData
	}{
		{
			UserData: nil,
		},
		{
			UserData: &userdata.UserData{},
		},
		{
			UserData: &userdata.UserData{
				Networking: &userdata.Networking{},
			},
		},
		{
			UserData: &userdata.UserData{
				Networking: &userdata.Networking{
					OS: &userdata.OSNet{},
				},
			},
		},
		{
			UserData: &userdata.UserData{
				Networking: &userdata.Networking{
					OS: &userdata.OSNet{
						Devices: []userdata.Device{
							userdata.Device{
								Interface: "lo",
								CIDR:      "10.0.0.1/32",
							},
						},
					},
				},
			},
		},
		{
			UserData: &userdata.UserData{
				Networking: &userdata.Networking{
					OS: &userdata.OSNet{
						Devices: []userdata.Device{
							userdata.Device{
								Interface: "lo",
								DHCP:      true,
							},
						},
					},
				},
			},
		},
		{
			UserData: &userdata.UserData{
				Networking: &userdata.Networking{
					OS: &userdata.OSNet{
						Devices: []userdata.Device{
							userdata.Device{
								Interface: "lo",
								DHCP:      true,
								Routes: []userdata.Route{
									userdata.Route{
										Network: "192.168.0.0/24",
										Gateway: "192.168.0.254",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		nwd, err := New()
		suite.Assert().NoError(err)

		err = nwd.Parse(test.UserData)
		suite.Assert().NoError(err)
		for _, dev := range nwd.Interfaces {
			log.Printf("%+v", dev)
		}
	}

	// suite.Assert().NoError(err)
}

func (suite *NetworkdSuite) TestParse() {

}
