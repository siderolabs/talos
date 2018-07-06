package chunker

import (
	"context"
	"fmt"
	"io"
)

// Chunker is an interface for embedding all chunking interfaces under one name.
type Chunker interface {
	ChunkReader
}

// ChunkReader is an interface describing a reader that streams data in []byte
// chunks.
type ChunkReader interface {
	Read(context.Context) <-chan []byte
}

// DefaultChunker is a conecrete type that implements the Chunker interface.
type DefaultChunker struct {
	source  ChunkSource
	options *Options
}

// ChunkSource is an interface describing the source of a Chunker.
type ChunkSource interface {
	io.ReaderAt
	io.Seeker
	io.Closer
	io.Writer
}

// NewDefaultChunker initializes a DefaultChunker with default values.
func NewDefaultChunker(source ChunkSource, setters ...Option) Chunker {
	opts := &Options{
		Size: 1024,
	}

	for _, setter := range setters {
		setter(opts)
	}

	return &DefaultChunker{
		source,
		opts,
	}
}

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

// Read implements ChunkReader.
func (c *DefaultChunker) Read(ctx context.Context) <-chan []byte {
	// Create a buffered channel of length 1.
	ch := make(chan []byte, 1)

	go func(ch chan []byte) {
		defer close(ch)
		// nolint: errcheck
		defer c.source.Close()

		offset, err := c.source.Seek(0, io.SeekStart)
		if err != nil {
			return
		}
		buf := make([]byte, c.options.Size)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, err := c.source.ReadAt(buf, offset)
				if err != nil {
					if err != io.EOF {
						fmt.Printf("read error: %s\n", err.Error())
						break
					}
				}
				offset += int64(n)
				if n != 0 {
					// Copy the buffer since we will modify it in the next loop.
					b := make([]byte, n)
					copy(b, buf[:n])
					ch <- b
				}
				// Clear the buffer.
				for i := 0; i < n; i++ {
					buf[i] = 0
				}
			}
		}
	}(ch)

	return ch
}
