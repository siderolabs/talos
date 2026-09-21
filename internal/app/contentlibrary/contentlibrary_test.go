// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package contentlibrary_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/app/contentlibrary"
	"github.com/siderolabs/talos/internal/pkg/contentlibrary/staging"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

const testLibrary = "vm-images"

// setup returns a service backed by a temporary directory standing in for the library, and that
// directory's path.
func setup(t *testing.T, ready bool) (*contentlibrary.Service, string) {
	t.Helper()

	path := t.TempDir()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	contentLibraryStatus := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, testLibrary)
	contentLibraryStatus.TypedSpec().VolumeID = "u-vm-images"
	contentLibraryStatus.TypedSpec().Ready = ready

	if ready {
		contentLibraryStatus.TypedSpec().Path = path
	} else {
		contentLibraryStatus.TypedSpec().Error = "waiting for volume \"u-vm-images\" to be mounted"
	}

	require.NoError(t, st.Create(t.Context(), contentLibraryStatus))

	return contentlibrary.NewService(st, zaptest.NewLogger(t)), path
}

// listStream collects what List sends.
type listStream struct {
	grpc.ServerStream

	ctx   context.Context //nolint:containedctx
	items []*machine.ContentLibraryServiceListResponse
}

func (s *listStream) Context() context.Context { return s.ctx }

func (s *listStream) Send(resp *machine.ContentLibraryServiceListResponse) error {
	s.items = append(s.items, resp)

	return nil
}

// uploadStream replays a canned sequence of requests to Upload.
type uploadStream struct {
	grpc.ServerStream

	ctx      context.Context //nolint:containedctx
	requests []*machine.ContentLibraryServiceUploadRequest
	response *machine.ContentLibraryServiceUploadResponse
}

func (s *uploadStream) Context() context.Context { return s.ctx }

func (s *uploadStream) Recv() (*machine.ContentLibraryServiceUploadRequest, error) {
	if len(s.requests) == 0 {
		return nil, io.EOF
	}

	req := s.requests[0]
	s.requests = s.requests[1:]

	return req, nil
}

func (s *uploadStream) SendAndClose(resp *machine.ContentLibraryServiceUploadResponse) error {
	s.response = resp

	return nil
}

func uploadRequests(libraryID, name string, overwrite bool, chunks ...string) []*machine.ContentLibraryServiceUploadRequest {
	return uploadRequestsWithDigest(libraryID, name, overwrite, "", chunks...)
}

func uploadRequestsWithDigest(libraryID, name string, overwrite bool, dgst string, chunks ...string) []*machine.ContentLibraryServiceUploadRequest {
	requests := make([]*machine.ContentLibraryServiceUploadRequest, 0, 1+len(chunks))

	requests = append(requests, &machine.ContentLibraryServiceUploadRequest{
		Request: &machine.ContentLibraryServiceUploadRequest_Info{
			Info: &machine.ContentLibraryServiceUploadInfo{
				LibraryId: libraryID,
				Name:      name,
				Overwrite: overwrite,
				Digest:    dgst,
			},
		},
	})

	for _, chunk := range chunks {
		requests = append(requests, &machine.ContentLibraryServiceUploadRequest{
			Request: &machine.ContentLibraryServiceUploadRequest_Chunk{
				Chunk: &common.Data{Bytes: []byte(chunk)},
			},
		})
	}

	return requests
}

func upload(t *testing.T, svc *contentlibrary.Service, name string, overwrite bool, chunks ...string) error {
	t.Helper()

	return uploadWithDigest(t, svc, name, overwrite, "", chunks...)
}

func uploadWithDigest(t *testing.T, svc *contentlibrary.Service, name string, overwrite bool, dgst string, chunks ...string) error {
	t.Helper()

	return svc.Upload(&uploadStream{ctx: t.Context(), requests: uploadRequestsWithDigest(testLibrary, name, overwrite, dgst, chunks...)})
}

func list(t *testing.T, svc *contentlibrary.Service, libraryID string) ([]*machine.ContentLibraryServiceListResponse, error) {
	t.Helper()

	srv := &listStream{ctx: t.Context()}

	err := svc.List(&machine.ContentLibraryServiceListRequest{LibraryId: libraryID}, srv)

	return srv.items, err
}

