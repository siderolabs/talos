// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// ContentLibrarySuite verifies the content library: the ContentLibraryConfig document, the
// ContentLibraryStatus resource and the machine.ContentLibraryService API.
type ContentLibrarySuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *ContentLibrarySuite) SuiteName() string {
	return "hypervisor.ContentLibrarySuite"
}

// SetupTest ...
func (suite *ContentLibrarySuite) SetupTest() {
	if !suite.Capabilities().SupportsVolumes {
		suite.T().Skip("cluster doesn't support volumes")
	}

	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 5*time.Minute)
}

// TearDownTest ...
func (suite *ContentLibrarySuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// provisionLibrary declares a content library on a directory user volume and waits for it to come up.
//
// It returns a context bound to the node holding the library, the library's ID and the path its
// contents sit at. The library and the volume backing it are removed when the test ends.
func (suite *ContentLibrarySuite) provisionLibrary() (context.Context, string, string) {
	if testing.Short() {
		suite.T().Skip("skipping test in short mode.")
	}

	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping test for non-qemu provisioner")
	}

	node := suite.RandomDiscoveredNodeInternalIP()

	ctx := client.WithNode(suite.ctx, node)

	name, path := provisionContentLibrary(&suite.APISuite, ctx, node)

	return ctx, name, path
}

// TestLibraryBecomesReady covers a library declared on a user volume being reported as ready, and
// pointed at the volume it was declared on.
func (suite *ContentLibrarySuite) TestLibraryBecomesReady() {
	ctx, name, path := suite.provisionLibrary()

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.State(), []string{name},
		func(cls *hypervisor.ContentLibraryStatus, asrt *assert.Assertions) {
			asrt.Equal(name, cls.TypedSpec().VolumeName)
			asrt.Equal(constants.UserVolumePrefix+name, cls.TypedSpec().VolumeID, "deprecated field still populated for older clients") //nolint:staticcheck
			asrt.Equal(path, cls.TypedSpec().Path)
		},
	)

	// An empty library lists nothing, rather than failing.
	suite.Assert().Empty(suite.list(ctx, name))
}

// TestUploadListDelete covers the contents of a library over its whole life: uploaded, listed,
// stored on the volume and deleted again.
func (suite *ContentLibrarySuite) TestUploadListDelete() {
	ctx, name, path := suite.provisionLibrary()

	contents := bytes.Repeat([]byte("talos"), 1024)

	resp, err := suite.Client.ContentLibraryUpload(ctx, name, "image.raw", "", bytes.NewReader(contents))
	suite.Require().NoError(err)
	suite.Assert().Equal("image.raw", resp.GetName())
	suite.Assert().Equal(uint64(len(contents)), resp.GetSize())

	files := suite.list(ctx, name)
	suite.Require().Len(files, 1)
	suite.Assert().Equal([]string{"image.raw"}, files[0].GetNames())
	suite.Assert().Equal(digest.SHA256.FromBytes(contents).String(), files[0].GetSha256())
	suite.Assert().Equal(uint64(len(contents)), files[0].GetSize())

	// Both the name and immutable SHA256 object expose the uploaded bytes.
	suite.Assert().Equal(string(contents), suite.ReadFile(ctx, contentLibraryObjectPath(path, digest.SHA256.FromBytes(contents))))
	suite.Assert().Equal(string(contents), suite.ReadFile(ctx, filepath.Join(path, "image.raw")))

	suite.Require().NoError(suite.delete(ctx, name, "image.raw"))

	files = suite.list(ctx, name)
	suite.Require().Len(files, 1)
	suite.Assert().Empty(files[0].GetNames())
	suite.Assert().Equal(digest.SHA256.FromBytes(contents).String(), files[0].GetSha256())

	// Deleting what is no longer there is an error, not a silent success.
	suite.Assert().Equal(codes.NotFound, status.Code(suite.delete(ctx, name, "image.raw")))
}

// TestNoOverwrite covers both identical and different bytes landing on an existing name.
func (suite *ContentLibrarySuite) TestNoOverwrite() {
	ctx, name, path := suite.provisionLibrary()

	contents := bytes.Repeat([]byte("talos"), 1024)

	_, err := suite.Client.ContentLibraryUpload(ctx, name, "image.raw", "", bytes.NewReader(contents))
	suite.Require().NoError(err)

	_, err = suite.Client.ContentLibraryUpload(ctx, name, "image.raw", "", bytes.NewReader(contents))
	suite.Assert().Equal(codes.AlreadyExists, status.Code(err))

	suite.Assert().Equal(string(contents), suite.ReadFile(ctx, filepath.Join(path, "image.raw")))

	replacement := bytes.Repeat([]byte("sidero"), 512)

	_, err = suite.Client.ContentLibraryUpload(ctx, name, "image.raw", "", bytes.NewReader(replacement))
	suite.Assert().Equal(codes.AlreadyExists, status.Code(err))

	suite.Assert().Equal(string(contents), suite.ReadFile(ctx, filepath.Join(path, "image.raw")))
}

// TestUploadDigest covers an upload which declares the digest of what it is carrying: the node
// hashes what it received and only then stores it.
func (suite *ContentLibrarySuite) TestUploadDigest() {
	ctx, name, path := suite.provisionLibrary()

	contents := bytes.Repeat([]byte("talos"), 1024)

	for _, algorithm := range []digest.Algorithm{digest.SHA256} {
		fileName := algorithm.String() + ".raw"

		resp, err := suite.Client.ContentLibraryUpload(ctx, name, fileName, algorithm.FromBytes(contents).String(), bytes.NewReader(contents))
		suite.Require().NoError(err)
		suite.Assert().Equal(uint64(len(contents)), resp.GetSize())

		// The canonical SHA256 identity survives the trip through apid.
		suite.Assert().Equal(digest.SHA256.FromBytes(contents).String(), resp.GetDigests().GetSha256())

		suite.Assert().Equal(string(contents), suite.ReadFile(ctx, filepath.Join(path, fileName)))
	}
}

