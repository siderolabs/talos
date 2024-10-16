// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package codec registers the gRPC for optimized marshaling.
//
// Package should be dummy imported to enable.
package codec

import (
	"fmt"

	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/mem"

	talosproto "github.com/siderolabs/talos/pkg/machinery/proto"
)

// gogoMessage is the interface for gogoproto additions.
//
// We use only a subset of that interface but include additional methods
// to prevent accidental successful type assertion for unrelated types.
type gogoMessage interface {
	Marshal() ([]byte, error)
	MarshalTo([]byte) (int, error)
	MarshalToSizedBuffer([]byte) (int, error)
	Unmarshal([]byte) error
}

// Codec provides protobuf encoding.Codec.
type Codec struct{}

// Marshal implements encoding.Codec.
func (Codec) Marshal(v any) ([]byte, error) {
	// some third-party types (like from etcd and containerd) implement gogoMessage
	if gm, ok := v.(gogoMessage); ok {
		return gm.Marshal()
	}

	// our types implement Message (with or without vtproto additions depending on build configuration)
	if m, ok := v.(talosproto.Message); ok {
		return talosproto.Marshal(m)
	}

	// no types implement protobuf API v1 only, so don't check for it

	return nil, fmt.Errorf("failed to marshal %T", v)
}

// Unmarshal implements encoding.Codec.
func (Codec) Unmarshal(data []byte, v any) error {
	// some third-party types (like from etcd and containerd) implement gogoMessage
	if gm, ok := v.(gogoMessage); ok {
		return gm.Unmarshal(data)
	}

	// our types implement Message (with or without vtproto additions depending on build configuration)
	if m, ok := v.(talosproto.Message); ok {
		return talosproto.Unmarshal(data, m)
	}

	// no types implement protobuf API v1 only, so don't check for it

	return fmt.Errorf("failed to unmarshal %T", v)
}

// Name implements encoding.Codec.
func (Codec) Name() string {
	return proto.Name // overrides google.golang.org/grpc/encoding/proto codec
}

type codecV1Bridge struct {
	codec interface {
		Marshal(v any) ([]byte, error)
		Unmarshal(data []byte, v any) error
		Name() string
	}
}

var _ encoding.CodecV2 = codecV1Bridge{}

func (c codecV1Bridge) Marshal(v any) (mem.BufferSlice, error) {
	data, err := c.codec.Marshal(v)
	if err != nil {
		return nil, err
	}

	return mem.BufferSlice{mem.NewBuffer(&data, nil)}, nil
}

func (c codecV1Bridge) Unmarshal(data mem.BufferSlice, v any) (err error) {
	return c.codec.Unmarshal(data.Materialize(), v)
}

func (c codecV1Bridge) Name() string {
	return c.codec.Name()
}

func init() {
	encoding.RegisterCodecV2(codecV1Bridge{codec: Codec{}})
}
