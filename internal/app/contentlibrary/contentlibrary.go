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
	"sort"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/opencontainers/go-digest"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cas "github.com/siderolabs/talos/internal/pkg/contentlibrary"
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

	if contentLibraryStatus.TypedSpec().Phase != hypervisor.ContentLibraryPhaseReady {
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
	case strings.ContainsAny(name, "/\\\x00"):
		return status.Errorf(codes.InvalidArgument, "name %q must not contain path separators or NUL", name)
	case name == "objects":
		return status.Errorf(codes.InvalidArgument, "name %q is reserved for object storage", name)
	case strings.HasPrefix(name, staging.Prefix):
		// Leading dots are reserved: uploads in flight are staged under one, and "." and ".." would
		// address the library directory and its parent rather than a file in it.
		return status.Errorf(codes.InvalidArgument, "name %q must not start with a dot", name)
	}

	return nil
}

// parseDigest checks the digest an upload declared, if it declared one.
//
// An empty digest is what a client which does not know the digest of what it is uploading sends,
// and does not assert an expected identity.
func parseDigest(s string) (digest.Digest, error) {
	if s == "" {
		return "", nil
	}

	dgst, err := digest.Parse(s)
	if err != nil {
		return "", status.Errorf(codes.InvalidArgument, "digest %q is invalid: %v", s, err)
	}

	if dgst.Algorithm() != digest.SHA256 {
		return "", status.Errorf(
			codes.InvalidArgument,
			"digest algorithm %q is not supported, expected sha256",
			dgst.Algorithm(),
		)
	}

	return dgst, nil
}

// uploadSink writes staged bytes and computes their SHA256 identity in the same pass.
type uploadSink struct {
	io.Writer

	digester digest.Digester
}

// newUploadSink returns a sink writing to w and hashing what passes through it.
func newUploadSink(w io.Writer) uploadSink {
	digester := digest.SHA256.Digester()

	return uploadSink{
		Writer:   io.MultiWriter(w, digester.Hash()),
		digester: digester,
	}
}

// digests returns the digests of everything written to the sink.
func (sink uploadSink) digests() *machine.ContentLibraryServiceUploadDigests {
	return &machine.ContentLibraryServiceUploadDigests{
		Sha256: sink.digester.Digest().String(),
	}
}

// verify reports whether what was written is what the upload declared it would be.
//
// expected has been through parseDigest and can only be SHA256.
func (sink uploadSink) verify(expected digest.Digest) error {
	if expected == "" {
		return nil
	}

	if actual := sink.digester.Digest(); actual != expected {
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

// List immutable objects and their attached names.
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

	aliases := map[string][]string{}

	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			object, resolveErr := cas.Resolve(root, entry.Name())
			if resolveErr == nil {
				aliases[object] = append(aliases[object], entry.Name())
			}
		}
	}

	return listObjects(root, aliases, srv)
}

