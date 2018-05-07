package chunker

import (
	"context"
	"fmt"
	"io"
	"os"
)

type Options struct {
	Size int
}

type Option func(*Options)

type Chunker interface {
	ChunkReader
}

type ChunkReader interface {
	Read(context.Context) <-chan []byte
}

type DefaultChunker struct {
	path    string
	options *Options
}

func Size(s int) Option {
	return func(args *Options) {
		args.Size = s
	}
}

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

func (c *DefaultChunker) Read(ctx context.Context) <-chan []byte {
	// Create a buffered channel of length 1.
	ch := make(chan []byte, 1)
	file, err := os.OpenFile(c.path, os.O_RDONLY, 0)
	if err != nil {
		return nil
	}

	go func(ch chan []byte, f *os.File) {
		defer close(ch)
		defer f.Close()

		offset, err := f.Seek(0, io.SeekStart)
		if err != nil {
			return
		}
		buf := make([]byte, c.options.Size, c.options.Size)
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
