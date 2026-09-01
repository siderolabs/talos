// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package ipmi is a tiny in-process client for the local BMC via the OpenIPMI
// character device (/dev/ipmiN).
//
// Talos ships no `ipmitool` binary, so BMC discovery talks to the kernel IPMI
// device directly. The `ipmi_si` driver is auto-loaded by udev from the DMI
// type-38 (IPMI Device Information) platform device, so `/dev/ipmi0` is present
// on machines which have a BMC with no extra configuration.
//
// Only read-only discovery commands are implemented: Get Device ID (IPMI spec
// 20.1) and Get LAN Configuration Parameters (IPMI spec 23.2).
package ipmi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
)

const (
	systemInterfaceAddrType = 0x0c // IPMI_SYSTEM_INTERFACE_ADDR_TYPE
	bmcChannel              = 0x0f // IPMI_BMC_CHANNEL

	netfnApp       = 0x06
	netfnTransport = 0x0c

	cmdGetDeviceID       = 0x01
	cmdGetLANConfigParam = 0x02

	// Get LAN Configuration Parameters selectors (IPMI spec 23.2).
	lanParamIPAddr  = 3
	lanParamMAC     = 5
	lanParamSubnet  = 6
	lanParamGateway = 12

	// LAN channel numbering is vendor-specific (Dell iDRAC=1, HPE iLO=2, Cisco
	// CIMC=1, Supermicro=1, others vary), so the whole valid LAN-channel range is
	// probed for the first channel with a configured IPv4 address.
	minLANChannel = 1
	maxLANChannel = 11
)

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

// Transport issues a single IPMI request and returns the completion code and the
// response payload (the bytes after the completion code).
//
// [Dev.SendRecv] implements it for a local BMC.
type Transport func(ctx context.Context, netfn, cmd byte, data []byte) (byte, []byte, error)

// DeviceID issues Get Device ID: it confirms the BMC is reachable and reports its
// manufacturer, product and firmware revision.
func DeviceID(ctx context.Context, transport Transport) (DeviceInfo, error) {
	cc, resp, err := transport(ctx, netfnApp, cmdGetDeviceID, nil)
	if err != nil {
		return DeviceInfo{}, err
	}

	if cc != 0 {
		return DeviceInfo{}, fmt.Errorf("get device id: completion code 0x%02x", cc)
	}

	return parseDeviceID(resp)
}

// getLANParam issues Get LAN Configuration Parameters for a single selector, returning
// the parameter data (the parameter revision byte stripped).
func getLANParam(ctx context.Context, transport Transport, channel, param byte) ([]byte, error) {
	cc, resp, err := transport(ctx, netfnTransport, cmdGetLANConfigParam, []byte{channel, param, 0, 0})
	if err != nil {
		return nil, err
	}

	if cc != 0 {
		return nil, fmt.Errorf("get lan param %d on channel %d: completion code 0x%02x", param, channel, cc)
	}

	if len(resp) < 1 { // resp[0] = parameter revision; rest = data
		return nil, errors.New("short lan param response")
	}

	return resp[1:], nil
}

// lanConfig reads the network configuration of a single LAN channel.
//
// The IPv4 address is required, the remaining parameters are best-effort: a BMC may
// implement only a subset of the selectors.
//
//nolint:gocyclo
func lanConfig(ctx context.Context, transport Transport, channel byte) (LANConfig, error) {
	cfg := LANConfig{Channel: channel}

	data, err := getLANParam(ctx, transport, channel, lanParamIPAddr)
	if err != nil {
		return cfg, err
	}

	addr, err := parseIPv4(data)
	if err != nil {
		return cfg, err
	}

	if !addr.IsValid() || addr.IsUnspecified() {
		return cfg, fmt.Errorf("channel %d has no IPv4 address configured", channel)
	}

	// the netmask is optional: fall back to a host route if the BMC doesn't report it
	bits := addr.BitLen()

	if data, err = getLANParam(ctx, transport, channel, lanParamSubnet); err == nil {
		if ones, maskErr := parseNetmask(data); maskErr == nil {
			bits = ones
		}
	}

	cfg.Address = netip.PrefixFrom(addr, bits)

	if data, err = getLANParam(ctx, transport, channel, lanParamMAC); err == nil {
		if mac, macErr := parseMAC(data); macErr == nil {
			cfg.HardwareAddr = mac
		}
	}

	if data, err = getLANParam(ctx, transport, channel, lanParamGateway); err == nil {
		if gw, gwErr := parseIPv4(data); gwErr == nil && !gw.IsUnspecified() {
			cfg.Gateway = gw
		}
	}

	return cfg, nil
}

