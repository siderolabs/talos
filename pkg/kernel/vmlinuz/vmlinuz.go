// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package vmlinuz provides utilities for reading bzImage kernel format.
package vmlinuz

import (
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/klauspost/compress/zstd"
	"github.com/xi2/xz"
)

type decompressFunc func(io.Reader) (io.ReadCloser, error)

type zstdWrapper struct {
	*zstd.Decoder
}

func (w zstdWrapper) Read(p []byte) (int, error) {
	return w.Decoder.Read(p)
}

func (w zstdWrapper) Close() error {
	w.Decoder.Close()

	return nil
}

// Based on https://github.com/torvalds/linux/blob/master/scripts/extract-vmlinux.
var bzImageMagic = []struct {
	magic  []byte
	reader decompressFunc
}{
	{
		magic: []byte("\3757zXZ\000"),
		reader: func(r io.Reader) (io.ReadCloser, error) {
			xr, err := xz.NewReader(r, xz.DefaultDictMax)
			if err != nil {
				return nil, err
			}

			return io.NopCloser(xr), nil
		},
	},
	{
		magic: []byte("(\265/\375"),
		reader: func(r io.Reader) (io.ReadCloser, error) {
			zr, err := zstd.NewReader(r)
			if err != nil {
				return nil, err
			}

			return zstdWrapper{zr}, nil
		},
	},
	{
		magic: []byte("\037\213\010"),
		reader: func(r io.Reader) (io.ReadCloser, error) {
			return gzip.NewReader(r)
		},
	},
	{
		magic: []byte("BZh"),
		reader: func(r io.Reader) (io.ReadCloser, error) {
			return ioutil.NopCloser(bzip2.NewReader(r)), nil
		},
	},
}

// Decompress the bzImage Linux kernel format and extract vmlinux kernel.
//
// Only following formats are supported: gzip, xz and bzip2.
func Decompress(r io.Reader) (io.ReadCloser, error) {
	// read first 64Kb and look for signature
	head := make([]byte, 65536)

	if _, err := io.ReadFull(r, head); err != nil {
		return nil, fmt.Errorf("error reading 64Kb vmlinuz head: %w", err)
	}

	start := -1

	var decompress decompressFunc

	for _, matcher := range bzImageMagic {
		start = bytes.Index(head, matcher.magic)
		if start != -1 {
			decompress = matcher.reader

			break
		}
	}

	if start == -1 {
		return nil, fmt.Errorf("error looking for vmlinuz magic")
	}

	return decompress(io.MultiReader(bytes.NewReader(head[start:]), r))
}
