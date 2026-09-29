// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package lldp receives and decodes LLDP advertisements.
package lldp

import (
	"cmp"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/mdlayher/ethernet"
	"github.com/siderolabs/go-lldp/pkg/lldp"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// Listener receives Ethernet frames. The returned frame is valid until the next read.
// Close must unblock ReadFrame.
type Listener interface {
	// SetReadDeadline bounds the next ReadFrame, which then fails with os.ErrDeadlineExceeded.
	SetReadDeadline(time.Time) error
	ReadFrame() ([]byte, error)
	Close() error
}

// ListenerFactory opens a receive-only listener on the interface with the given kernel index.
type ListenerFactory func(linkIndex uint32) (Listener, error)

// NeighborKey is the length-delimited raw chassis and port identity, including subtypes.
type NeighborKey string

// Neighbor is an advertisement, including protocol identity and lifetime.
// A zero TTL withdraws the neighbor identified by Key.
type Neighbor struct {
	Spec network.LLDPNeighborSpec
	Key  NeighborKey
	TTL  uint16
}

// DecodeFrame decodes an Ethernet LLDP frame without interpreting unsupported TLVs.
func DecodeFrame(frame []byte) (Neighbor, error) {
	var envelope ethernet.Frame

	if err := envelope.UnmarshalBinary(frame); err != nil {
		return Neighbor{}, fmt.Errorf("decode LLDP Ethernet header: %w", err)
	}

	if envelope.EtherType != lldp.EtherType {
		return Neighbor{}, errors.New("not an LLDP Ethernet frame")
	}

	return decodeTLVs(envelope.Payload)
}

//nolint:gocyclo
func decodeTLVs(data []byte) (Neighbor, error) {
	var frame lldp.Frame

	if err := frame.UnmarshalBinary(data); err != nil {
		return Neighbor{}, fmt.Errorf("decode LLDP frame: %w", err)
	}

	// The library validates framing and mandatory ordering, but leaves ID
	// semantics and duplicate mandatory TLVs to the caller.
	var neighbor Neighbor

	chassisID, err := decodeChassisID(frame.ChassisID)
	if err != nil {
		return Neighbor{}, err
	}

	portID, err := decodePortID(frame.PortID)
	if err != nil {
		return Neighbor{}, err
	}

	neighbor.Spec.ChassisID = chassisID
	neighbor.Spec.PortID = portID
	neighbor.Key = neighborKey(frame.ChassisID, frame.PortID)
	neighbor.TTL = uint16(frame.TTL / time.Second)
	vlans := map[uint16]string{}

	for _, tlv := range frame.Optional {
		value := tlv.Value

		switch tlv.Type {
		case lldp.TLVTypeEnd, lldp.TLVTypeSystemCapabilities:
			// End is consumed by the library; capabilities are not exposed.
		case lldp.TLVTypeChassisID, lldp.TLVTypePortID, lldp.TLVTypeTTL:
			return Neighbor{}, errors.New("duplicate mandatory LLDP TLV")
		case lldp.TLVTypePortDescription:
			neighbor.Spec.PortDescription = printableText(value)
		case lldp.TLVTypeSystemName:
			neighbor.Spec.SystemName = printableText(value)
		case lldp.TLVTypeSystemDescription:
			neighbor.Spec.SystemDescription = printableText(value)
		case lldp.TLVTypeManagementAddress:
			address, err := decodeManagementAddress(value)
			if err != nil {
				return Neighbor{}, err
			}

			if address != "" {
				neighbor.Spec.ManagementAddresses = append(neighbor.Spec.ManagementAddresses, address)
			}
		case lldp.TLVTypeOrganizationSpecific:
			if err := decodeVLAN(value, vlans); err != nil {
				return Neighbor{}, err
			}
		}
	}

	slices.Sort(neighbor.Spec.ManagementAddresses)
	neighbor.Spec.ManagementAddresses = slices.Compact(neighbor.Spec.ManagementAddresses)

	for id, name := range vlans {
		neighbor.Spec.VLANs = append(neighbor.Spec.VLANs, network.LLDPVLANSpec{ID: id, Name: name})
	}

	slices.SortFunc(neighbor.Spec.VLANs, func(a, b network.LLDPVLANSpec) int { return cmp.Compare(a.ID, b.ID) })

	return neighbor, nil
}

// neighborKey identifies a neighbor by its raw advertised bytes, so that two
// identifiers which render alike but differ on the wire stay distinct.
func neighborKey(chassis *lldp.ChassisID, port *lldp.PortID) NeighborKey {
	key := binary.BigEndian.AppendUint16(nil, uint16(len(chassis.ID)+1))
	key = append(key, byte(chassis.Subtype))
	key = append(key, chassis.ID...)
	key = binary.BigEndian.AppendUint16(key, uint16(len(port.ID)+1))
	key = append(key, byte(port.Subtype))
	key = append(key, port.ID...)

	return NeighborKey(key)
}

func decodeChassisID(chassis *lldp.ChassisID) (string, error) {
	if len(chassis.ID) == 0 || len(chassis.ID) > 255 ||
		chassis.Subtype < lldp.ChassisIDSubtypeChassisComponent || chassis.Subtype > lldp.ChassisIDSubtypeLocallyAssigned {
		return "", errors.New("invalid LLDP chassis identifier")
	}

	if chassis.Subtype == lldp.ChassisIDSubtypeMACAddress {
		return macAddress(chassis.ID)
	}

	if chassis.Subtype == lldp.ChassisIDSubtypeNetworkAddress {
		return networkAddress(chassis.ID)
	}

	return printableText(chassis.ID), nil
}

func decodePortID(port *lldp.PortID) (string, error) {
	if len(port.ID) == 0 || len(port.ID) > 255 ||
		port.Subtype < lldp.PortIDSubtypeInterfaceAlias || port.Subtype > lldp.PortIDSubtypeLocallyAssigned {
		return "", errors.New("invalid LLDP port identifier")
	}

	if port.Subtype == lldp.PortIDSubtypeMACAddress {
		return macAddress(port.ID)
	}

	if port.Subtype == lldp.PortIDSubtypeNetworkAddress {
		return networkAddress(port.ID)
	}

	return printableText(port.ID), nil
}

func macAddress(value []byte) (string, error) {
	if len(value) != 6 {
		return "", errors.New("invalid LLDP MAC identifier length")
	}

	return net.HardwareAddr(value).String(), nil
}

// networkAddress renders an IANA address family. Families other than IPv4 and
// IPv6 have no textual form here, so the field is dropped.
func networkAddress(value []byte) (string, error) {
	if len(value) < 2 {
		return "", errors.New("invalid LLDP network address")
	}

	if value[0] != 1 && value[0] != 2 {
		return "", nil
	}

	length := 4
	if value[0] == 2 {
		length = 16
	}

	if len(value) != length+1 {
		return "", errors.New("invalid LLDP IP address length")
	}

	address, _ := netip.AddrFromSlice(value[1:])

	return address.String(), nil
}

// printableText returns valid, printable UTF-8 verbatim. Anything else is
// dropped rather than rendered in an invented encoding.
func printableText(value []byte) string {
	if !utf8.Valid(value) {
		return ""
	}

	text := string(value)

	for _, r := range text {
		if !unicode.IsPrint(r) {
			return ""
		}
	}

	return text
}

func decodeManagementAddress(value []byte) (string, error) {
	if len(value) < 1 {
		return "", errors.New("missing LLDP management address length")
	}

	length := int(value[0])
	if length < 2 || length > 32 || len(value) < length+7 {
		return "", errors.New("invalid LLDP management address length")
	}

	if len(value) != length+7+int(value[length+6]) {
		return "", errors.New("invalid LLDP management address OID length")
	}

	return networkAddress(value[1 : length+1])
}

func decodeVLAN(value []byte, vlans map[uint16]string) error {
	if len(value) < 4 {
		return errors.New("truncated LLDP organizational TLV header")
	}

	if string(value[:3]) != "\x00\x80\xc2" { // IEEE 802.1 OUI.
		return nil
	}

	var name string

	switch value[3] {
	case 1: // IEEE 802.1 port VLAN ID.
		if len(value) != 6 {
			return errors.New("invalid LLDP PVID length")
		}
	case 3: // IEEE 802.1 VLAN name.
		if len(value) < 7 || int(value[6]) != len(value)-7 || value[6] > 32 {
			return errors.New("invalid LLDP VLAN name length")
		}

		name = printableText(value[7:])
	default:
		return nil
	}

	mergeVLAN(vlans, binary.BigEndian.Uint16(value[4:6]), name)

	return nil
}

// mergeVLAN prefers a named VLAN over a bare PVID; otherwise the first one wins.
func mergeVLAN(vlans map[uint16]string, id uint16, name string) {
	if previous, exists := vlans[id]; !exists || previous == "" {
		vlans[id] = name
	}
}
