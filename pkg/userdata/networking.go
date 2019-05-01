/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"net"
	"strconv"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/xerrors"
)

// Device represents a network interface
type Device struct {
	Interface string  `yaml:"interface"`
	CIDR      string  `yaml:"cidr"`
	DHCP      bool    `yaml:"dhcp"`
	Routes    []Route `yaml:"routes"`
	Bond      *Bond   `yaml:"bond"`
}

type NetworkDeviceCheck func(*Device) error

func (d *Device) Validate(checks ...NetworkDeviceCheck) error {
	var result *multierror.Error

	for _, check := range checks {
		result = multierror.Append(result, check(d))
	}

	return result.ErrorOrNil()
}

func CheckDeviceInterface() NetworkDeviceCheck {
	return func(d *Device) error {
		var result *multierror.Error

		if d.Interface == "" {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "networking.os.device.interface", "", ErrRequiredSection))
		}

		return result.ErrorOrNil()
	}
}

func CheckDeviceAddressing() NetworkDeviceCheck {
	return func(d *Device) error {
		var result *multierror.Error

		if d.DHCP == true && d.CIDR != "" {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "networking.os.device", "", ErrBadAddressing))
		}

		if !d.DHCP && d.CIDR == "" {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "networking.os.device", "", ErrBadAddressing))
		}

		if d.CIDR != "" {
			if _, _, err := net.ParseCIDR(d.CIDR); err != nil {
				result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "networking.os.device.CIDR", "", err))
			}
		}

		return result.ErrorOrNil()
	}
}

func CheckDeviceRoutes() NetworkDeviceCheck {
	return func(d *Device) error {
		var result *multierror.Error

		if len(d.Routes) == 0 {
			return result.ErrorOrNil()
		}

		for idx, route := range d.Routes {
			if _, _, err := net.ParseCIDR(route.Network); err != nil {
				result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "networking.os.device.route["+strconv.Itoa(idx)+"].Network", route.Network, ErrInvalidAddress))

			}
			if ip := net.ParseIP(route.Gateway); ip == nil {
				result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "networking.os.device.route["+strconv.Itoa(idx)+"].Gateway", route.Gateway, ErrInvalidAddress))

			}
		}
		return result.ErrorOrNil()
	}
}

// Bond contains the various options for configuring a
// bonded interface
type Bond struct {
	Mode       string   `yaml:"mode"`
	HashPolicy string   `yaml:"hashpolicy"`
	LACPRate   string   `yaml:"lacprate"`
	Interfaces []string `yaml:"interfaces"`
}

// Route represents a network route
type Route struct {
	Network string `yaml:"network"`
	Gateway string `yaml:"gateway"`
}
