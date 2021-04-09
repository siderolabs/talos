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
	"strings"

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

// Validate implements the config.Provider interface.
//nolint:gocyclo,cyclop
func (c *Config) Validate(mode config.RuntimeMode, options ...config.ValidationOption) ([]string, error) {
	var (
		warnings []string
		result   *multierror.Error
	)

	opts := config.NewValidationOptions(options...)

	if c.MachineConfig == nil {
		result = multierror.Append(result, errors.New("machine instructions are required"))

		return nil, result.ErrorOrNil()
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

	// TODO rework machine type validation https://github.com/talos-systems/talos/issues/3413

	if c.MachineConfig.MachineType == "" {
		warnings = append(warnings, `machine type is empty`)
	}

	if c.Machine().Type() == machine.TypeInit || c.Machine().Type() == machine.TypeControlPlane {
		warn, err := ValidateCNI(c.Cluster().Network().CNI())
		warnings = append(warnings, warn...)
		result = multierror.Append(result, err)
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

	if opts.Strict {
		for _, w := range warnings {
			result = multierror.Append(result, fmt.Errorf("warning: %s", w))
		}

		warnings = nil
	}

	return warnings, result.ErrorOrNil()
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

	if c.ClusterNetwork != nil && !valid.IsDNSName(c.ClusterNetwork.DNSDomain) {
		result = multierror.Append(result, fmt.Errorf("%q is not a valid DNS name", c.ClusterNetwork.DNSDomain))
	}

	if ecp := c.ExternalCloudProviderConfig; ecp != nil {
		result = multierror.Append(result, ecp.Validate())
	}

	result = multierror.Append(result, c.ClusterInlineManifests.Validate())

	return result.ErrorOrNil()
}

// ValidateCNI validates CNI config.
func ValidateCNI(cni config.CNI) ([]string, error) {
	var (
		warnings []string
		result   *multierror.Error
	)

	switch cni.Name() {
	case constants.FlannelCNI:
		fallthrough
	case constants.NoneCNI:
		if len(cni.URLs()) != 0 {
			err := fmt.Errorf(`"urls" field should be empty for %q CNI`, cni.Name())
			result = multierror.Append(result, err)
		}

	case constants.CustomCNI:
		if len(cni.URLs()) == 0 {
			warn := fmt.Sprintf(`"urls" field should not be empty for %q CNI`, cni.Name())
			warnings = append(warnings, warn)
		}

		for _, u := range cni.URLs() {
			if err := talosnet.ValidateEndpointURI(u); err != nil {
				result = multierror.Append(result, err)
			}
		}

	default:
		err := fmt.Errorf("cni name should be one of [%q, %q, %q]", constants.FlannelCNI, constants.CustomCNI, constants.NoneCNI)
		result = multierror.Append(result, err)
	}

	return warnings, result.ErrorOrNil()
}

// Validate validates external cloud provider configuration.
func (ecp *ExternalCloudProviderConfig) Validate() error {
	if !ecp.ExternalEnabled && (len(ecp.ExternalManifests) != 0) {
		return fmt.Errorf("external cloud provider is disabled, but manifests are provided")
	}

	var result *multierror.Error

	for _, url := range ecp.ExternalManifests {
		if err := talosnet.ValidateEndpointURI(url); err != nil {
			err = fmt.Errorf("invalid external cloud provider manifest url %q: %w", url, err)
			result = multierror.Append(result, err)
		}
	}

	return result.ErrorOrNil()
}

// Validate the inline manifests.
func (manifests ClusterInlineManifests) Validate() error {
	var result *multierror.Error

	manifestNames := map[string]struct{}{}

	for _, manifest := range manifests {
		if strings.TrimSpace(manifest.InlineManifestName) == "" {
			result = multierror.Append(result, fmt.Errorf("inline manifest name can't be empty"))
		}

		if _, ok := manifestNames[manifest.InlineManifestName]; ok {
			result = multierror.Append(result, fmt.Errorf("inline manifest name %q is duplicate", manifest.InlineManifestName))
		}

		manifestNames[manifest.InlineManifestName] = struct{}{}
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