func listObjects(root *os.Root, aliases map[string][]string, srv grpc.ServerStreamingServer[machine.ContentLibraryServiceListResponse]) error {
	objects, err := root.Open("objects/sha256")
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	if err != nil {
		return status.Errorf(codes.Internal, "failed to list objects: %v", err)
	}

	defer objects.Close() //nolint:errcheck

	stored, err := objects.ReadDir(-1)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to read objects: %v", err)
	}

	sort.Slice(stored, func(i, j int) bool { return stored[i].Name() < stored[j].Name() })

	for _, entry := range stored {
		if !entry.Type().IsRegular() {
			continue
		}

		object, pathErr := cas.ObjectPath("sha256:" + entry.Name())
		if pathErr != nil {
			continue
		}

		info, infoErr := entry.Info()
		if infoErr != nil {
			return status.Errorf(codes.Internal, "failed to stat object: %v", infoErr)
		}

		names := aliases[object]
		sort.Strings(names)

		if err := srv.Send(&machine.ContentLibraryServiceListResponse{
			Sha256: "sha256:" + entry.Name(),
			Names:  names,
			Size:   uint64(info.Size()),
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

	{
		if _, err = root.Lstat(info.GetName()); err == nil {
			return status.Errorf(codes.AlreadyExists, "name %q already exists", info.GetName())
		} else if !errors.Is(err, fs.ErrNotExist) {
			return status.Errorf(codes.Internal, "failed to stat %q: %v", info.GetName(), err)
		}
	}

	written, digests, err := svc.receiveFile(srv, root, info.GetLibraryId(), info.GetName(), dgst)
	if err != nil {
		return withDigests(err, digests)
	}

	svc.logger.Info("uploaded a file to a content library",
		zap.String("library", info.GetLibraryId()),
		zap.String("name", info.GetName()),
		zap.Uint64("size", written),
		zap.String("sha256", digests.GetSha256()),
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
	libraryID string,
	name string,
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

	if err = svc.claimName(srv.Context(), root, libraryID, name, tmpName, digests.GetSha256()); err != nil {
		return 0, digests, err
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

// claimName publishes immutable bytes and atomically claims a create-only name.
func (svc *Service) claimName(ctx context.Context, root *os.Root, libraryID, name, tmpName, hash string) error {
	if err := ctx.Err(); err != nil {
		return status.FromContextError(err).Err()
	}

	release, err := svc.claimLibraryFile(ctx, libraryID, name)
	if err != nil {
		return err
	}
	defer release()

	if err := cas.Publish(root, tmpName, name, hash); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return status.Errorf(codes.AlreadyExists, "name %q already exists", name)
		}

		return status.Errorf(codes.Internal, "failed to publish %q: %v", name, err)
	}

	return nil
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

// claimLibraryFile claims the right to mutate one file of a library.
//
// Returns a callable to be used for releasing the claim, once the mutation is done.
func (svc *Service) claimLibraryFile(ctx context.Context, libraryID, name string) (func(), error) {
	md := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryID).Metadata()

	current, err := svc.state.Get(ctx, md)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, status.Errorf(codes.NotFound, "content library %q is not configured", libraryID)
		}

		return nil, status.Errorf(codes.Internal, "failed to get content library %q: %v", libraryID, err)
	}

	finalizer := hypervisor.ContentLibraryMutationFinalizer(name)
	alreadyLocked := status.Errorf(codes.FailedPrecondition, "file %q is already being changed by another request", name)

	// state.AddFinalizer open-coded, for the already-claimed check to be atomic with the add, and
	// for the phase expectation: a library on its way out is refused rather than marked, as a
	// marker on it would block that teardown for as long as it is held.
	_, err = svc.state.UpdateWithConflicts(ctx, md, func(r resource.Resource) error {
		if r.Metadata().Finalizers().Has(finalizer) {
			return alreadyLocked
		}

		r.Metadata().Finalizers().Add(finalizer)

		return nil
	}, state.WithUpdateOwner(current.Metadata().Owner()), state.WithExpectedPhase(resource.PhaseRunning))

	switch {
	case errors.Is(err, alreadyLocked):
		return nil, alreadyLocked
	case state.IsPhaseConflictError(err):
		return nil, status.Errorf(codes.FailedPrecondition, "content library %q is going away", libraryID)
	case err != nil:
		return nil, status.Errorf(codes.Internal, "failed to claim %q in content library %q: %v", name, libraryID, err)
	}

	return func() {
		if err := svc.state.RemoveFinalizer(context.WithoutCancel(ctx), md, finalizer); err != nil && !state.IsNotFoundError(err) {
			svc.logger.Error("failed to release a content library file",
				zap.String("library", libraryID),
				zap.String("name", name),
				zap.Error(err),
			)
		}
	}, nil
}

// Delete removes an alias or an explicitly selected immutable object.
func (svc *Service) Delete(ctx context.Context, req *machine.ContentLibraryServiceDeleteRequest) (*machine.ContentLibraryServiceDeleteResponse, error) {
	switch req.GetTarget().(type) {
	case *machine.ContentLibraryServiceDeleteRequest_Sha256:
		if _, err := cas.ObjectPath(req.GetSha256()); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		return svc.deleteObject(ctx, req)
	case *machine.ContentLibraryServiceDeleteRequest_Name:
		if req.GetForce() {
			return nil, status.Error(codes.InvalidArgument, "force is valid only when deleting a SHA256 object")
		}

		return svc.deleteAlias(ctx, req)
	default:
		return nil, status.Error(codes.InvalidArgument, "an alias name or SHA256 object identity is required")
	}
}

func (svc *Service) deleteObject(ctx context.Context, req *machine.ContentLibraryServiceDeleteRequest) (*machine.ContentLibraryServiceDeleteResponse, error) {
	root, err := svc.openLibrary(ctx, req.GetLibraryId())
	if err != nil {
		return nil, err
	}

	defer root.Close() //nolint:errcheck

	release, err := svc.claimLibraryFile(ctx, req.GetLibraryId(), req.GetSha256())
	if err != nil {
		return nil, err
	}

	defer release()

	if err := cas.RemoveObject(root, req.GetSha256(), req.GetForce()); err != nil {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return nil, status.Errorf(codes.NotFound, "object %q not found", req.GetSha256())
		case errors.Is(err, cas.ErrObjectReferenced):
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		default:
			return nil, status.Errorf(codes.Internal, "failed to delete object %q: %v", req.GetSha256(), err)
		}
	}

	svc.logger.Info("deleted an object from a content library",
		zap.String("library", req.GetLibraryId()),
		zap.String("sha256", req.GetSha256()),
		zap.Bool("force", req.GetForce()),
	)

	return &machine.ContentLibraryServiceDeleteResponse{}, nil
}

// deleteAlias removes a validated root alias without removing its object.
func (svc *Service) deleteAlias(ctx context.Context, req *machine.ContentLibraryServiceDeleteRequest) (*machine.ContentLibraryServiceDeleteResponse, error) {
	if err := validateName(req.GetName()); err != nil {
		return nil, err
	}

	root, err := svc.openLibrary(ctx, req.GetLibraryId())
	if err != nil {
		return nil, err
	}

	defer root.Close() //nolint:errcheck

	if _, err := cas.Resolve(root, req.GetName()); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, status.Errorf(codes.NotFound, "alias %q not found", req.GetName())
		}

		return nil, status.Errorf(codes.FailedPrecondition, "alias %q is not a content library object: %v", req.GetName(), err)
	}

	release, err := svc.claimLibraryFile(ctx, req.GetLibraryId(), req.GetName())
	if err != nil {
		return nil, err
	}

	defer release()

	if err := cas.RemoveAlias(root, req.GetName()); err != nil {
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
