package chunker

import (
	"context"
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
