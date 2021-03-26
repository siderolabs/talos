// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package v1alpha1 provides user-facing v1alpha1 machine configs
package v1alpha1

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"

	valid "github.com/asaskevich/govalidator"
	"github.com/hashicorp/go-multierror"
	talosnet "github.com/talos-systems/net"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

var (
	// General.

	// ErrRequiredSection denotes a section is required.
	ErrRequiredSection = errors.New("required config section")
	// ErrInvalidVersion denotes that the config file version is invalid.
	ErrInvalidVersion = errors.New("invalid config version")

	// Security.

	// ErrInvalidCert denotes that the certificate specified is invalid.
	ErrInvalidCert = errors.New("certificate is invalid")
	// ErrInvalidCertType denotes that the certificate type is invalid.
	ErrInvalidCertType = errors.New("certificate type is invalid")

	// Services.

	// ErrUnsupportedCNI denotes that the specified CNI is invalid.
	ErrUnsupportedCNI = errors.New("unsupported CNI driver")
	// ErrInvalidTrustdToken denotes that a trustd token has not been specified.
	ErrInvalidTrustdToken = errors.New("trustd token is invalid")

	// Networking.

	// ErrBadAddressing denotes that an incorrect combination of network
	// address methods have been specified.
	ErrBadAddressing = errors.New("invalid network device addressing method")
	// ErrInvalidAddress denotes that a bad address was provided.
	ErrInvalidAddress = errors.New("invalid network address")
)

// NetworkDeviceCheck defines the function type for checks.
type NetworkDeviceCheck func(*Device) error

// Validate implements the Configurator interface.
//nolint:gocyclo,cyclop
func (c *Config) Validate(mode config.RuntimeMode, options ...config.ValidationOption) error {
	var result *multierror.Error

	opts := config.NewValidationOptions(options...)

	if c.MachineConfig == nil {
		result = multierror.Append(result, errors.New("machine instructions are required"))
	}

	if err := c.ClusterConfig.Validate(); err != nil {
		result = multierror.Append(result, err)
	}

	if mode.RequiresInstall() {
		if c.MachineConfig.MachineInstall == nil {
			result = multierror.Append(result, fmt.Errorf("install instructions are required in %q mode", mode))
		} else {
			if opts.Local {
				if c.MachineConfig.MachineInstall.InstallDisk == "" && len(c.MachineConfig.MachineInstall.DiskMatchers()) == 0 {
					result = multierror.Append(result, fmt.Errorf("either install disk or diskSelector should be defined"))
				}
			} else {
				disk, err := c.MachineConfig.MachineInstall.Disk()

				if err != nil {
					result = multierror.Append(result, err)
				} else {
					if disk == "" {
						result = multierror.Append(result, fmt.Errorf("an install disk is required in %q mode", mode))
					}

					if _, err := os.Stat(disk); os.IsNotExist(err) {
						result = multierror.Append(result, fmt.Errorf("specified install disk does not exist: %q", c.MachineConfig.MachineInstall.InstallDisk))
					}
				}
			}
		}
	}

	if c.Machine().Type() == machine.TypeInit || c.Machine().Type() == machine.TypeControlPlane {
		switch c.Cluster().Network().CNI().Name() {
		case constants.CustomCNI:
			// custom CNI with URLs or an empty list of manifests which will get applied
		case constants.DefaultCNI:
			// it's flannel bby
		default:
			result = multierror.Append(result, errors.New("cni name should be one of [custom,flannel]"))
		}
	}

	if c.Machine().Type() == machine.TypeJoin {
		for _, d := range c.Machine().Network().Devices() {
			if d.VIPConfig() != nil {
				result = multierror.Append(result, errors.New("virtual (shared) IP is not allowed on non-controlplane nodes"))
			}
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

	for _, label := range []string{constants.EphemeralPartitionLabel, constants.StatePartitionLabel} {
		encryptionConfig := c.MachineConfig.SystemDiskEncryption().Get(label)
		if encryptionConfig != nil {
			if len(encryptionConfig.Keys()) == 0 {
				result = multierror.Append(result, fmt.Errorf("no encryption keys provided for the ephemeral partition encryption"))
			}

			slotsInUse := map[int]bool{}
			for _, key := range encryptionConfig.Keys() {
				if slotsInUse[key.Slot()] {
					result = multierror.Append(result, fmt.Errorf("encryption key slot %d is already in use", key.Slot()))
				}

				slotsInUse[key.Slot()] = true

				if key.NodeID() == nil && key.Static() == nil {
					result = multierror.Append(result, fmt.Errorf("encryption key at slot %d doesn't have any settings", key.Slot()))
				}
			}
		}
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
// has been specified.
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

	// check VIP IP is valid
	if d.DeviceVIPConfig != nil {
		if ip := net.ParseIP(d.DeviceVIPConfig.IP()); ip == nil {
			result = multierror.Append(result, fmt.Errorf("[%s] failed to parse %q as IP address", "networking.os.device.vip", d.DeviceVIPConfig.IP()))
		}
	}

	return result.ErrorOrNil()
}

// CheckDeviceRoutes ensures that the specified routes are valid.
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
