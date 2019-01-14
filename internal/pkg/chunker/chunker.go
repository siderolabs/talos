/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

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