// TestRoundTrip covers the whole API against a ready library.
func TestRoundTrip(t *testing.T) {
	t.Parallel()

	svc, path := setup(t, true)

	items, err := list(t, svc, testLibrary)
	require.NoError(t, err)
	assert.Empty(t, items)

	srv := &uploadStream{ctx: t.Context(), requests: uploadRequests(testLibrary, "image.raw", false, "talos", "-images")}
	require.NoError(t, svc.Upload(srv))
	assert.Equal(t, "image.raw", srv.response.GetName())
	assert.Equal(t, uint64(12), srv.response.GetSize())

	contents, err := os.ReadFile(filepath.Join(path, "image.raw"))
	require.NoError(t, err)
	assert.Equal(t, "talos-images", string(contents))

	items, err = list(t, svc, testLibrary)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "image.raw", items[0].GetName())
	assert.Equal(t, uint64(12), items[0].GetSize())

	_, err = svc.Delete(t.Context(), &machine.ContentLibraryServiceDeleteRequest{LibraryId: testLibrary, Name: "image.raw"})
	require.NoError(t, err)

	items, err = list(t, svc, testLibrary)
	require.NoError(t, err)
	assert.Empty(t, items)

	_, err = svc.Delete(t.Context(), &machine.ContentLibraryServiceDeleteRequest{LibraryId: testLibrary, Name: "image.raw"})
	assert.Equal(t, codes.NotFound, grpcstatus.Code(err))
}

// TestOverwrite covers an upload landing on a name which is already taken.
func TestOverwrite(t *testing.T) {
	t.Parallel()

	svc, path := setup(t, true)

	err := upload(t, svc, "image.raw", false, "first")
	require.NoError(t, err)

	err = upload(t, svc, "image.raw", false, "second")
	assert.Equal(t, codes.AlreadyExists, grpcstatus.Code(err))

	contents, err := os.ReadFile(filepath.Join(path, "image.raw"))
	require.NoError(t, err)
	assert.Equal(t, "first", string(contents))

	err = upload(t, svc, "image.raw", true, "second")
	require.NoError(t, err)

	contents, err = os.ReadFile(filepath.Join(path, "image.raw"))
	require.NoError(t, err)
	assert.Equal(t, "second", string(contents))

	// Neither the upload which landed nor the one which was refused staged anything which outlived
	// it.
	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "image.raw", entries[0].Name())
}

// TestConcurrentOverwrite covers uploads of the same name racing each other: without overwrite, one
// of them has to lose, rather than both reporting success with one silently clobbering the other.
func TestConcurrentOverwrite(t *testing.T) {
	t.Parallel()

	svc, path := setup(t, true)

	const uploads = 8

	var (
		wg     sync.WaitGroup
		errs   = make([]error, uploads)
		start  = make(chan struct{})
		chunks = make([]string, uploads)
	)

	for i := range uploads {
		chunks[i] = fmt.Sprintf("payload-%d", i)

		wg.Go(func() {
			<-start

			errs[i] = upload(t, svc, "image.raw", false, chunks[i])
		})
	}

	close(start)
	wg.Wait()

	var succeeded int

	for i, err := range errs {
		if err == nil {
			succeeded++

			contents, readErr := os.ReadFile(filepath.Join(path, "image.raw"))
			require.NoError(t, readErr)
			assert.Equal(t, chunks[i], string(contents), "the upload which succeeded is the one stored")

			continue
		}

		assert.Equal(t, codes.AlreadyExists, grpcstatus.Code(err))
	}

	assert.Equal(t, 1, succeeded)

	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "image.raw", entries[0].Name())
}

