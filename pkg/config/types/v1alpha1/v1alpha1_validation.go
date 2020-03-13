// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package v1alpha1 provides user-facing v1alpha1 machine configs
//nolint: dupl
package v1alpha1

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/constants"
)

var (
	// General

	// ErrRequiredSection denotes a section is required
	ErrRequiredSection = errors.New("required config section")
	// ErrInvalidVersion denotes that the config file version is invalid
	ErrInvalidVersion = errors.New("invalid config version")

	// Security

	// ErrInvalidCert denotes that the certificate specified is invalid
	ErrInvalidCert = errors.New("certificate is invalid")
	// ErrInvalidCertType denotes that the certificate type is invalid
	ErrInvalidCertType = errors.New("certificate type is invalid")

	// Services

	// ErrUnsupportedCNI denotes that the specified CNI is invalid
	ErrUnsupportedCNI = errors.New("unsupported CNI driver")
	// ErrInvalidTrustdToken denotes that a trustd token has not been specified
	ErrInvalidTrustdToken = errors.New("trustd token is invalid")

	// Networking

	// ErrBadAddressing denotes that an incorrect combination of network
	// address methods have been specified
	ErrBadAddressing = errors.New("invalid network device addressing method")
	// ErrInvalidAddress denotes that a bad address was provided
	ErrInvalidAddress = errors.New("invalid network address")
)

// NetworkDeviceCheck defines the function type for checks.
//nolint: dupl
type NetworkDeviceCheck func(machine.Device) error

// Validate implements the Configurator interface.
//nolint: gocyclo
func (c *Config) Validate(mode runtime.Mode) error {
	var result *multierror.Error

	if c.MachineConfig == nil {
		result = multierror.Append(result, errors.New("machine instructions are required"))
	}

	if c.ClusterConfig == nil {
		result = multierror.Append(result, errors.New("cluster instructions are required"))
	}

	if c.Cluster().Endpoint() == nil || c.Cluster().Endpoint().String() == "" {
		result = multierror.Append(result, errors.New("a cluster endpoint is required"))
	}

	if mode == runtime.Metal {
		if c.MachineConfig.MachineInstall == nil {
			result = multierror.Append(result, fmt.Errorf("install instructions are required in %q mode", runtime.Metal.String()))
		}

		if c.MachineConfig.MachineInstall.InstallDisk == "" {
			result = multierror.Append(result, fmt.Errorf("an install disk is required in %q mode", runtime.Metal.String()))
		}

		if _, err := os.Stat(c.MachineConfig.MachineInstall.InstallDisk); os.IsNotExist(err) {
			result = multierror.Append(result, fmt.Errorf("specified install disk does not exist: %q", c.MachineConfig.MachineInstall.InstallDisk))
		}
	}

	if c.Machine().Type() == machine.TypeInit {
		switch c.Cluster().Network().CNI().Name() {
		case "custom":
			if len(c.Cluster().Network().CNI().URLs()) == 0 {
				result = multierror.Append(result, errors.New("at least one url should be specified if using \"custom\" option for CNI"))
			}
		case constants.DefaultCNI:
			// it's flannel bby
		default:
			result = multierror.Append(result, errors.New("cni name should be one of [custom,flannel]"))
		}
	}

	for _, device := range c.MachineConfig.MachineNetwork.NetworkInterfaces {
		if err := ValidateNetworkDevices(device, CheckDeviceInterface, CheckDeviceAddressing); err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result.ErrorOrNil()
}

// ValidateNetworkDevices runs the specified validation checks specific to the
// network devices.
//nolint: dupl
func ValidateNetworkDevices(d machine.Device, checks ...NetworkDeviceCheck) error {
	var result *multierror.Error

	if d.Ignore {
		return result.ErrorOrNil()
	}

	for _, check := range checks {
		result = multierror.Append(result, check(d))
	}

	return result.ErrorOrNil()
}

// CheckDeviceInterface ensures that the interface has been specified.
//nolint: dupl
func CheckDeviceInterface(d machine.Device) error {
	var result *multierror.Error

	if d.Interface == "" {
		result = multierror.Append(result, fmt.Errorf("[%s] %q: %w", "networking.os.device.interface", "", ErrRequiredSection))
	}

	return result.ErrorOrNil()
}

// CheckDeviceAddressing ensures that an appropriate addressing method.
// has been specified
//nolint: dupl
func CheckDeviceAddressing(d machine.Device) error {
	var result *multierror.Error

	// Test for both dhcp and cidr specified
	if d.DHCP && d.CIDR != "" {
		result = multierror.Append(result, fmt.Errorf("[%s] %q: %w", "networking.os.device", "", ErrBadAddressing))
	}

	// test for neither dhcp nor cidr specified
	if !d.DHCP && d.CIDR == "" && len(d.Vlans) == 0 {
		result = multierror.Append(result, fmt.Errorf("[%s] %q: %w", "networking.os.device", "", ErrBadAddressing))
	}

	// ensure cidr is a valid address
	if d.CIDR != "" {
		if _, _, err := net.ParseCIDR(d.CIDR); err != nil {
			result = multierror.Append(result, fmt.Errorf("[%s] %q: %w", "networking.os.device.CIDR", "", err))
		}
	}

	return result.ErrorOrNil()
}

// CheckDeviceRoutes ensures that the specified routes are valid.
//nolint: dupl
func CheckDeviceRoutes(d machine.Device) error {
	var result *multierror.Error

	if len(d.Routes) == 0 {
		return result.ErrorOrNil()
	}

	for idx, route := range d.Routes {
		if _, _, err := net.ParseCIDR(route.Network); err != nil {
			result = multierror.Append(result, fmt.Errorf("[%s] %q: %w", "networking.os.device.route["+strconv.Itoa(idx)+"].Network", route.Network, ErrInvalidAddress))
		}

		if ip := net.ParseIP(route.Gateway); ip == nil {
			result = multierror.Append(result, fmt.Errorf("[%s] %q: %w", "networking.os.device.route["+strconv.Itoa(idx)+"].Gateway", route.Gateway, ErrInvalidAddress))
		}
	}

	return result.ErrorOrNil()
}
