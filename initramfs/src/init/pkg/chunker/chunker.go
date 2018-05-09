package chunker

import (
	"context"
	"fmt"
	"io"
	"os"
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
	path    string
	options *Options
}

// NewDefaultChunker initializes a DefaultChunker with default values.
func NewDefaultChunker(path string, setters ...Option) Chunker {
	opts := &Options{
		Size: 1024,
	}

	for _, setter := range setters {
		setter(opts)
	}
	return &DefaultChunker{
		path,
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
	file, err := os.OpenFile(c.path, os.O_RDONLY, 0)
	if err != nil {
		return nil
	}

	go func(ch chan []byte, f *os.File) {
		defer close(ch)
		// nolint: errcheck
		defer f.Close()

		offset, err := f.Seek(0, io.SeekStart)
		if err != nil {
			return
		}
		buf := make([]byte, c.options.Size)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, err := f.ReadAt(buf, offset)
				if err != nil {
					if err != io.EOF {
						fmt.Printf("read %s: %s\n", c.path, err.Error())
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
	}(ch, file)

	return ch
}
