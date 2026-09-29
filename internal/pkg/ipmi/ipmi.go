// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package ipmi discovers the local BMC through the OpenIPMI character device
// (/dev/ipmiN), using github.com/bougou/go-ipmi as the transport.
//
// Talos ships no `ipmitool` binary, so BMC discovery talks to the kernel IPMI
// device directly. The `ipmi_si` driver is auto-loaded by udev from the DMI
// type-38 (IPMI Device Information) platform device, so `/dev/ipmi0` is present
// on machines which have a BMC with no extra configuration.
//
// Only read-only discovery commands are issued: Get Device ID (IPMI spec 20.1)
// and Get LAN Configuration Parameters (IPMI spec 23.2).
package ipmi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"github.com/bougou/go-ipmi/pkg/client"
	"github.com/bougou/go-ipmi/pkg/types"
)

// DevicePathGlob matches the OpenIPMI character devices.
const DevicePathGlob = "/dev/ipmi*"

// LAN channel numbering is vendor-specific (Dell iDRAC=1, HPE iLO=2, Cisco
// CIMC=1, Supermicro=1, others vary), so the whole valid LAN-channel range is
// probed for the first channel with a configured IPv4 address.
const (
	minLANChannel = 1
	maxLANChannel = 11
)

// ErrNoLANChannel is returned when no LAN channel has an IPv4 address configured.
var ErrNoLANChannel = errors.New("no BMC channel with a configured IPv4 address")

// DeviceNumber extracts the OpenIPMI device number from a path matched by
// [DevicePathGlob].
//
// It reports false for a path which is not a device: the glob also matches the
// `/dev/ipmi` directory some udev rulesets create.
func DeviceNumber(path string) (int32, bool) {
	num, err := strconv.ParseInt(strings.TrimPrefix(path, "/dev/ipmi"), 10, 32)
	if err != nil {
		return 0, false
	}

	return int32(num), true
}

// DeviceInfo is the BMC identity as reported by Get Device ID.
type DeviceInfo struct {
	// ManufacturerID is the IANA enterprise number of the BMC vendor (e.g. 674 = Dell).
	ManufacturerID uint32
	// ProductID is the vendor-specific product identifier.
	ProductID uint16
	// Firmware is the BMC firmware revision, "major.minor".
	Firmware string
	// IPMIVersion is the IPMI specification version supported, e.g. "2.0".
	IPMIVersion string
}

// LANConfig is the BMC network configuration of a single LAN channel.
type LANConfig struct {
	// Channel is the IPMI LAN channel this configuration was read from.
	Channel uint8
	// Address is the BMC IPv4 address together with the subnet mask.
	Address netip.Prefix
	// Gateway is the BMC default gateway.
	Gateway netip.Addr
	// HardwareAddr is the MAC address of the BMC LAN interface.
	HardwareAddr net.HardwareAddr
}

// Open connects to the local BMC exposed as /dev/ipmi<devnum>.
func Open(ctx context.Context, devnum int32) (*client.Client, error) {
	c, err := client.NewOpenClient()
	if err != nil {
		return nil, err
	}

	if err = c.ConnectOpen(ctx, devnum); err != nil {
		return nil, err
	}

	return c, nil
}

// DeviceID issues Get Device ID: it confirms the BMC is reachable and reports its
// manufacturer, product and firmware revision.
func DeviceID(ctx context.Context, c *client.Client) (DeviceInfo, error) {
	resp, err := c.GetDeviceID(ctx)
	if err != nil {
		return DeviceInfo{}, err
	}

	return DeviceInfo{
		// go-ipmi decodes all 24 bits, the upper 4 are reserved (IPMI spec 20.1)
		ManufacturerID: resp.ManufacturerID & 0x0fffff,
		ProductID:      resp.ProductID,
		Firmware:       fmt.Sprintf("%d.%02d", resp.MajorFirmwareRevision, resp.MinorFirmwareRevision),
		IPMIVersion:    fmt.Sprintf("%d.%d", resp.MajorIPMIVersion, resp.MinorIPMIVersion),
	}, nil
}

// LANParamGetter reads a single Get LAN Configuration Parameters selector into param.
//
// [client.Client.GetLanConfigParamFor] implements it for a local BMC.
type LANParamGetter func(ctx context.Context, channel uint8, param types.LanConfigParameter) error

// FindLANConfig returns the configuration of the first LAN channel with an IPv4 address
// configured.
//
// It returns [ErrNoLANChannel] if no channel has one, which is not a failure: a BMC may
// have no LAN channel configured, or may not implement the commands to report it.
func FindLANConfig(ctx context.Context, get LANParamGetter) (LANConfig, error) {
	for ch := uint8(minLANChannel); ch <= maxLANChannel; ch++ {
		// probing all channels of an unresponsive BMC takes a while, so cancellation is
		// reported instead of being flattened into ErrNoLANChannel
		if err := ctx.Err(); err != nil {
			return LANConfig{}, err
		}

		cfg, err := lanConfig(ctx, get, ch)
		if err == nil {
			return cfg, nil
		}
	}

	return LANConfig{}, ErrNoLANChannel
}

// lanConfig reads the network configuration of a single LAN channel.
//
// The IPv4 address is required, the remaining parameters are best-effort: a BMC may
// implement only a subset of the selectors.
//
//nolint:gocyclo
func lanConfig(ctx context.Context, get LANParamGetter, channel uint8) (LANConfig, error) {
	var ip types.LanConfigParam_IP

	if err := get(ctx, channel, &ip); err != nil {
		return LANConfig{}, err
	}

	addr, ok := netip.AddrFromSlice(ip.IP.To4())
	if !ok || addr.IsUnspecified() {
		return LANConfig{}, fmt.Errorf("channel %d has no IPv4 address configured", channel)
	}

	cfg := LANConfig{Channel: channel}

	// the netmask is optional: fall back to a host route if the BMC doesn't report a
	// usable (contiguous, non-zero) one
	bits := addr.BitLen()

	var mask types.LanConfigParam_SubnetMask

	if get(ctx, channel, &mask) == nil {
		if ones, _ := net.IPMask(mask.SubnetMask.To4()).Size(); ones > 0 {
			bits = ones
		}
	}

	cfg.Address = netip.PrefixFrom(addr, bits)

	var mac types.LanConfigParam_MAC

	if get(ctx, channel, &mac) == nil {
		cfg.HardwareAddr = mac.MAC
	}

	var gateway types.LanConfigParam_DefaultGatewayIP

	if get(ctx, channel, &gateway) == nil {
		if gw, ok := netip.AddrFromSlice(gateway.IP.To4()); ok && !gw.IsUnspecified() {
			cfg.Gateway = gw
		}
	}

	return cfg, nil
}
