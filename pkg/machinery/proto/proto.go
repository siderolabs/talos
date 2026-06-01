// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package proto defines a functions to work with proto messages.
package proto

import (
	"net/netip"
	"net/url"
	"sync"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/protoenc"
	_ "google.golang.org/grpc/encoding/gzip" //nolint:depguard // enable compression server-side
	"google.golang.org/protobuf/proto"       //nolint:depguard

	"github.com/siderolabs/talos/pkg/machinery/api/common"
)

// Message is the main interface for protobuf API v2 messages.
type Message = proto.Message

// UnmarshalOptions is alias for [proto.UnmarshalOptions].
type UnmarshalOptions = proto.UnmarshalOptions

type vtprotoEqual interface {
	EqualMessageVT(proto.Message) bool
}

// Equal reports whether two messages are equal.
func Equal(a, b Message) bool {
	if vm, ok := a.(vtprotoEqual); ok {
		return vm.EqualMessageVT(b)
	}

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
//
//gotagsrewrite:gen
type Mount struct {
	Destination string           `protobuf:"1"`
	Type        string           `protobuf:"2"`
	Source      string           `protobuf:"3"`
	Options     []string         `protobuf:"4"`
	UIDMappings []LinuxIDMapping `protobuf:"5"`
	GIDMappings []LinuxIDMapping `protobuf:"6"`
}

// LinuxIDMapping specifies UID/GID mappings.
//
//gotagsrewrite:gen
type LinuxIDMapping struct {
	ContainerID uint32 `protobuf:"1"`
	HostID      uint32 `protobuf:"2"`
	Size        uint32 `protobuf:"3"`
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
		func(v *x509.PEMEncodedCertificate) ([]byte, error) {
			source := common.PEMEncodedCertificate{
				Crt: v.Crt,
			}

			return proto.Marshal(&source)
		},
		func(slc []byte) (*x509.PEMEncodedCertificate, error) {
			var dest common.PEMEncodedCertificate

			if err := proto.Unmarshal(slc, &dest); err != nil {
				return nil, err
			}

			return &x509.PEMEncodedCertificate{
				Crt: dest.Crt,
			}, nil
		},
	)

	protoenc.RegisterEncoderDecoder(
		func(v netip.Addr) ([]byte, error) {
			ipEncoded, err := v.MarshalBinary()
			if err != nil {
				return nil, err
			}

			source := common.NetIP{
				Ip: ipEncoded,
			}

			return proto.Marshal(&source)
		},
		func(slc []byte) (netip.Addr, error) {
			if len(slc) == 0 || len(slc) == 4 || len(slc) == 16 {
				var parsedIP netip.Addr

				if err := parsedIP.UnmarshalBinary(slc); err != nil {
					return netip.Addr{}, err
				}

				return parsedIP, nil
			}

			var dest common.NetIP

			if err := proto.Unmarshal(slc, &dest); err != nil {
				return netip.Addr{}, err
			}

			var parsedIP netip.Addr

			if err := parsedIP.UnmarshalBinary(dest.Ip); err != nil {
				return netip.Addr{}, err
			}

			return parsedIP, nil
		},
	)

	protoenc.RegisterEncoderDecoder(
		func(v netip.AddrPort) ([]byte, error) {
			ipEncoded, err := v.Addr().MarshalBinary()
			if err != nil {
				return nil, err
			}

			source := common.NetIPPort{
				Ip:   ipEncoded,
				Port: int32(v.Port()),
			}

			return proto.Marshal(&source)
		},
		func(slc []byte) (netip.AddrPort, error) {
			var dest common.NetIPPort

			if err := proto.Unmarshal(slc, &dest); err != nil {
				return netip.AddrPort{}, err
			}

			var parsedIP netip.Addr

			if err := parsedIP.UnmarshalBinary(dest.Ip); err != nil {
				return netip.AddrPort{}, err
			}

			return netip.AddrPortFrom(parsedIP, uint16(dest.Port)), nil
		},
	)

	protoenc.RegisterEncoderDecoder(
		func(v netip.Prefix) ([]byte, error) {
			ipEncoded, err := v.Addr().WithZone("").MarshalBinary()
			if err != nil {
				return nil, err
			}

			source := common.NetIPPrefix{
				Ip:           ipEncoded,
				PrefixLength: int32(v.Bits()),
			}

			return proto.Marshal(&source)
		},
		func(slc []byte) (netip.Prefix, error) {
			var dest common.NetIPPrefix

			if err := proto.Unmarshal(slc, &dest); err != nil {
				return netip.Prefix{}, err
			}

			var parsedIP netip.Addr

			if err := parsedIP.UnmarshalBinary(dest.Ip); err != nil {
				return netip.Prefix{}, err
			}

			return netip.PrefixFrom(parsedIP, int(dest.PrefixLength)), nil
		},
	)

	protoenc.RegisterEncoderDecoder(
		func(v specs.Mount) ([]byte, error) {
			// use the intermediate type which is assignable to specs.Mount so that
			// we can be sure that `specs.Mount` and `Mount` have exactly same fields.
			//
			// as in Go []T1 is not assignable to []T2, even if T1 and T2 are assignable, we cannot
			// use direct conversion of Mount and specs.Mount
			type mountConverter struct {
				Destination string
				Type        string
				Source      string
				Options     []string
				UIDMappings []specs.LinuxIDMapping
				GIDMappings []specs.LinuxIDMapping
			}

			mount := func(m mountConverter) Mount {
				return Mount{
					Destination: m.Destination,
					Type:        m.Type,
					Source:      m.Source,
					Options:     m.Options,
					UIDMappings: xslices.Map(m.UIDMappings, func(m specs.LinuxIDMapping) LinuxIDMapping { return LinuxIDMapping(m) }),
					GIDMappings: xslices.Map(m.GIDMappings, func(m specs.LinuxIDMapping) LinuxIDMapping { return LinuxIDMapping(m) }),
				}
			}(mountConverter(v))

			return protoenc.Marshal(&mount)
		},
		func(slc []byte) (specs.Mount, error) {
			var result Mount

			err := protoenc.Unmarshal(slc, &result)
			if err != nil {
				return specs.Mount{}, err
			}

			return specs.Mount{
				Destination: result.Destination,
				Type:        result.Type,
				Source:      result.Source,
				Options:     result.Options,
				UIDMappings: xslices.Map(result.UIDMappings, func(v LinuxIDMapping) specs.LinuxIDMapping { return specs.LinuxIDMapping(v) }),
				GIDMappings: xslices.Map(result.GIDMappings, func(v LinuxIDMapping) specs.LinuxIDMapping { return specs.LinuxIDMapping(v) }),
			}, nil
		},
	)
}
