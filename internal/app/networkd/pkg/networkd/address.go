/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package networkd

import (
	"errors"
	"net"

	"github.com/talos-systems/talos/pkg/userdata"
)

type AddressConfig interface {
	Addressing() error
}

func NewAddress(dev userdata.Device) (AddressConfig, error) {
	switch {
	case dev.DHCP:
		return &DHCPConfig{}, nil
	case dev.CIDR != "":
		ip, ipnet, err := net.ParseCIDR(dev.CIDR)
		if err != nil {
			return nil, err
		}
		return &StaticConfig{
			IP:    ip,
			IPNet: ipnet,
		}, nil
	}

	return nil, errors.New("unsupported network addressing method")
}

type DHCPConfig struct {
}

func (d *DHCPConfig) Addressing() (err error) {

	return err
}

type StaticConfig struct {
	IP    net.IP
	IPNet *net.IPNet
}

func (s *StaticConfig) Addressing() (err error) {

	return err
}
