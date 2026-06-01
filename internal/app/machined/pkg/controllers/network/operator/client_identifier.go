// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package operator

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/iana"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// GetDHCPv6ClientIdentifier returns the DHCPv6 client identifier to use.
func GetDHCPv6ClientIdentifier(ctx context.Context, st state.State, logger *zap.Logger, linkName string, spec network.ClientIdentifierSpec) ([]dhcpv6.Modifier, error) {
	switch spec.ClientIdentifier {
	case nethelpers.ClientIdentifierNone:
		return nil, nil //nolint:nilerr
	case nethelpers.ClientIdentifierMAC:
		link, err := safe.StateGetByID[*network.LinkStatus](ctx, st, linkName)
		if err != nil {
			return nil, fmt.Errorf("error getting link %q: %w", linkName, err)
		}

		if len(link.TypedSpec().HardwareAddr) == 0 {
			return nil, fmt.Errorf("link %q has no hardware address", linkName)
		}

		duid := dhcpv6.DUIDLL{
			HWType:        iana.HWTypeEthernet,
			LinkLayerAddr: net.HardwareAddr(link.TypedSpec().HardwareAddr),
		}

		return []dhcpv6.Modifier{dhcpv6.WithClientID(&duid)}, nil
	case nethelpers.ClientIdentifierDUID:
		if spec.DUIDRawHex == "" {
			return nil, fmt.Errorf("duidRawHex must be set when clientIdentifier is DUID")
		}

		duidBin, err := hex.DecodeString(spec.DUIDRawHex)
		if err != nil {
			logger.Error("failed to parse DUID, ignored", zap.String("link", linkName))

			return nil, nil //nolint:nilerr
		}

		duid, err := dhcpv6.DUIDFromBytes(duidBin)
		if err != nil {
			logger.Error("failed to parse DUID, ignored", zap.String("link", linkName))

			return nil, nil //nolint:nilerr
		}

		return []dhcpv6.Modifier{dhcpv6.WithClientID(duid)}, nil
	default:
		return nil, fmt.Errorf("unknown client identifier %d", spec.ClientIdentifier)
	}
}

// GetDHCP4ClientIdentifier returns the DHCP client identifier to use.
func GetDHCP4ClientIdentifier(ctx context.Context, st state.State, logger *zap.Logger, linkName string, spec network.ClientIdentifierSpec) ([]dhcpv4.Modifier, error) {
	switch spec.ClientIdentifier {
	case nethelpers.ClientIdentifierNone:
		return nil, nil //nolint:nilerr
	case nethelpers.ClientIdentifierMAC:
		link, err := safe.StateGetByID[*network.LinkStatus](ctx, st, linkName)
		if err != nil {
			return nil, fmt.Errorf("error getting link %q: %w", linkName, err)
		}

		if len(link.TypedSpec().HardwareAddr) == 0 {
			return nil, fmt.Errorf("link %q has no hardware address", linkName)
		}

		// per RFC 2132, section 9.14
		identifier := append([]byte{byte(iana.HWTypeEthernet)}, link.TypedSpec().HardwareAddr...)

		return []dhcpv4.Modifier{dhcpv4.WithOption(dhcpv4.OptClientIdentifier(identifier))}, nil
	case nethelpers.ClientIdentifierDUID:
		if spec.DUIDRawHex == "" {
			return nil, fmt.Errorf("duidRawHex must be set when clientIdentifier is DUID")
		}

		duidBin, err := hex.DecodeString(spec.DUIDRawHex)
		if err != nil {
			logger.Error("failed to parse DUID, ignored", zap.String("link", linkName))

			return nil, nil //nolint:nilerr
		}

		_, err = dhcpv6.DUIDFromBytes(duidBin)
		if err != nil {
			logger.Error("failed to parse DUID, ignored", zap.String("link", linkName))

			return nil, nil //nolint:nilerr
		}

		return []dhcpv4.Modifier{dhcpv4.WithOption(dhcpv4.OptClientIdentifier(duidBin))}, nil
	default:
		return nil, fmt.Errorf("unknown client identifier %d", spec.ClientIdentifier)
	}
}