// TestRejectsNames covers the names which must never reach the filesystem.
func TestRejectsNames(t *testing.T) {
	t.Parallel()

	svc, path := setup(t, true)

	for _, name := range []string{"", "../escape.raw", "/etc/passwd", "sub/dir.raw", ".hidden", ".", ".."} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := upload(t, svc, name, true, "payload")
			assert.Equal(t, codes.InvalidArgument, grpcstatus.Code(err))

			_, err = svc.Delete(t.Context(), &machine.ContentLibraryServiceDeleteRequest{LibraryId: testLibrary, Name: name})
			assert.Equal(t, codes.InvalidArgument, grpcstatus.Code(err))
		})
	}

	// Nothing was created anywhere, inside the library or above it.
	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	assert.Empty(t, entries)

	_, err = os.Stat(filepath.Join(filepath.Dir(path), "escape.raw"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

// TestStaleStagedUploadDoesNotBlock covers what an upload interrupted by a node going down leaves
// behind: it is invisible to the API, so it must not stop the name being uploaded again.
func TestStaleStagedUploadDoesNotBlock(t *testing.T) {
	t.Parallel()

	svc, path := setup(t, true)

	stale := filepath.Join(path, staging.Prefix+"image.raw.0123456789abcdef"+staging.Suffix)
	require.NoError(t, os.WriteFile(stale, []byte("partial"), 0o600))

	require.NoError(t, upload(t, svc, "image.raw", false, "talos"))

	contents, err := os.ReadFile(filepath.Join(path, "image.raw"))
	require.NoError(t, err)
	assert.Equal(t, "talos", string(contents))

	// The leftover is still not library contents; sweeping it is the controller's job.
	items, err := list(t, svc, testLibrary)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "image.raw", items[0].GetName())
}

// TestRejectsLongName covers a name long enough that the name its upload would be staged under no
// longer fits in NAME_MAX: the caller has to hear that, not an opaque internal error.
func TestRejectsLongName(t *testing.T) {
	t.Parallel()

	svc, _ := setup(t, true)

	err := upload(t, svc, strings.Repeat("a", 231), true, "payload")
	assert.Equal(t, codes.InvalidArgument, grpcstatus.Code(err))

	require.NoError(t, upload(t, svc, strings.Repeat("a", 230), true, "payload"))
}

// TestDigestVerified covers an upload which declares what it is carrying: the node hashes what it
// received and stores it, under every algorithm the API offers.
func TestDigestVerified(t *testing.T) {
	t.Parallel()

	const contents = "talos-images"

	for _, algorithm := range []digest.Algorithm{digest.SHA256, digest.SHA512} {
		t.Run(algorithm.String(), func(t *testing.T) {
			t.Parallel()

			svc, path := setup(t, true)

			// Computed rather than written out, so the expectation cannot drift from the chunks
			// below, which are what it has to describe.
			dgst := algorithm.FromString(contents)

			require.NoError(t, uploadWithDigest(t, svc, "image.raw", false, dgst.String(), "talos", "-images"))

			stored, err := os.ReadFile(filepath.Join(path, "image.raw"))
			require.NoError(t, err)
			assert.Equal(t, contents, string(stored))
		})
	}
}

// TestDigestsReported covers what the node says about an upload it was not asked to verify: the
// contents are hashed under both algorithms either way, so a client which did not know the digest
// of what it was uploading learns it.
func TestDigestsReported(t *testing.T) {
	t.Parallel()

	svc, _ := setup(t, true)

	srv := &uploadStream{ctx: t.Context(), requests: uploadRequests(testLibrary, "image.raw", false, "talos", "-images")}
	require.NoError(t, svc.Upload(srv))

	assert.Equal(t, digest.SHA256.FromString("talos-images").String(), srv.response.GetDigests().GetSha256())
	assert.Equal(t, digest.SHA512.FromString("talos-images").String(), srv.response.GetDigests().GetSha512())
}

// TestDigestMismatchReportsDigests covers an upload which was refused telling the client what it
// actually received: there is no response to carry it, so the error has to.
func TestDigestMismatchReportsDigests(t *testing.T) {
	t.Parallel()

	svc, _ := setup(t, true)

	err := uploadWithDigest(t, svc, "image.raw", false, digest.FromString("something else").String(), "talos")
	require.Equal(t, codes.DataLoss, grpcstatus.Code(err))

	details := grpcstatus.Convert(err).Details()
	require.Len(t, details, 1)

	digests, ok := details[0].(*machine.ContentLibraryServiceUploadDigests)
	require.True(t, ok, "the error should carry the digests, got %T", details[0])

	assert.Equal(t, digest.SHA256.FromString("talos").String(), digests.GetSha256())
	assert.Equal(t, digest.SHA512.FromString("talos").String(), digests.GetSha512())
}

// TestDigestMismatch covers contents which are not what the upload said they would be: the name
// they were meant to take stays free, and nothing is left behind under any other.
func TestDigestMismatch(t *testing.T) {
	t.Parallel()

	svc, path := setup(t, true)

	err := uploadWithDigest(t, svc, "image.raw", false, digest.FromString("something else").String(), "talos")
	assert.Equal(t, codes.DataLoss, grpcstatus.Code(err))

	items, err := list(t, svc, testLibrary)
	require.NoError(t, err)
	assert.Empty(t, items)

	// Not even staged: the upload cleans up after itself rather than leaving it for the controller.
	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// TestDigestMismatchDoesNotOverwrite covers why the digest is checked before the staged file claims
// its name: an upload which turns out to be corrupt must not have destroyed the file it was
// replacing.
func TestDigestMismatchDoesNotOverwrite(t *testing.T) {
	t.Parallel()

	svc, path := setup(t, true)

	require.NoError(t, upload(t, svc, "image.raw", false, "first"))

	err := uploadWithDigest(t, svc, "image.raw", true, digest.FromString("first").String(), "second")
	assert.Equal(t, codes.DataLoss, grpcstatus.Code(err))

	contents, err := os.ReadFile(filepath.Join(path, "image.raw"))
	require.NoError(t, err)
	assert.Equal(t, "first", string(contents))

	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "image.raw", entries[0].Name())
}

// TestRejectsDigests covers the digests which no contents could ever match: they are the caller's
// mistake, and saying so is not the same as saying the upload was corrupt.
func TestRejectsDigests(t *testing.T) {
	t.Parallel()

	svc, path := setup(t, true)

	// In a cleanup so that it runs once the parallel subtests below are done, rather than the
	// moment the loop has finished starting them, which would make it assert nothing.
	t.Cleanup(func() {
		entries, err := os.ReadDir(path)
		assert.NoError(t, err)
		assert.Empty(t, entries)
	})

	for _, dgst := range []string{
		"sha256:",
		"sha256:" + strings.Repeat("z", 64),
		"sha256:abcd",
		// A bare hex string names no algorithm, so there is nothing to hash with.
		strings.Repeat("a", 64),
		"md5:" + strings.Repeat("a", 32),
		"sha1:" + strings.Repeat("a", 40),
		// Registered by go-digest, deliberately not offered here.
		"sha384:" + strings.Repeat("a", 96),
		"SHA256:" + strings.Repeat("a", 64),
	} {
		t.Run(dgst, func(t *testing.T) {
			t.Parallel()

			err := uploadWithDigest(t, svc, "image.raw", true, dgst, "talos")
			assert.Equal(t, codes.InvalidArgument, grpcstatus.Code(err))
		})
	}
}

// TestInterruptedUploadLeavesNothing covers an upload which never completes: the name it was
// meant to take must stay free, and the staged contents must not be listed.
func TestInterruptedUploadLeavesNothing(t *testing.T) {
	t.Parallel()

	svc, path := setup(t, true)

	// A second message which is not a chunk aborts the upload.
	srv := &uploadStream{
		ctx: t.Context(),
		requests: append(
			uploadRequests(testLibrary, "image.raw", false),
			&machine.ContentLibraryServiceUploadRequest{
				Request: &machine.ContentLibraryServiceUploadRequest_Info{
					Info: &machine.ContentLibraryServiceUploadInfo{LibraryId: testLibrary, Name: "image.raw"},
				},
			},
		),
	}

	err := svc.Upload(srv)
	assert.Equal(t, codes.InvalidArgument, grpcstatus.Code(err))

	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// TestEmptyStream covers a client which closes the stream without sending anything: the upload info
// never arrived, which is the client's fault and has to be said so.
func TestEmptyStream(t *testing.T) {
	t.Parallel()

	svc, _ := setup(t, true)

	err := svc.Upload(&uploadStream{ctx: t.Context()})
	assert.Equal(t, codes.InvalidArgument, grpcstatus.Code(err))
}

// TestNotReady covers a library whose volume is not mounted yet: it exists, but there is nowhere to
// read or write.
func TestNotReady(t *testing.T) {
	t.Parallel()

	svc, _ := setup(t, false)

	_, err := list(t, svc, testLibrary)
	assert.Equal(t, codes.FailedPrecondition, grpcstatus.Code(err))

	err = upload(t, svc, "image.raw", false, "payload")
	assert.Equal(t, codes.FailedPrecondition, grpcstatus.Code(err))
}

// TestUnknownLibrary covers a library which is not configured at all.
func TestUnknownLibrary(t *testing.T) {
	t.Parallel()

	svc, _ := setup(t, true)

	_, err := list(t, svc, "not-a-library")
	assert.Equal(t, codes.NotFound, grpcstatus.Code(err))

	_, err = list(t, svc, "")
	assert.Equal(t, codes.InvalidArgument, grpcstatus.Code(err))
}
