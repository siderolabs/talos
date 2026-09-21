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
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/opencontainers/go-digest"
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

var supportedDigestAlgorithms = []digest.Algorithm{digest.SHA256, digest.SHA512}

// parseDigest checks the digest an upload declared, if it declared one.
//
// An empty digest is what a client which does not know the digest of what it is uploading sends,
// and leaves the contents unverified.
func parseDigest(s string) (digest.Digest, error) {
	if s == "" {
		return "", nil
	}

	dgst, err := digest.Parse(s)
	if err != nil {
		return "", status.Errorf(codes.InvalidArgument, "digest %q is invalid: %v", s, err)
	}

	if !slices.Contains(supportedDigestAlgorithms, dgst.Algorithm()) {
		return "", status.Errorf(
			codes.InvalidArgument,
			"digest algorithm %q is not supported, expected one of %v",
			dgst.Algorithm(), supportedDigestAlgorithms,
		)
	}

	return dgst, nil
}

// uploadSink is where the chunks of an upload are written: the file it is staged in, and a hash of
// everything which goes into it under each algorithm the API can verify.
//
// Every upload is hashed under all of them, whether or not it declared a digest, so that what the
// node received is reported back either way: a client which did not know the digest of what it was
// uploading learns it, and one which did learns what it actually sent instead.
type uploadSink struct {
	io.Writer

	digesters map[digest.Algorithm]digest.Digester
}

// newUploadSink returns a sink writing to w and hashing what passes through it.
func newUploadSink(w io.Writer) uploadSink {
	sink := uploadSink{digesters: make(map[digest.Algorithm]digest.Digester, len(supportedDigestAlgorithms))}

	writers := make([]io.Writer, 0, 1+len(supportedDigestAlgorithms))
	writers = append(writers, w)

	for _, algorithm := range supportedDigestAlgorithms {
		digester := algorithm.Digester()

		sink.digesters[algorithm] = digester
		writers = append(writers, digester.Hash())
	}

	sink.Writer = io.MultiWriter(writers...)

	return sink
}

// digests returns the digests of everything written to the sink.
func (sink uploadSink) digests() *machine.ContentLibraryServiceUploadDigests {
	return &machine.ContentLibraryServiceUploadDigests{
		Sha256: sink.digesters[digest.SHA256].Digest().String(),
		Sha512: sink.digesters[digest.SHA512].Digest().String(),
	}
}

// verify reports whether what was written is what the upload declared it would be.
//
// dgst has been through parseDigest, so its algorithm is one the sink hashed under.
func (sink uploadSink) verify(expected digest.Digest) error {
	if expected == "" {
		return nil
	}

	if actual := sink.digesters[expected.Algorithm()].Digest(); actual != expected {
		return status.Errorf(codes.DataLoss, "uploaded content's digest (%s) doesn't match the expected one (%s)", actual, expected)
	}

	return nil
}