// FindLANConfig returns the configuration of the first LAN channel with an IPv4 address
// configured.
//
// It returns [ErrNoLANChannel] if no channel has one, which is not a failure: a BMC may
// have no LAN channel configured, or may not implement the commands to report it.
func FindLANConfig(ctx context.Context, transport Transport) (LANConfig, error) {
	for ch := byte(minLANChannel); ch <= maxLANChannel; ch++ {
		// probing all channels of an unresponsive BMC takes a while, so cancellation is
		// reported instead of being flattened into ErrNoLANChannel
		if err := ctx.Err(); err != nil {
			return LANConfig{}, err
		}

		cfg, err := lanConfig(ctx, transport, ch)
		if err == nil {
			return cfg, nil
		}
	}

	return LANConfig{}, ErrNoLANChannel
}

// parseDeviceID decodes a Get Device ID response payload (IPMI spec 20.1),
// excluding the completion code.
func parseDeviceID(resp []byte) (DeviceInfo, error) {
	// resp[2]=fw major (7 bits), [3]=fw minor (BCD), [4]=ipmi version (BCD),
	// [6..8]=manufacturer id (LS-first, 20 bits), [9..10]=product id (LS-first).
	if len(resp) < 11 {
		return DeviceInfo{}, fmt.Errorf("short get device id response: %d bytes", len(resp))
	}

	return DeviceInfo{
		ManufacturerID: uint32(resp[6]) | uint32(resp[7])<<8 | (uint32(resp[8])&0x0f)<<16,
		ProductID:      uint16(resp[9]) | uint16(resp[10])<<8,
		Firmware:       fmt.Sprintf("%d.%02x", resp[2]&0x7f, resp[3]),
		IPMIVersion:    fmt.Sprintf("%d.%d", resp[4]&0x0f, (resp[4]>>4)&0x0f),
	}, nil
}

// parseIPv4 decodes a 4-byte LAN parameter as an IPv4 address.
func parseIPv4(data []byte) (netip.Addr, error) {
	if len(data) < 4 {
		return netip.Addr{}, fmt.Errorf("short IPv4 lan parameter: %d bytes", len(data))
	}

	return netip.AddrFrom4([4]byte(data[:4])), nil
}

// parseMAC decodes a 6-byte LAN parameter as a MAC address.
func parseMAC(data []byte) (net.HardwareAddr, error) {
	if len(data) < 6 {
		return nil, fmt.Errorf("short MAC lan parameter: %d bytes", len(data))
	}

	return net.HardwareAddr(data[:6]), nil
}

// parseNetmask decodes a 4-byte LAN parameter as a prefix length.
func parseNetmask(data []byte) (int, error) {
	mask, err := parseIPv4(data)
	if err != nil {
		return 0, err
	}

	ones, bits := net.IPMask(mask.AsSlice()).Size()
	if ones == 0 && bits == 0 {
		return 0, fmt.Errorf("non-contiguous subnet mask %s", mask)
	}

	return ones, nil
}

// ErrNoLANChannel is returned when no LAN channel has an IPv4 address configured.
var ErrNoLANChannel = errors.New("no BMC channel with a configured IPv4 address")
