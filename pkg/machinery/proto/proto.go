// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package proto

import (
	"net/url"
	"sync"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/siderolabs/protoenc"
	"github.com/talos-systems/crypto/x509"
	"google.golang.org/protobuf/proto" //nolint:depguard
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/api/common"
)

// Message is the main interface for protobuf API v2 messages.
type Message = proto.Message

// Equal reports whether two messages are equal.
func Equal(a, b Message) bool {
	return proto.Equal(a, b)
}

// vtprotoMessage is the interface for vtproto additions.
//
// We use only a subset of that interface but include additional methods
// to prevent accidental successful type assertion for unrelated types.
type vtprotoMessage interface {
	MarshalVT() ([]byte, error)
	MarshalToVT([]byte) (int, error)
	MarshalToSizedBufferVT([]byte) (int, error)
	UnmarshalVT([]byte) error
}

// Marshal returns the wire-format encoding of m.
func Marshal(m Message) ([]byte, error) {
	if vm, ok := m.(vtprotoMessage); ok {
		return vm.MarshalVT()
	}

	return proto.Marshal(m)
}

// Unmarshal parses the wire-format message in b and places the result in m.
// The provided message must be mutable (e.g., a non-nil pointer to a message).
func Unmarshal(b []byte, m Message) error {
	if vm, ok := m.(vtprotoMessage); ok {
		return vm.UnmarshalVT(b)
	}

	return proto.Unmarshal(b, m)
}

var once sync.Once

// RegisterDefaultTypes registers the pair of encoders/decoders for common types.
func RegisterDefaultTypes() {
	once.Do(registerDefaultTypes)
}

// Mount specifies a mount for a container.
type Mount struct {
	Destination string   `protobuf:"1"`
	Type        string   `protobuf:"2"`
	Source      string   `protobuf:"3"`
	Options     []string `protobuf:"4"`
}

//nolint:gocyclo
func registerDefaultTypes() {
	protoenc.RegisterEncoderDecoder(
		func(v *url.URL) ([]byte, error) {
			source := common.URL{
				FullPath: v.String(),
			}

			return proto.Marshal(&source)
		},
		func(slc []byte) (*url.URL, error) {
			var dest common.URL

			if err := proto.Unmarshal(slc, &dest); err != nil {
				return nil, err
			}

			return url.Parse(dest.FullPath)
		},
	)

	protoenc.RegisterEncoderDecoder(
		func(v *x509.PEMEncodedCertificateAndKey) ([]byte, error) {
			source := common.PEMEncodedCertificateAndKey{
				Crt: v.Crt,
				Key: v.Key,
			}

			return proto.Marshal(&source)
		},
		func(slc []byte) (*x509.PEMEncodedCertificateAndKey, error) {
			var dest common.PEMEncodedCertificateAndKey

			if err := proto.Unmarshal(slc, &dest); err != nil {
				return nil, err
			}

			return &x509.PEMEncodedCertificateAndKey{
				Crt: dest.Crt,
				Key: dest.Key,
			}, nil
		},
	)

	protoenc.RegisterEncoderDecoder(
		func(v *x509.PEMEncodedKey) ([]byte, error) {
			source := common.PEMEncodedKey{
				Key: v.Key,
			}

			return proto.Marshal(&source)
		},
		func(slc []byte) (*x509.PEMEncodedKey, error) {
			var dest common.PEMEncodedKey

			if err := proto.Unmarshal(slc, &dest); err != nil {
				return nil, err
			}

			return &x509.PEMEncodedKey{
				Key: dest.Key,
			}, nil
		},
	)

	protoenc.RegisterEncoderDecoder(
		func(v netaddr.IP) ([]byte, error) {
			ipEncoded, err := v.MarshalBinary()
			if err != nil {
				return nil, err
			}

			source := common.NetIP{
				Ip: ipEncoded,
			}

			return proto.Marshal(&source)
		},
		func(slc []byte) (netaddr.IP, error) {
			var dest common.NetIP

			if err := proto.Unmarshal(slc, &dest); err != nil {
				return netaddr.IP{}, err
			}

			var parsedIP netaddr.IP

			if err := parsedIP.UnmarshalBinary(dest.Ip); err != nil {
				return netaddr.IP{}, err
			}

			return parsedIP, nil
		},
	)

	protoenc.RegisterEncoderDecoder(
		func(v netaddr.IPPort) ([]byte, error) {
			ipEncoded, err := v.IP().MarshalBinary()
			if err != nil {
				return nil, err
			}

			source := common.NetIPPort{
				Ip:   ipEncoded,
				Port: int32(v.Port()),
			}

			return proto.Marshal(&source)
		},
		func(slc []byte) (netaddr.IPPort, error) {
			var dest common.NetIPPort

			if err := proto.Unmarshal(slc, &dest); err != nil {
				return netaddr.IPPort{}, err
			}

			var parsedIP netaddr.IP

			if err := parsedIP.UnmarshalBinary(dest.Ip); err != nil {
				return netaddr.IPPort{}, err
			}

			return netaddr.IPPortFrom(parsedIP, uint16(dest.Port)), nil
		},
	)

	protoenc.RegisterEncoderDecoder(
		func(v netaddr.IPPrefix) ([]byte, error) {
			ipEncoded, err := v.IP().WithZone("").MarshalBinary()
			if err != nil {
				return nil, err
			}

			source := common.NetIPPrefix{
				Ip:           ipEncoded,
				PrefixLength: int32(v.Bits()),
			}

			return proto.Marshal(&source)
		},
		func(slc []byte) (netaddr.IPPrefix, error) {
			var dest common.NetIPPrefix

			if err := proto.Unmarshal(slc, &dest); err != nil {
				return netaddr.IPPrefix{}, err
			}

			var parsedIP netaddr.IP

			if err := parsedIP.UnmarshalBinary(dest.Ip); err != nil {
				return netaddr.IPPrefix{}, err
			}

			return netaddr.IPPrefixFrom(parsedIP, uint8(dest.PrefixLength)), nil
		},
	)

	protoenc.RegisterEncoderDecoder(
		func(v specs.Mount) ([]byte, error) {
			mount := Mount(v)

			return protoenc.Marshal(&mount)
		},
		func(slc []byte) (specs.Mount, error) {
			var result Mount

			err := protoenc.Unmarshal(slc, &result)
			if err != nil {
				return specs.Mount{}, err
			}

			return specs.Mount(result), nil
		},
	)
}