// withDigests attaches the digests of an upload which completed to the error which refused it.
//
// A failed call carries no response, so this is the only way the client is told what the node
// actually received, which is what it needs to tell a corrupted image from a mistyped digest.
func withDigests(err error, digests *machine.ContentLibraryServiceUploadDigests) error {
	if digests == nil {
		// The upload never got as far as being hashed in full, so there is nothing to report.
		return err
	}

	st, ok := status.FromError(err)
	if !ok {
		return err
	}

	withDetails, detailsErr := st.WithDetails(digests)
	if detailsErr != nil {
		// Whatever the status could not be made to carry, the refusal itself still has to reach
		// the client.
		return err
	}

	return withDetails.Err()
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

	// Before the library is even resolved: a digest which cannot be met by any contents is the
	// client's mistake, and saying so costs nothing.
	dgst, err := parseDigest(info.GetDigest())
	if err != nil {
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

	written, digests, err := svc.receiveFile(srv, root, info.GetName(), info.GetOverwrite(), dgst)
	if err != nil {
		return withDigests(err, digests)
	}

	svc.logger.Info("uploaded a file to a content library",
		zap.String("library", info.GetLibraryId()),
		zap.String("name", info.GetName()),
		zap.Uint64("size", written),
		zap.String("sha256", digests.GetSha256()),
		zap.String("sha512", digests.GetSha512()),
	)

	return srv.SendAndClose(&machine.ContentLibraryServiceUploadResponse{
		Name:    info.GetName(),
		Size:    written,
		Digests: digests,
	})
}

// receiveFile streams the rest of the request into the library.
//
// The contents are staged under a temporary name and only claim the name they were uploaded under
// once the stream ends, so an interrupted upload leaves no half-written image behind under the name
// it was meant to take. A non-empty dgst is checked before the staged file claims that name, so
// contents which do not match it are never visible in the library at all.
//
//nolint:gocyclo
func (svc *Service) receiveFile(
	srv grpc.ClientStreamingServer[machine.ContentLibraryServiceUploadRequest, machine.ContentLibraryServiceUploadResponse],
	root *os.Root,
	name string,
	overwrite bool,
	dgst digest.Digest,
) (written uint64, digests *machine.ContentLibraryServiceUploadDigests, err error) {
	// Whatever a failure leaves staged is swept by hypervisor.ContentLibraryController when it next
	// brings the library up.
	tmpName := staging.Name(name)

	// O_EXCL so that a staging name is never reused: it is drawn at random, and silently writing
	// into another upload's file would be worse than failing.
	f, err := root.OpenFile(tmpName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, uploadFileMode)
	if err != nil {
		return 0, nil, status.Errorf(codes.Internal, "failed to create %q: %v", name, err)
	}

	defer func() {
		// Cleared once the upload is committed, after which there is nothing staged left to clean
		// up here.
		if err != nil && tmpName != "" {
			f.Close()            //nolint:errcheck
			root.Remove(tmpName) //nolint:errcheck
		}
	}()

	// The contents are hashed as they are written, so reporting and verifying them costs one pass
	// over the stream and the hash state, whatever the size of the image.
	sink := newUploadSink(f)

	for {
		var msg *machine.ContentLibraryServiceUploadRequest

		msg, err = srv.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return 0, nil, err
		}

		chunk := msg.GetChunk()
		if chunk == nil {
			err = status.Error(codes.InvalidArgument, "only the first message may carry the upload info")

			return 0, nil, err
		}

		var n int

		if n, err = sink.Write(chunk.GetBytes()); err != nil {
			return 0, nil, status.Errorf(codes.Internal, "failed to write %q: %v", name, err)
		}

		written += uint64(n)
	}

	// The upload has completed, so from here on every outcome reports what the node received,
	// whether it went on to store it or not.
	digests = sink.digests()

	// Before anything is flushed or the name is claimed: contents which are not what they were
	// declared to be never become library contents, and the staged file the deferred cleanup above
	// removes is all they leave behind.
	if err = sink.verify(dgst); err != nil {
		return 0, digests, err
	}

	// Virtual machine images are expected to survive a reboot of the node they were uploaded to, so
	// both the contents and the directory entry naming them are flushed before the upload is
	// reported as successful.
	if err = f.Sync(); err != nil {
		return 0, digests, status.Errorf(codes.Internal, "failed to flush %q: %v", name, err)
	}

	if err = f.Close(); err != nil {
		return 0, digests, status.Errorf(codes.Internal, "failed to close %q: %v", name, err)
	}

	if overwrite {
		if err = root.Rename(tmpName, name); err != nil {
			return 0, digests, status.Errorf(codes.Internal, "failed to rename %q: %v", name, err)
		}

		// Cleared: the rename consumed the staged name, there is nothing left to clean up.
		tmpName = ""
	} else {
		// A link fails if the name is taken, which makes claiming it atomic: unlike the stat in
		// Upload, two concurrent uploads of the same name cannot both get past this one.
		if err = root.Link(tmpName, name); err != nil {
			if errors.Is(err, fs.ErrExist) {
				return 0, digests, status.Errorf(codes.AlreadyExists, "file %q already exists", name)
			}

			return 0, digests, status.Errorf(codes.Internal, "failed to link %q: %v", name, err)
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

	return written, digests, nil
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
