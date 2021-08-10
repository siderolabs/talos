// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"encoding/base64"
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
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
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

	// ErrInvalidAddress denotes that a bad address was provided.
	ErrInvalidAddress = errors.New("invalid network address")
)

// NetworkDeviceCheck defines the function type for checks.
type NetworkDeviceCheck func(*Device, map[string]string) ([]string, error)

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

	if t := c.Machine().Type(); t != machine.TypeUnknown && t.String() != c.MachineConfig.MachineType {
		warnings = append(warnings, fmt.Sprintf("use %q instead of %q for machine type", t.String(), c.MachineConfig.MachineType))
	}

	switch c.Machine().Type() { //nolint:exhaustive
	case machine.TypeInit, machine.TypeControlPlane:
		warn, err := ValidateCNI(c.Cluster().Network().CNI())
		warnings = append(warnings, warn...)
		result = multierror.Append(result, err)

	case machine.TypeWorker:
		for _, d := range c.Machine().Network().Devices() {
			if d.VIPConfig() != nil {
				result = multierror.Append(result, errors.New("virtual (shared) IP is not allowed on non-controlplane nodes"))
			}
		}

	default:
		result = multierror.Append(result, fmt.Errorf("unknown machine type %q", c.MachineConfig.MachineType))
	}

	if c.MachineConfig.MachineNetwork != nil {
		bondedInterfaces := map[string]string{}

		for _, device := range c.MachineConfig.MachineNetwork.NetworkInterfaces {
			if device.Bond() != nil {
				for _, iface := range device.Bond().Interfaces() {
					if otherIface, exists := bondedInterfaces[iface]; exists && otherIface != device.Interface() {
						result = multierror.Append(result, fmt.Errorf("interface %q is declared as part of two bonds: %q and %q", iface, otherIface, device.Interface()))
					}

					bondedInterfaces[iface] = device.Interface()
				}
			}
		}

		for _, device := range c.MachineConfig.MachineNetwork.NetworkInterfaces {
			warn, err := ValidateNetworkDevices(device, bondedInterfaces, CheckDeviceInterface, CheckDeviceAddressing, CheckDeviceRoutes)
			warnings = append(warnings, warn...)
			result = multierror.Append(result, err)
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
func ValidateNetworkDevices(d *Device, bondedInterfaces map[string]string, checks ...NetworkDeviceCheck) ([]string, error) {
	var result *multierror.Error

	if d == nil {
		return nil, fmt.Errorf("empty device")
	}

	if d.DeviceIgnore {
		return nil, result.ErrorOrNil()
	}

	var warnings []string

	for _, check := range checks {
		warn, err := check(d, bondedInterfaces)
		warnings = append(warnings, warn...)
		result = multierror.Append(result, err)
	}

	return warnings, result.ErrorOrNil()
}

// CheckDeviceInterface ensures that the interface has been specified.
func CheckDeviceInterface(d *Device, bondedInterfaces map[string]string) ([]string, error) {
	var result *multierror.Error

	if d == nil {
		return nil, fmt.Errorf("empty device")
	}

	if d.DeviceInterface == "" {
		result = multierror.Append(result, fmt.Errorf("[%s]: %w", "networking.os.device.interface", ErrRequiredSection))
	}

	if d.DeviceBond != nil {
		result = multierror.Append(result, checkBond(d.DeviceBond))
	}

	if d.DeviceWireguardConfig != nil {
		result = multierror.Append(result, checkWireguard(d.DeviceWireguardConfig))
	}

	if d.DeviceVlans != nil {
		result = multierror.Append(result, checkVlans(d))
	}

	return nil, result.ErrorOrNil()
}

//nolint:gocyclo,cyclop
func checkBond(b *Bond) error {
	var result *multierror.Error

	bondMode, err := nethelpers.BondModeByName(b.BondMode)
	if err != nil {
		result = multierror.Append(result, err)
	}

	_, err = nethelpers.BondXmitHashPolicyByName(b.BondHashPolicy)
	if err != nil {
		result = multierror.Append(result, err)
	}

	_, err = nethelpers.LACPRateByName(b.BondLACPRate)
	if err != nil {
		result = multierror.Append(result, err)
	}

	_, err = nethelpers.ARPValidateByName(b.BondARPValidate)
	if err != nil {
		result = multierror.Append(result, err)
	}

	_, err = nethelpers.ARPAllTargetsByName(b.BondARPAllTargets)
	if err != nil {
		result = multierror.Append(result, err)
	}

	_, err = nethelpers.PrimaryReselectByName(b.BondPrimaryReselect)
	if err != nil {
		result = multierror.Append(result, err)
	}

	_, err = nethelpers.FailOverMACByName(b.BondFailOverMac)
	if err != nil {
		result = multierror.Append(result, err)
	}

	_, err = nethelpers.ADSelectByName(b.BondADSelect)
	if err != nil {
		result = multierror.Append(result, err)
	}

	if b.BondMIIMon == 0 {
		if b.BondUpDelay != 0 {
			result = multierror.Append(result, fmt.Errorf("bond.upDelay can't be set if miiMon is zero"))
		}

		if b.BondDownDelay != 0 {
			result = multierror.Append(result, fmt.Errorf("bond.downDelay can't be set if miiMon is zero"))
		}
	} else {
		if b.BondUpDelay%b.BondMIIMon != 0 {
			result = multierror.Append(result, fmt.Errorf("bond.upDelay should be a multiple of miiMon"))
		}

		if b.BondDownDelay%b.BondMIIMon != 0 {
			result = multierror.Append(result, fmt.Errorf("bond.downDelay should be a multiple of miiMon"))
		}
	}

	if len(b.BondARPIPTarget) > 0 {
		result = multierror.Append(result, fmt.Errorf("bond.arpIPTarget is not supported"))
	}

	if b.BondLACPRate != "" && bondMode != nethelpers.BondMode8023AD {
		result = multierror.Append(result, fmt.Errorf("bond.lacpRate is only available in 802.3ad mode"))
	}

	if b.BondADActorSystem != "" {
		result = multierror.Append(result, fmt.Errorf("bond.adActorSystem is not supported"))
	}

	if (bondMode == nethelpers.BondMode8023AD || bondMode == nethelpers.BondModeALB || bondMode == nethelpers.BondModeTLB) && b.BondARPValidate != "" {
		result = multierror.Append(result, fmt.Errorf("bond.arpValidate is not available in %s mode", bondMode))
	}

	if !(bondMode == nethelpers.BondModeActiveBackup || bondMode == nethelpers.BondModeALB || bondMode == nethelpers.BondModeTLB) && b.BondPrimary != "" {
		result = multierror.Append(result, fmt.Errorf("bond.primary is not available in %s mode", bondMode))
	}

	if (bondMode == nethelpers.BondMode8023AD || bondMode == nethelpers.BondModeALB || bondMode == nethelpers.BondModeTLB) && b.BondARPInterval > 0 {
		result = multierror.Append(result, fmt.Errorf("bond.arpInterval is not available in %s mode", bondMode))
	}

	if bondMode != nethelpers.BondModeRoundrobin && b.BondPacketsPerSlave > 1 {
		result = multierror.Append(result, fmt.Errorf("bond.packetsPerSlave is not available in %s mode", bondMode))
	}

	if !(bondMode == nethelpers.BondModeALB || bondMode == nethelpers.BondModeTLB) && b.BondTLBDynamicLB > 0 {
		result = multierror.Append(result, fmt.Errorf("bond.tlbDynamicTLB is not available in %s mode", bondMode))
	}

	if bondMode != nethelpers.BondMode8023AD && b.BondADActorSysPrio > 0 {
		result = multierror.Append(result, fmt.Errorf("bond.adActorSysPrio is only available in 802.3ad mode"))
	}

	if bondMode != nethelpers.BondMode8023AD && b.BondADUserPortKey > 0 {
		result = multierror.Append(result, fmt.Errorf("bond.adUserPortKey is only available in 802.3ad mode"))
	}

	return result.ErrorOrNil()
}

func checkWireguard(b *DeviceWireguardConfig) error {
	var result *multierror.Error

	// avoid pulling in wgctrl code to keep machinery dependencies slim
	checkKey := func(key string) error {
		raw, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			return err
		}

		if len(raw) != 32 {
			return fmt.Errorf("wrong key %q length: %d", key, len(raw))
		}

		return nil
	}

	if err := checkKey(b.WireguardPrivateKey); err != nil {
		result = multierror.Append(result, fmt.Errorf("private key is invalid: %w", err))
	}

	for _, peer := range b.WireguardPeers {
		if err := checkKey(peer.WireguardPublicKey); err != nil {
			result = multierror.Append(result, fmt.Errorf("public key invalid: %w", err))
		}

		if peer.WireguardEndpoint != "" {
			if _, err := net.ResolveUDPAddr("", peer.WireguardEndpoint); err != nil {
				result = multierror.Append(result, fmt.Errorf("peer endpoint %q is invalid: %w", peer.WireguardEndpoint, err))
			}
		}

		for _, allowedIP := range peer.WireguardAllowedIPs {
			if _, _, err := net.ParseCIDR(allowedIP); err != nil {
				result = multierror.Append(result, fmt.Errorf("peer allowed IP %q is invalid: %w", allowedIP, err))
			}
		}
	}

	return result.ErrorOrNil()
}

func checkVlans(d *Device) error {
	var result *multierror.Error

	// check VLAN addressing
	for _, vlan := range d.DeviceVlans {
		if len(vlan.VlanAddresses) > 0 && vlan.VlanCIDR != "" {
			result = multierror.Append(result, fmt.Errorf("[%s] %s.%d: %s", "networking.os.device.vlan", d.DeviceInterface, vlan.VlanID, "vlan can't have both .cidr and .addresses set"))
		}

		if vlan.VlanCIDR != "" {
			if err := validateIPOrCIDR(vlan.VlanCIDR); err != nil {
				result = multierror.Append(result, fmt.Errorf("[%s] %s.%d: %w", "networking.os.device.vlan.CIDR", d.DeviceInterface, vlan.VlanID, err))
			}
		}

		for _, address := range vlan.VlanAddresses {
			if err := validateIPOrCIDR(address); err != nil {
				result = multierror.Append(result, fmt.Errorf("[%s] %s.%d: %w", "networking.os.device.vlan.addresses", d.DeviceInterface, vlan.VlanID, err))
			}
		}
	}

	return result.ErrorOrNil()
}

func validateIPOrCIDR(address string) error {
	if strings.IndexByte(address, '/') >= 0 {
		_, _, err := net.ParseCIDR(address)

		return err
	}

	if ip := net.ParseIP(address); ip == nil {
		return fmt.Errorf("failed to parse IP address %q", address)
	}

	return nil
}

// CheckDeviceAddressing ensures that an appropriate addressing method.
// has been specified.
//
//nolint:gocyclo
func CheckDeviceAddressing(d *Device, bondedInterfaces map[string]string) ([]string, error) {
	var result *multierror.Error

	if d == nil {
		return nil, fmt.Errorf("empty device")
	}

	var warnings []string

	if _, bonded := bondedInterfaces[d.Interface()]; bonded {
		if d.DeviceDHCP || d.DeviceCIDR != "" || len(d.DeviceAddresses) > 0 || d.DeviceVIPConfig != nil {
			result = multierror.Append(result, fmt.Errorf("[%s] %q: %s", "networking.os.device", d.DeviceInterface, "bonded interface shouldn't have any addressing methods configured"))
		}
	}

	// ensure either legacy CIDR is set or new addresses, but not both
	if len(d.DeviceAddresses) > 0 && d.DeviceCIDR != "" {
		result = multierror.Append(result, fmt.Errorf("[%s] %q: %s", "networking.os.device", d.DeviceInterface, "interface can't have both .cidr and .addresses set"))
	}

	// ensure cidr is a valid address
	if d.DeviceCIDR != "" {
		if err := validateIPOrCIDR(d.DeviceCIDR); err != nil {
			result = multierror.Append(result, fmt.Errorf("[%s] %q: %w", "networking.os.device.CIDR", d.DeviceInterface, err))
		}

		warnings = append(warnings, fmt.Sprintf("%q: machine.network.interface.cidr is deprecated, please use machine.network.interface.addresses", d.DeviceInterface))
	}

	// ensure addresses are valid addresses
	for _, address := range d.DeviceAddresses {
		if err := validateIPOrCIDR(address); err != nil {
			result = multierror.Append(result, fmt.Errorf("[%s] %q: %w", "networking.os.device.addresses", d.DeviceInterface, err))
		}
	}

	// check VIP IP is valid
	if d.DeviceVIPConfig != nil {
		if ip := net.ParseIP(d.DeviceVIPConfig.IP()); ip == nil {
			result = multierror.Append(result, fmt.Errorf("[%s] failed to parse %q as IP address", "networking.os.device.vip", d.DeviceVIPConfig.IP()))
		}
	}

	return warnings, result.ErrorOrNil()
}

// CheckDeviceRoutes ensures that the specified routes are valid.
func CheckDeviceRoutes(d *Device, bondedInterfaces map[string]string) ([]string, error) {
	var result *multierror.Error

	if d == nil {
		return nil, fmt.Errorf("empty device")
	}

	if len(d.DeviceRoutes) == 0 {
		return nil, result.ErrorOrNil()
	}

	for idx, route := range d.DeviceRoutes {
		if route.Network() != "" {
			if _, _, err := net.ParseCIDR(route.Network()); err != nil {
				result = multierror.Append(result, fmt.Errorf("[%s] %q: %w", "networking.os.device.route["+strconv.Itoa(idx)+"].Network", route.Network(), ErrInvalidAddress))
			}
		}

		if ip := net.ParseIP(route.Gateway()); ip == nil {
			result = multierror.Append(result, fmt.Errorf("[%s] %q: %w", "networking.os.device.route["+strconv.Itoa(idx)+"].Gateway", route.Gateway(), ErrInvalidAddress))
		}
	}

	return nil, result.ErrorOrNil()
}
