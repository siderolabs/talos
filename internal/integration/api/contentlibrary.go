// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
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
	return "api.ContentLibrarySuite"
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

	// Randomized so repeated runs against the same cluster don't collide on a leftover volume.
	name := fmt.Sprintf("cl-%04x", rand.Int31())
	volumeID := constants.UserVolumePrefix + name

	suite.T().Logf("testing the content library %q on node %s", name, node)

	// A directory volume, not a partition: none of these tests store more than a few kilobytes, and a
	// partition would claim a spare disk which the storage tests need, and which removing the document
	// does not hand back.
	volumeDoc := blockcfg.NewUserVolumeConfigV1Alpha1()
	volumeDoc.MetaName = name
	volumeDoc.VolumeType = new(block.VolumeTypeDirectory)

	libraryDoc := hypervisorcfg.NewContentLibraryConfigV1Alpha1()
	libraryDoc.MetaName = name
	libraryDoc.BackingConfig.VolumeName = name

	suite.PatchMachineConfig(ctx, volumeDoc, libraryDoc)

	suite.T().Cleanup(func() {
		// A context of its own: cleanups run after TearDownTest, which has already canceled
		// suite.ctx by then.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		nodeCtx := client.WithNode(cleanupCtx, node)

		suite.RemoveMachineConfigDocumentsByName(nodeCtx, hypervisorcfg.ContentLibraryConfigKind, name)
		suite.RemoveMachineConfigDocumentsByName(nodeCtx, blockcfg.UserVolumeConfigKind, name)

		rtestutils.AssertNoResource[*hypervisor.ContentLibraryStatus](nodeCtx, suite.T(), suite.Client.COSI, name)
	})

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, []string{volumeID},
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
		},
	)

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, []string{name},
		func(cls *hypervisor.ContentLibraryStatus, asrt *assert.Assertions) {
			asrt.True(cls.TypedSpec().Ready, "error: %q", cls.TypedSpec().Error)
		},
	)

	return ctx, name, filepath.Join(constants.UserVolumeMountPoint, name)
}

// TestLibraryBecomesReady covers a library declared on a user volume being reported as ready, and
// pointed at the volume it was declared on.
func (suite *ContentLibrarySuite) TestLibraryBecomesReady() {
	ctx, name, path := suite.provisionLibrary()

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, []string{name},
		func(cls *hypervisor.ContentLibraryStatus, asrt *assert.Assertions) {
			asrt.Equal(constants.UserVolumePrefix+name, cls.TypedSpec().VolumeID)
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

	resp, err := suite.Client.ContentLibraryUpload(ctx, name, "image.raw", false, bytes.NewReader(contents))
	suite.Require().NoError(err)
	suite.Assert().Equal("image.raw", resp.GetName())
	suite.Assert().Equal(uint64(len(contents)), resp.GetSize())

	files := suite.list(ctx, name)
	suite.Require().Len(files, 1)
	suite.Assert().Equal("image.raw", files[0].GetName())
	suite.Assert().Equal(uint64(len(contents)), files[0].GetSize())

	// The file really is on the volume, under the path the status reports.
	suite.Assert().Equal(string(contents), suite.ReadFile(ctx, filepath.Join(path, "image.raw")))

	suite.Require().NoError(suite.delete(ctx, name, "image.raw"))

	suite.Assert().Empty(suite.list(ctx, name))

	// Deleting what is no longer there is an error, not a silent success.
	suite.Assert().Equal(codes.NotFound, status.Code(suite.delete(ctx, name, "image.raw")))
}

// TestOverwrite covers an upload landing on a name which is already taken: it has to say so, and
// only replace the file when it was asked to.
func (suite *ContentLibrarySuite) TestOverwrite() {
	ctx, name, path := suite.provisionLibrary()

	contents := bytes.Repeat([]byte("talos"), 1024)

	_, err := suite.Client.ContentLibraryUpload(ctx, name, "image.raw", false, bytes.NewReader(contents))
	suite.Require().NoError(err)

	_, err = suite.Client.ContentLibraryUpload(ctx, name, "image.raw", false, bytes.NewReader(contents))
	suite.Assert().Equal(codes.AlreadyExists, status.Code(err))

	suite.Assert().Equal(string(contents), suite.ReadFile(ctx, filepath.Join(path, "image.raw")))

	replacement := bytes.Repeat([]byte("sidero"), 512)

	_, err = suite.Client.ContentLibraryUpload(ctx, name, "image.raw", true, bytes.NewReader(replacement))
	suite.Require().NoError(err)

	suite.Assert().Equal(string(replacement), suite.ReadFile(ctx, filepath.Join(path, "image.raw")))
}

// TestRejectsNames covers the names which must never reach the filesystem: a library is flat, and
// nothing in it is addressable outside it.
func (suite *ContentLibrarySuite) TestRejectsNames() {
	ctx, name, _ := suite.provisionLibrary()

	contents := bytes.Repeat([]byte("talos"), 1024)

	for _, badName := range []string{"../escape.raw", "/etc/passwd", "sub/dir.raw", ".hidden", ""} {
		_, err := suite.Client.ContentLibraryUpload(ctx, name, badName, true, bytes.NewReader(contents))
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

	cli, err := suite.Client.ContentLibraryClient.List(ctx, &machineapi.ContentLibraryServiceListRequest{
		LibraryId: "not-a-library",
	})
	suite.Require().NoError(err)

	// A server streaming call reports the failure on the first Recv, not when it is started.
	_, err = cli.Recv()
	suite.Assert().Equal(codes.NotFound, status.Code(err))

	_, err = suite.Client.ContentLibraryUpload(ctx, "not-a-library", "image.raw", false, bytes.NewReader(nil))
	suite.Assert().Equal(codes.NotFound, status.Code(err))

	suite.Assert().Equal(codes.NotFound, status.Code(suite.delete(ctx, "not-a-library", "image.raw")))
}

// delete removes a file from a content library on a single node.
func (suite *ContentLibrarySuite) delete(nodeCtx context.Context, libraryID, name string) error {
	_, err := suite.Client.ContentLibraryClient.Delete(nodeCtx, &machineapi.ContentLibraryServiceDeleteRequest{
		LibraryId: libraryID,
		Name:      name,
	})

	return err
}

// list returns the contents of a content library on a single node.
func (suite *ContentLibrarySuite) list(nodeCtx context.Context, libraryID string) []*machineapi.ContentLibraryServiceListResponse {
	cli, err := suite.Client.ContentLibraryClient.List(nodeCtx, &machineapi.ContentLibraryServiceListRequest{
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
