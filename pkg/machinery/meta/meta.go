// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package meta provides interfaces for encoding and decoding META values.
package meta

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/siderolabs/gen/xslices"
)

// Value represents a key/value pair for META.
type Value struct {
	Key   uint8
	Value string
}

func (v Value) String() string {
	return fmt.Sprintf("0x%x=%s", v.Key, v.Value)
}

// Parse k=v expression.
func (v *Value) Parse(s string) error {
	k, vv, ok := strings.Cut(s, "=")
	if !ok {
		return fmt.Errorf("invalid value %q", s)
	}

	key, err := strconv.ParseUint(k, 0, 8)
	if err != nil {
		return fmt.Errorf("invalid key %q", k)
	}

	v.Key = uint8(key)
	v.Value = vv

	return nil
}

// Values is a collection of Value.
type Values []Value

// Encode returns a string representation of Values for the environment variable.
//
// Each Value is encoded a k=v, split by ';' character.
// The result is base64 encoded.
func (v Values) Encode(allowGzip bool) string {
	raw := []byte(strings.Join(xslices.Map(v, Value.String), ";"))

	if allowGzip && len(raw) > 256 {
		var buf bytes.Buffer

		gzW, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
		if err != nil {
			panic(err)
		}

		if _, err := gzW.Write(raw); err != nil {
			panic(err)
		}

		if err := gzW.Close(); err != nil {
			panic(err)
		}

		raw = buf.Bytes()
	}

	return base64.StdEncoding.EncodeToString(raw)
}

// DecodeValues parses a string representation of Values for the environment variable.
//
// See Encode for the details of the encoding.
func DecodeValues(s string) (Values, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}

	if len(b) == 0 {
		return nil, nil
	}

	// do un-gzip if needed
	if hasGzipMagic(b) {
		gzR, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}

		defer gzR.Close() //nolint:errcheck

		b, err = io.ReadAll(gzR)
		if err != nil {
			return nil, err
		}

		if err := gzR.Close(); err != nil {
			return nil, err
		}
	}

	parts := strings.Split(string(b), ";")

	result := make(Values, 0, len(parts))

	for _, v := range parts {
		var vv Value

		if err := vv.Parse(v); err != nil {
			return nil, err
		}

		result = append(result, vv)
	}

	return result, nil
}

func hasGzipMagic(b []byte) bool {
	if len(b) < 10 {
		return false
	}

	// See https://en.wikipedia.org/wiki/Gzip#File_format.
	return b[0] == 0x1f && b[1] == 0x8b
}
