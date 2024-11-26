// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package registry

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/errdefs"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/siderolabs/gen/xerrors"
)

type singleFileStore struct {
	root fs.StatFS
	path string
}

// Info implements [content.InfoProvider] reading method.
func (s *singleFileStore) Info(_ context.Context, dgst digest.Digest) (content.Info, error) {
	p, err := s.blobPath(dgst)
	if err != nil {
		return content.Info{}, fmt.Errorf("calculating blob info path: %w", err)
	}

	fi, err := s.root.Stat(p)
	if err != nil {
		if os.IsNotExist(err) || errors.Is(err, errdefs.ErrNotFound) {
			return content.Info{}, xerrors.NewTaggedf[notFoundTag]("content '%s': %w", dgst, errdefs.ErrNotFound)
		}

		return content.Info{}, xerrors.NewTagged[internalErrorTag](err)
	}

	return content.Info{
		Digest:    dgst,
		Size:      fi.Size(),
		CreatedAt: fi.ModTime(),
		UpdatedAt: getATime(fi),
	}, nil
}

// ReaderAt implements [content.Provider] reading method.
func (s *singleFileStore) ReaderAt(_ context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	p, err := s.blobPath(desc.Digest)
	if err != nil {
		return nil, fmt.Errorf("calculating blob path for ReaderAt: %w", err)
	}

	reader, err := openReaderAt(p, s.root)
	if err != nil {
		return nil, fmt.Errorf("blob '%s' expected at '%s': %w", desc.Digest, p, err)
	}

	return reader, nil
}

// Status implements [content.IngestManager] ingesting which we don't need.
func (s *singleFileStore) Status(context.Context, string) (content.Status, error) {
	return content.Status{}, errUnimplemented
}

// ListStatuses implements [content.IngestManager] ingesting which we don't need.
func (s *singleFileStore) ListStatuses(context.Context, ...string) ([]content.Status, error) {
	return nil, errUnimplemented
}

// Abort implements [content.IngestManager] ingesting which we don't need.
func (s *singleFileStore) Abort(context.Context, string) error { return errUnimplemented }

// Writer implements [content.Ingester] ingesting which we don't need.
func (s *singleFileStore) Writer(context.Context, ...content.WriterOpt) (content.Writer, error) {
	return nil, errUnimplemented
}

// Walk implements [content.Manager] ingesting which we don't need.
func (s *singleFileStore) Walk(context.Context, content.WalkFunc, ...string) error {
	return errUnimplemented
}

// Delete implements [content.Manager] ingesting which we don't need.
func (s *singleFileStore) Delete(context.Context, digest.Digest) error { return errUnimplemented }

// Update implements [content.Manager] ingesting which we don't need.
func (s *singleFileStore) Update(context.Context, content.Info, ...string) (content.Info, error) {
	return content.Info{}, errUnimplemented
}

func (s *singleFileStore) blobPath(dgst digest.Digest) (string, error) {
	if err := dgst.Validate(); err != nil {
		return "", fmt.Errorf("cannot calculate blob path from invalid digest: %v: %w", err, errdefs.ErrInvalidArgument)
	}

	return filepath.Join(s.path, dgst.String()), nil
}

var errUnimplemented = errors.New("unimplemented")

func getATime(fi os.FileInfo) time.Time {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return time.Unix(st.Atim.Unix())
	}

	return fi.ModTime()
}
