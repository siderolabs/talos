// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package contentlibrary implements machine.ContentLibraryService.
package contentlibrary

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/talos/internal/pkg/contentlibrary/staging"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// uploadFileMode is the mode of the files this service creates: nothing but Talos itself reads or
// writes a library, and the images kept in one are not world-readable material.
const uploadFileMode = 0o600

// maxFileNameLength bounds a file name so that the name its upload is staged under still fits in
// NAME_MAX, rather than failing late with ENAMETOOLONG.
const maxFileNameLength = 255 - staging.NameOverhead

// Service implements machine.ContentLibraryService.
type Service struct {
	machine.UnimplementedContentLibraryServiceServer

	state  state.State
	logger *zap.Logger
}

// NewService creates a new ContentLibraryService.
func NewService(state state.State, logger *zap.Logger) *Service {
	return &Service{
		state:  state,
		logger: logger,
	}
}

// openLibrary resolves a library ID to its directory on the backing volume.
//
// The returned root confines every path below to the library directory: names coming in over the
// API are never joined onto it directly.
func (svc *Service) openLibrary(ctx context.Context, libraryID string) (*os.Root, error) {
	if libraryID == "" {
		return nil, status.Error(codes.InvalidArgument, "library_id is required")
	}

	contentLibraryStatus, err := safe.StateGetByID[*hypervisor.ContentLibraryStatus](ctx, svc.state, libraryID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, status.Errorf(codes.NotFound, "content library %q is not configured", libraryID)
		}

		return nil, status.Errorf(codes.Internal, "failed to get content library %q: %v", libraryID, err)
	}

	if !contentLibraryStatus.TypedSpec().Ready {
		return nil, status.Errorf(codes.FailedPrecondition, "content library %q is not ready: %s", libraryID, contentLibraryStatus.TypedSpec().Error)
	}

	root, err := os.OpenRoot(contentLibraryStatus.TypedSpec().Path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to open content library %q: %v", libraryID, err)
	}

	return root, nil
}

// validateName checks a file name supplied over the API.
//
// A library is flat: a name is one path element within it and nothing else. The root already stops
// a traversal from escaping, this makes the rejection explicit rather than letting `../x` turn into
// a file literally named that.
func validateName(name string) error {
	switch {
	case name == "":
		return status.Error(codes.InvalidArgument, "name is required")
	case len(name) > maxFileNameLength:
		return status.Errorf(codes.InvalidArgument, "name %q must be %d characters or fewer", name, maxFileNameLength)
	case strings.ContainsRune(name, os.PathSeparator):
		return status.Errorf(codes.InvalidArgument, "name %q must not contain a path separator", name)
	case strings.HasPrefix(name, staging.Prefix):
		// Leading dots are reserved: uploads in flight are staged under one, and "." and ".." would
		// address the library directory and its parent rather than a file in it.
		return status.Errorf(codes.InvalidArgument, "name %q must not start with a dot", name)
	}

	return nil
}

// List files stored in a content library.
func (svc *Service) List(req *machine.ContentLibraryServiceListRequest, srv grpc.ServerStreamingServer[machine.ContentLibraryServiceListResponse]) error {
	root, err := svc.openLibrary(srv.Context(), req.GetLibraryId())
	if err != nil {
		return err
	}

	defer root.Close() //nolint:errcheck

	dir, err := root.Open(".")
	if err != nil {
		return status.Errorf(codes.Internal, "failed to open content library: %v", err)
	}

	defer dir.Close() //nolint:errcheck

	entries, err := dir.ReadDir(-1)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to read content library: %v", err)
	}

	for _, entry := range entries {
		// Directories, symlinks and uploads in flight are not library contents.
		if !entry.Type().IsRegular() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// Deleted while the directory was being listed.
				continue
			}

			return status.Errorf(codes.Internal, "failed to stat %q: %v", entry.Name(), err)
		}

		if err := srv.Send(&machine.ContentLibraryServiceListResponse{
			Name:       info.Name(),
			Size:       uint64(info.Size()),
			ModifiedAt: timestamppb.New(info.ModTime()),
		}); err != nil {
			return err
		}
	}

	return nil
}

// Upload a file to a content library.
//
//nolint:gocyclo
func (svc *Service) Upload(srv grpc.ClientStreamingServer[machine.ContentLibraryServiceUploadRequest, machine.ContentLibraryServiceUploadResponse]) error {
	msg, err := srv.Recv()
	if err != nil {
		if errors.Is(err, io.EOF) {
			// The client closed the stream without sending anything at all.
			return status.Error(codes.InvalidArgument, "the first message must carry the upload info")
		}

		return err
	}

	info := msg.GetInfo()
	if info == nil {
		return status.Error(codes.InvalidArgument, "the first message must carry the upload info")
	}

	if err = validateName(info.GetName()); err != nil {
		return err
	}

	root, err := svc.openLibrary(srv.Context(), info.GetLibraryId())
	if err != nil {
		return err
	}

	defer root.Close() //nolint:errcheck

	if !info.GetOverwrite() {
		// A cheap early rejection only, so that a large image is not streamed in just to be refused
		// at the end: the check which actually keeps two uploads from clobbering each other is the
		// link in receiveFile.
		if _, err = root.Stat(info.GetName()); err == nil {
			return status.Errorf(codes.AlreadyExists, "file %q already exists", info.GetName())
		} else if !errors.Is(err, fs.ErrNotExist) {
			return status.Errorf(codes.Internal, "failed to stat %q: %v", info.GetName(), err)
		}
	}

	written, err := svc.receiveFile(srv, root, info.GetName(), info.GetOverwrite())
	if err != nil {
		return err
	}

	svc.logger.Info("uploaded a file to a content library",
		zap.String("library", info.GetLibraryId()),
		zap.String("name", info.GetName()),
		zap.Uint64("size", written),
	)

	return srv.SendAndClose(&machine.ContentLibraryServiceUploadResponse{
		Name: info.GetName(),
		Size: written,
	})
}

