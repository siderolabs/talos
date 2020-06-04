// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package file

import (
	"context"
	"io"
	"os"

	"github.com/talos-systems/talos/pkg/chunker"
	"github.com/talos-systems/talos/pkg/chunker/stream"
	"github.com/talos-systems/talos/pkg/follow"
)

// Options is the functional options struct.
type Options struct {
	Size   int
	Follow bool
}

// Option is the functional option func.
type Option func(*Options)

// WithSize sets the chunk size of the Chunker.
func WithSize(s int) Option {
	return func(args *Options) {
		args.Size = s
	}
}

// WithFollow file updates using inotify().
func WithFollow() Option {
	return func(args *Options) {
		args.Follow = true
	}
}

// Source is an interface describing the source of a File.
type Source = *os.File

// NewChunker initializes a Chunker with default values.
func NewChunker(ctx context.Context, source Source, setters ...Option) chunker.Chunker {
	opts := &Options{
		Size: 1024,
	}

	for _, setter := range setters {
		setter(opts)
	}

	var r io.ReadCloser = source

	if opts.Follow {
		r = follow.NewReader(ctx, source)
	}

	return stream.NewChunker(ctx, r, stream.Size(opts.Size))
}
