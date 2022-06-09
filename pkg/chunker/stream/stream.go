// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package stream

import (
	"context"
	"fmt"
	"io"

	"github.com/talos-systems/talos/pkg/chunker"
	"github.com/talos-systems/talos/pkg/machinery/generic/slices"
)

// Options is the functional options struct.
type Options struct {
	Size int
}

// Option is the functional option func.
type Option func(*Options)

// Size sets the chunk size of the Chunker.
func Size(s int) Option {
	return func(args *Options) {
		args.Size = s
	}
}

// Stream is a conecrete type that implements the chunker.Chunker interface.
type Stream struct {
	source  Source
	options *Options

	ctx context.Context //nolint:containedctx
}

// Source is an interface describing the source of a Stream.
type Source interface {
	io.ReadCloser
}

// NewChunker initializes a Chunker with default values.
func NewChunker(ctx context.Context, source Source, setters ...Option) chunker.Chunker {
	opts := &Options{
		Size: 1024,
	}

	for _, setter := range setters {
		setter(opts)
	}

	return &Stream{
		source,
		opts,
		ctx,
	}
}

// Read implements ChunkReader.
func (c *Stream) Read() <-chan []byte {
	// Create a buffered channel of length 1.
	ch := make(chan []byte, 1)

	go func(ch chan []byte) {
		defer close(ch)
		//nolint:errcheck
		defer c.source.Close()

		buf := make([]byte, c.options.Size)

		for {
			select {
			case <-c.ctx.Done():
				return
			default:
			}

			n, err := c.source.Read(buf)
			if err != nil {
				if err != io.EOF {
					fmt.Printf("read error: %s\n", err.Error())
				}

				break
			}

			if n != 0 {
				// Copy the buffer since we will modify it in the next loop.
				b := slices.Copy(buf, n)

				select {
				case <-c.ctx.Done():
					return
				case ch <- b:
				}
			}
		}
	}(ch)

	return ch
}