// receiveFile streams the rest of the request into the library.
//
// The contents are staged under a temporary name and only claim the name they were uploaded under
// once the stream ends, so an interrupted upload leaves no half-written image behind under the name
// it was meant to take.
//
//nolint:gocyclo
func (svc *Service) receiveFile(
	srv grpc.ClientStreamingServer[machine.ContentLibraryServiceUploadRequest, machine.ContentLibraryServiceUploadResponse],
	root *os.Root,
	name string,
	overwrite bool,
) (written uint64, err error) {
	// Whatever a failure leaves staged is swept by hypervisor.ContentLibraryController when it next
	// brings the library up.
	tmpName := staging.Name(name)

	// O_EXCL so that a staging name is never reused: it is drawn at random, and silently writing
	// into another upload's file would be worse than failing.
	f, err := root.OpenFile(tmpName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, uploadFileMode)
	if err != nil {
		return 0, status.Errorf(codes.Internal, "failed to create %q: %v", name, err)
	}

	defer func() {
		// Cleared once the upload is committed, after which there is nothing staged left to clean
		// up here.
		if err != nil && tmpName != "" {
			f.Close()            //nolint:errcheck
			root.Remove(tmpName) //nolint:errcheck
		}
	}()

	for {
		var msg *machine.ContentLibraryServiceUploadRequest

		msg, err = srv.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return 0, err
		}

		chunk := msg.GetChunk()
		if chunk == nil {
			err = status.Error(codes.InvalidArgument, "only the first message may carry the upload info")

			return 0, err
		}

		var n int

		if n, err = f.Write(chunk.GetBytes()); err != nil {
			return 0, status.Errorf(codes.Internal, "failed to write %q: %v", name, err)
		}

		written += uint64(n)
	}

	// Virtual machine images are expected to survive a reboot of the node they were uploaded to, so
	// both the contents and the directory entry naming them are flushed before the upload is
	// reported as successful.
	if err = f.Sync(); err != nil {
		return 0, status.Errorf(codes.Internal, "failed to flush %q: %v", name, err)
	}

	if err = f.Close(); err != nil {
		return 0, status.Errorf(codes.Internal, "failed to close %q: %v", name, err)
	}

	if overwrite {
		if err = root.Rename(tmpName, name); err != nil {
			return 0, status.Errorf(codes.Internal, "failed to rename %q: %v", name, err)
		}

		// Cleared: the rename consumed the staged name, there is nothing left to clean up.
		tmpName = ""
	} else {
		// A link fails if the name is taken, which makes claiming it atomic: unlike the stat in
		// Upload, two concurrent uploads of the same name cannot both get past this one.
		if err = root.Link(tmpName, name); err != nil {
			if errors.Is(err, fs.ErrExist) {
				return 0, status.Errorf(codes.AlreadyExists, "file %q already exists", name)
			}

			return 0, status.Errorf(codes.Internal, "failed to link %q: %v", name, err)
		}
	}

	// Both failures below leave the upload in place under the name asked for, so neither is
	// reported to the client: it is already stored, and a retry would only be refused as already
	// existing.
	if syncErr := syncDir(root); syncErr != nil {
		// The contents are flushed, only the directory entry naming them might not survive a power
		// loss.
		svc.logger.Error("failed to flush a content library after an upload",
			zap.String("name", name),
			zap.Error(syncErr),
		)
	}

	if tmpName != "" {
		// The link left the staged file behind. Whatever is not cleaned up here is swept by
		// hypervisor.ContentLibraryController when it next brings the library up.
		if rmErr := root.Remove(tmpName); rmErr != nil {
			svc.logger.Error("failed to remove a staged upload",
				zap.String("name", tmpName),
				zap.Error(rmErr),
			)
		}

		tmpName = ""
	}

	return written, nil
}

// syncDir flushes the library directory itself, so that a name created in it survives a power loss.
func syncDir(root *os.Root) error {
	dir, err := root.Open(".")
	if err != nil {
		return fmt.Errorf("failed to open content library: %w", err)
	}

	defer dir.Close() //nolint:errcheck

	if err := dir.Sync(); err != nil {
		return fmt.Errorf("failed to flush content library: %w", err)
	}

	return nil
}

// Delete a file from a content library.
func (svc *Service) Delete(ctx context.Context, req *machine.ContentLibraryServiceDeleteRequest) (*machine.ContentLibraryServiceDeleteResponse, error) {
	if err := validateName(req.GetName()); err != nil {
		return nil, err
	}

	root, err := svc.openLibrary(ctx, req.GetLibraryId())
	if err != nil {
		return nil, err
	}

	defer root.Close() //nolint:errcheck

	if err := root.Remove(req.GetName()); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, status.Errorf(codes.NotFound, "file %q not found", req.GetName())
		}

		return nil, status.Errorf(codes.Internal, "failed to delete %q: %v", req.GetName(), err)
	}

	svc.logger.Info("deleted a file from a content library",
		zap.String("library", req.GetLibraryId()),
		zap.String("name", req.GetName()),
	)

	return &machine.ContentLibraryServiceDeleteResponse{}, nil
}
