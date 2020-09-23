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

	valid "github.com/asaskevich/govalidator"
	talosnet "github.com/talos-systems/net"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
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
type NetworkDeviceCheck func(*Device) error

// Validate implements the Configurator interface.
//nolint: gocyclo
func (c *Config) Validate(mode config.RuntimeMode) error {
	var result *multierror.Error

	if c.MachineConfig == nil {
		result = multierror.Append(result, errors.New("machine instructions are required"))
	}

	if err := c.ClusterConfig.Validate(); err != nil {
		result = multierror.Append(result, err)
	}

	if mode.RequiresInstall() {
		if c.MachineConfig.MachineInstall == nil {
			result = multierror.Append(result, fmt.Errorf("install instructions are required in %q mode", mode))
		}

		if c.MachineConfig.MachineInstall.InstallDisk == "" {
			result = multierror.Append(result, fmt.Errorf("an install disk is required in %q mode", mode))
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

	if c.MachineConfig.MachineNetwork != nil {
		for _, device := range c.MachineConfig.MachineNetwork.NetworkInterfaces {
			if err := ValidateNetworkDevices(device, CheckDeviceInterface, CheckDeviceAddressing); err != nil {
				result = multierror.Append(result, err)
			}
		}
	}

	if c.MachineConfig.MachineDisks != nil {
		for _, disk := range c.MachineConfig.MachineDisks {
			for i, pt := range disk.DiskPartitions {
				if pt.DiskSize == 0 && i != len(disk.DiskPartitions)-1 {
					result = multierror.Append(result, fmt.Errorf("partition for disk %q is set to occupy full disk, but it's not the last partition in the list", disk.Device()))
				}
			}
		}
	}

	if !valid.IsDNSName(c.ClusterConfig.ClusterNetwork.DNSDomain) {
		result = multierror.Append(result, fmt.Errorf("%q is not a valid DNS name", c.ClusterConfig.ClusterNetwork.DNSDomain))
	}

	return result.ErrorOrNil()
}

// Validate validates the config.
func (c *ClusterConfig) Validate() error {
	var result *multierror.Error

	if c == nil {
		return fmt.Errorf("cluster instructions are required")
	}

	if c.ControlPlane == nil || c.ControlPlane.Endpoint == nil {
		return fmt.Errorf("cluster controlplane endpoint is required")
	}

	if err := talosnet.ValidateEndpointURI(c.ControlPlane.Endpoint.URL.String()); err != nil {
		result = multierror.Append(result, fmt.Errorf("invalid controlplane endpoint: %w", err))
	}

	return result.ErrorOrNil()
}

// ValidateNetworkDevices runs the specified validation checks specific to the
// network devices.
//nolint: dupl
func ValidateNetworkDevices(d *Device, checks ...NetworkDeviceCheck) error {
	var result *multierror.Error

	if d == nil {
		return fmt.Errorf("empty device")
	}

	if d.DeviceIgnore {
		return result.ErrorOrNil()
	}

	for _, check := range checks {
		result = multierror.Append(result, check(d))
	}

	return result.ErrorOrNil()
}

// CheckDeviceInterface ensures that the interface has been specified.
//nolint: dupl
func CheckDeviceInterface(d *Device) error {
	var result *multierror.Error

	if d == nil {
		return fmt.Errorf("empty device")
	}

	if d.DeviceInterface == "" {
		result = multierror.Append(result, fmt.Errorf("[%s]: %w", "networking.os.device.interface", ErrRequiredSection))
	}

	return result.ErrorOrNil()
}

// CheckDeviceAddressing ensures that an appropriate addressing method.
// has been specified
//nolint: dupl
func CheckDeviceAddressing(d *Device) error {
	var result *multierror.Error

	if d == nil {
		return fmt.Errorf("empty device")
	}

	// Test for both dhcp and cidr specified
	if d.DeviceDHCP && d.DeviceCIDR != "" {
		result = multierror.Append(result, fmt.Errorf("[%s] %q: %w", "networking.os.device", d.DeviceInterface, ErrBadAddressing))
	}

	// ensure cidr is a valid address
	if d.DeviceCIDR != "" {
		if _, _, err := net.ParseCIDR(d.DeviceCIDR); err != nil {
			result = multierror.Append(result, fmt.Errorf("[%s] %q: %w", "networking.os.device.CIDR", d.DeviceInterface, err))
		}
	}

	return result.ErrorOrNil()
}

// CheckDeviceRoutes ensures that the specified routes are valid.
//nolint: dupl
func CheckDeviceRoutes(d *Device) error {
	var result *multierror.Error

	if d == nil {
		return fmt.Errorf("empty device")
	}

	if len(d.DeviceRoutes) == 0 {
		return result.ErrorOrNil()
	}

	for idx, route := range d.DeviceRoutes {
		if _, _, err := net.ParseCIDR(route.Network()); err != nil {
			result = multierror.Append(result, fmt.Errorf("[%s] %q: %w", "networking.os.device.route["+strconv.Itoa(idx)+"].Network", route.Network(), ErrInvalidAddress))
		}

		if ip := net.ParseIP(route.Gateway()); ip == nil {
			result = multierror.Append(result, fmt.Errorf("[%s] %q: %w", "networking.os.device.route["+strconv.Itoa(idx)+"].Gateway", route.Gateway(), ErrInvalidAddress))
		}
	}

	return result.ErrorOrNil()
}
