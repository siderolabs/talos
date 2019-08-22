/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// Package v1 provides user-facing v1 machine configs
// nolint: dupl
package v1

import (
	"net"
	"strconv"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/xerrors"
)

// Device represents a network interface
// nolint: dupl
type Device struct {
	Interface string  `yaml:"interface"`
	CIDR      string  `yaml:"cidr"`
	DHCP      bool    `yaml:"dhcp"`
	Routes    []Route `yaml:"routes"`
	Bond      *Bond   `yaml:"bond"`
	MTU       int     `yaml:"mtu"`
}

// NetworkDeviceCheck defines the function type for checks
// nolint: dupl
type NetworkDeviceCheck func(*Device) error

// Validate triggers the specified validation checks to run
// nolint: dupl
func (d *Device) Validate(checks ...NetworkDeviceCheck) error {
	var result *multierror.Error

	for _, check := range checks {
		result = multierror.Append(result, check(d))
	}

	return result.ErrorOrNil()
}

// CheckDeviceInterface ensures that the interface has been specified
// nolint: dupl
func CheckDeviceInterface() NetworkDeviceCheck {
	return func(d *Device) error {
		var result *multierror.Error

		if d.Interface == "" {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "networking.os.device.interface", "", ErrRequiredSection))
		}

		return result.ErrorOrNil()
	}
}

// CheckDeviceAddressing ensures that an appropriate addressing method
// has been specified
// nolint: dupl
func CheckDeviceAddressing() NetworkDeviceCheck {
	return func(d *Device) error {
		var result *multierror.Error

		// Test for both dhcp and cidr specified
		if d.DHCP && d.CIDR != "" {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "networking.os.device", "", ErrBadAddressing))
		}

		// test for neither dhcp nor cidr specified
		if !d.DHCP && d.CIDR == "" {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "networking.os.device", "", ErrBadAddressing))
		}

		// ensure cidr is a valid address
		if d.CIDR != "" {
			if _, _, err := net.ParseCIDR(d.CIDR); err != nil {
				result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "networking.os.device.CIDR", "", err))
			}
		}

		return result.ErrorOrNil()
	}
}

// CheckDeviceRoutes ensures that the specified routes are valid
// nolint: dupl
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
// nolint: dupl
type Bond struct {
	Mode       string   `yaml:"mode"`
	HashPolicy string   `yaml:"hashpolicy"`
	LACPRate   string   `yaml:"lacprate"`
	Interfaces []string `yaml:"interfaces"`
}

// Route represents a network route
// nolint: dupl
type Route struct {
	Network string `yaml:"network"`
	Gateway string `yaml:"gateway"`
}
