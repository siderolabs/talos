// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nic

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/address"
	"github.com/talos-systems/talos/pkg/config/machine"
)

type NicSuite struct {
	suite.Suite
}

func TestNicSuite(t *testing.T) {
	suite.Run(t, new(NicSuite))
}

func (suite *NicSuite) TestIgnoreNic() {
	mynic, err := New(WithName("yolo"), WithIgnore())

	suite.Require().NoError(err)
	suite.Assert().True(mynic.IsIgnored())
}

func (suite *NicSuite) TestNoName() {
	_, err := New()
	suite.Require().Error(err)
}

func (suite *NicSuite) TestBond() {
	testSettings := [][]Option{
		{
			WithName("yolobond"),
			WithBond(true),
		},
		{
			WithName("yolobond"),
			WithBond(true),
			WithBondMode("balance-xor"),
		},
		{
			WithName("yolobond"),
			WithBond(true),
			WithBondMode("802.3ad"),
			WithHashPolicy("layer3+4"),
		},
		{
			WithName("yolobond"),
			WithBond(true),
			WithBondMode("balance-tlb"),
			WithHashPolicy("encap3+4"),
			WithLACPRate("fast"),
		},
		{
			WithName("yolobond"),
			WithBond(true),
			WithBondMode("balance-alb"),
			WithHashPolicy("encap2+3"),
			WithLACPRate("slow"),
			WithUpDelay(200),
		},
		{
			WithName("yolobond"),
			WithBond(true),
			WithBondMode("broadcast"),
			WithHashPolicy("layer2+3"),
			WithLACPRate("fast"),
			WithUpDelay(300),
			WithDownDelay(400),
			WithMIIMon(500),
		},
		{
			WithName("yolobond"),
			WithBond(true),
			WithBondMode("balance-rr"),
			WithHashPolicy("layer2"),
			WithLACPRate("slow"),
			WithUpDelay(300),
			WithDownDelay(400),
			WithMIIMon(500),
			WithSubInterface("lo", "lo"),
		},
		{
			WithName("yolobond"),
			WithBond(true),
			WithBondMode("active-backup"),
			WithHashPolicy("layer2"),
			WithLACPRate("slow"),
			WithUpDelay(300),
			WithDownDelay(400),
			WithMIIMon(500),
			WithSubInterface("lo", "lo"),
			WithAddressing(&address.Static{}),
		},
	}

	for _, test := range testSettings {
		mynic, err := New(test...)
		suite.Require().NoError(err)
		suite.Assert().True(mynic.Bonded)
	}
}

func (suite *NicSuite) TestVlan() {
	testSettings := [][]Option{
		{
			WithName("eth0"),
			WithVlan(100),
		},
		{
			WithName("eth0"),
			WithVlan(100),
			WithVlanCIDR(100, "172.21.10.101/28", []machine.Route{}),
		},
	}
	for _, test := range testSettings {
		mynic, err := New(test...)
		suite.Require().NoError(err)
		suite.Assert().True(len(mynic.Vlans) > 0)
	}
}
