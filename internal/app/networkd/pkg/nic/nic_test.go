// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nic_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/address"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/nic"
	"github.com/talos-systems/talos/pkg/machinery/config"
)

type NicSuite struct {
	suite.Suite
}

func TestNicSuite(t *testing.T) {
	suite.Run(t, new(NicSuite))
}

func (suite *NicSuite) TestIgnoreNic() {
	mynic, err := nic.New(nic.WithName("yolo"), nic.WithIgnore())

	suite.Require().NoError(err)
	suite.Assert().True(mynic.IsIgnored())
}

func (suite *NicSuite) TestNoName() {
	_, err := nic.New()
	suite.Require().Error(err)
}

func (suite *NicSuite) TestBond() {
	testSettings := [][]nic.Option{
		{
			nic.WithName("yolobond"),
			nic.WithBond(true),
		},
		{
			nic.WithName("yolobond"),
			nic.WithBond(true),
			nic.WithBondMode("balance-xor"),
		},
		{
			nic.WithName("yolobond"),
			nic.WithBond(true),
			nic.WithBondMode("802.3ad"),
			nic.WithHashPolicy("layer3+4"),
		},
		{
			nic.WithName("yolobond"),
			nic.WithBond(true),
			nic.WithBondMode("balance-tlb"),
			nic.WithHashPolicy("encap3+4"),
			nic.WithLACPRate("fast"),
		},
		{
			nic.WithName("yolobond"),
			nic.WithBond(true),
			nic.WithBondMode("balance-alb"),
			nic.WithHashPolicy("encap2+3"),
			nic.WithLACPRate("slow"),
			nic.WithUpDelay(200),
		},
		{
			nic.WithName("yolobond"),
			nic.WithBond(true),
			nic.WithBondMode("broadcast"),
			nic.WithHashPolicy("layer2+3"),
			nic.WithLACPRate("fast"),
			nic.WithUpDelay(300),
			nic.WithDownDelay(400),
			nic.WithMIIMon(500),
		},
		{
			nic.WithName("yolobond"),
			nic.WithBond(true),
			nic.WithBondMode("balance-rr"),
			nic.WithHashPolicy("layer2"),
			nic.WithLACPRate("slow"),
			nic.WithUpDelay(300),
			nic.WithDownDelay(400),
			nic.WithMIIMon(500),
			nic.WithSubInterface("lo", "lo"),
		},
		{
			nic.WithName("yolobond"),
			nic.WithBond(true),
			nic.WithBondMode("active-backup"),
			nic.WithHashPolicy("layer2"),
			nic.WithLACPRate("slow"),
			nic.WithUpDelay(300),
			nic.WithDownDelay(400),
			nic.WithMIIMon(500),
			nic.WithSubInterface("lo", "lo"),
			nic.WithAddressing(&address.Static{}),
		},
	}

	for _, test := range testSettings {
		mynic, err := nic.New(test...)
		suite.Require().NoError(err)
		suite.Assert().True(mynic.Bonded)
	}
}

func (suite *NicSuite) TestVlan() {
	testSettings := [][]nic.Option{
		{
			nic.WithName("eth0"),
			nic.WithVlan(100),
		},
		{
			nic.WithName("eth0"),
			nic.WithVlan(100),
			nic.WithVlanCIDR(100, "172.21.10.101/28", []config.Route{}),
		},
	}
	for _, test := range testSettings {
		mynic, err := nic.New(test...)
		suite.Require().NoError(err)
		suite.Assert().True(len(mynic.Vlans) > 0)
	}
}
