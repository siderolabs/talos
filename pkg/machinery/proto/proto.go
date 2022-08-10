// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package proto

import (
	"encoding/json"
	"net/url"
	"sync"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/siderolabs/protoenc"
	"github.com/talos-systems/crypto/x509"
	"google.golang.org/protobuf/proto" //nolint:depguard
	"inet.af/netaddr"
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
	once.Do(func() {
		protoenc.RegisterEncoderDecoder(
			func(v netaddr.IPPrefix) ([]byte, error) { return v.MarshalText() },
			func(slc []byte) (netaddr.IPPrefix, error) {
				var result netaddr.IPPrefix

				err := result.UnmarshalText(slc)
				if err != nil {
					return netaddr.IPPrefix{}, err
				}

				return result, nil
			},
		)

		protoenc.RegisterEncoderDecoder(
			func(v netaddr.IPPort) ([]byte, error) { return v.MarshalText() },
			func(slc []byte) (netaddr.IPPort, error) {
				var result netaddr.IPPort

				err := result.UnmarshalText(slc)
				if err != nil {
					return netaddr.IPPort{}, err
				}

				return result, nil
			},
		)

		protoenc.RegisterEncoderDecoder(
			// TODO(DmitriyMV): use generated proto representation of this
			func(v *x509.PEMEncodedCertificateAndKey) ([]byte, error) {
				return json.Marshal(v)
			},
			func(slc []byte) (*x509.PEMEncodedCertificateAndKey, error) {
				var result *x509.PEMEncodedCertificateAndKey

				err := json.Unmarshal(slc, &result)
				if err != nil {
					return &x509.PEMEncodedCertificateAndKey{}, err
				}

				return result, nil
			},
		)

		protoenc.RegisterEncoderDecoder(
			// TODO(DmitriyMV): use generated proto representation of this
			func(v *x509.PEMEncodedKey) ([]byte, error) {
				return json.Marshal(v)
			},
			func(slc []byte) (*x509.PEMEncodedKey, error) {
				var result *x509.PEMEncodedKey

				err := json.Unmarshal(slc, &result)
				if err != nil {
					return &x509.PEMEncodedKey{}, err
				}

				return result, nil
			},
		)

		protoenc.RegisterEncoderDecoder(
			func(v *url.URL) ([]byte, error) { return []byte(v.String()), nil },
			func(slc []byte) (*url.URL, error) {
				parse, err := url.Parse(string(slc))
				if err != nil {
					return &url.URL{}, err
				}

				return parse, nil
			},
		)

		protoenc.RegisterEncoderDecoder(
			func(v specs.Mount) ([]byte, error) {
				return protoenc.Marshal(Mount(v))
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
	})
}

// Mount specifies a mount for a container.
type Mount struct {
	Destination string   `protobuf:"1"`
	Type        string   `protobuf:"2"`
	Source      string   `protobuf:"3"`
	Options     []string `protobuf:"4"`
}