// TestUploadReportsDigests covers an upload which declared nothing: the node still says what it
// received, which is the only way a client which does not know the digest of an image can learn
// the one to pin it to later.
func (suite *ContentLibrarySuite) TestUploadReportsDigests() {
	ctx, name, _ := suite.provisionLibrary()

	contents := bytes.Repeat([]byte("talos"), 1024)

	resp, err := suite.Client.ContentLibraryUpload(ctx, name, "image.raw", "", bytes.NewReader(contents))
	suite.Require().NoError(err)

	suite.Assert().Equal(digest.SHA256.FromBytes(contents).String(), resp.GetDigests().GetSha256())
}

// TestUploadDigestMismatch covers contents which are not what the upload declared: none of them may
// reach the library, under the name asked for or any other.
func (suite *ContentLibrarySuite) TestUploadDigestMismatch() {
	ctx, name, _ := suite.provisionLibrary()

	contents := bytes.Repeat([]byte("talos"), 1024)

	resp, err := suite.Client.ContentLibraryUpload(ctx, name, "image.raw", digest.FromString("something else").String(), bytes.NewReader(contents))
	suite.Assert().Equal(codes.DataLoss, status.Code(err))

	// The refusal is the only thing which can carry them, and it has to survive apid to do so.
	digests := client.ContentLibraryUploadDigests(resp, err)
	suite.Require().NotNil(digests)
	suite.Assert().Equal(digest.SHA256.FromBytes(contents).String(), digests.GetSha256())

	suite.Assert().Empty(suite.list(ctx, name))
}

// TestUploadRejectsDigests covers digests nothing could be verified against.
func (suite *ContentLibrarySuite) TestUploadRejectsDigests() {
	ctx := client.WithNode(suite.ctx, suite.RandomDiscoveredNodeInternalIP())

	for _, dgst := range []string{"not-a-digest", "sha384:" + strings.Repeat("a", 96), "sha512:" + strings.Repeat("a", 128)} {
		_, err := suite.Client.ContentLibraryUpload(ctx, "not-a-library", "image.raw", dgst, bytes.NewReader(nil))
		suite.Assert().Equalf(codes.InvalidArgument, status.Code(err), "uploading with the digest %q should have been rejected", dgst)
	}
}

// TestRejectsNames covers the names which must never reach the filesystem: a library is flat, and
// nothing in it is addressable outside it.
func (suite *ContentLibrarySuite) TestRejectsNames() {
	ctx, name, _ := suite.provisionLibrary()

	contents := bytes.Repeat([]byte("talos"), 1024)

	for _, badName := range []string{"../escape.raw", "/etc/passwd", "sub/dir.raw", ".hidden", "objects", ""} {
		_, err := suite.Client.ContentLibraryUpload(ctx, name, badName, "", bytes.NewReader(contents))
		suite.Assert().Equalf(codes.InvalidArgument, status.Code(err), "uploading %q should have been rejected", badName)

		err = suite.delete(ctx, name, badName)
		suite.Assert().Equalf(codes.InvalidArgument, status.Code(err), "deleting %q should have been rejected", badName)
	}

	// None of them created anything, under the name asked for or any other.
	suite.Assert().Empty(suite.list(ctx, name))
}

// TestUnknownLibrary covers a library which is not configured at all: it is not found, rather than
// a path on the node. Needs no library of its own, and so no spare disk.
func (suite *ContentLibrarySuite) TestUnknownLibrary() {
	ctx := client.WithNode(suite.ctx, suite.RandomDiscoveredNodeInternalIP())

	cli, err := suite.Client.ContentLibraryService().List(ctx, &machineapi.ContentLibraryServiceListRequest{
		LibraryId: "not-a-library",
	})
	suite.Require().NoError(err)

	// A server streaming call reports the failure on the first Recv, not when it is started.
	_, err = cli.Recv()
	suite.Assert().Equal(codes.NotFound, status.Code(err))

	_, err = suite.Client.ContentLibraryUpload(ctx, "not-a-library", "image.raw", "", bytes.NewReader(nil))
	suite.Assert().Equal(codes.NotFound, status.Code(err))

	suite.Assert().Equal(codes.NotFound, status.Code(suite.delete(ctx, "not-a-library", "image.raw")))
}

// delete removes a file from a content library on a single node.
func (suite *ContentLibrarySuite) delete(nodeCtx context.Context, libraryID, name string) error {
	_, err := suite.Client.ContentLibraryService().Delete(nodeCtx, &machineapi.ContentLibraryServiceDeleteRequest{
		LibraryId: libraryID,
		Target:    &machineapi.ContentLibraryServiceDeleteRequest_Name{Name: name},
	})

	return err
}

// list returns the contents of a content library on a single node.
func (suite *ContentLibrarySuite) list(nodeCtx context.Context, libraryID string) []*machineapi.ContentLibraryServiceListResponse {
	cli, err := suite.Client.ContentLibraryService().List(nodeCtx, &machineapi.ContentLibraryServiceListRequest{
		LibraryId: libraryID,
	})
	suite.Require().NoError(err)

	var files []*machineapi.ContentLibraryServiceListResponse

	for {
		resp, err := cli.Recv()
		if errors.Is(err, io.EOF) {
			break
		}

		suite.Require().NoError(err)

		files = append(files, resp)
	}

	return files
}

func init() {
	allSuites = append(allSuites, new(ContentLibrarySuite))
}
