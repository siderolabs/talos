// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	_ "embed"
	"errors"
	"io"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

// ImagesSuite ...
type ImagesSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *ImagesSuite) SuiteName() string {
	return "api.ImagesSuite"
}

// SetupTest ...
func (suite *ImagesSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)
}

// TearDownTest ...
func (suite *ImagesSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestList tests ImageService.List().
func (suite *ImagesSuite) TestList() {
	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	suite.T().Logf("using node %s", node)

	rcv, err := suite.Client.ImageClient.List(ctx, &machine.ImageServiceListRequest{
		Containerd: &common.ContainerdInstance{
			Driver:    common.ContainerDriver_CRI,
			Namespace: common.ContainerdNamespace_NS_CRI,
		},
	})
	suite.Require().NoError(err)

	var imageNames []string

	for {
		msg, err := rcv.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			suite.Require().NoError(err)
		}

		imageNames = append(imageNames, msg.GetName())
	}

	suite.Require().NotEmpty(imageNames, "expected to receive at least one image from List()")

	for _, name := range imageNames {
		if strings.Contains(name, "registry.k8s.io/pause") {
			return
		}
	}

	suite.Fail("expected to find pause image in the list")
}

// TestPull tests ImageService.Pull().
func (suite *ImagesSuite) TestPull() {
	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	suite.T().Logf("using node %s", node)

	image := "registry.k8s.io/kube-apiserver:v1.27.1"

	rcv, err := suite.Client.ImageClient.Pull(ctx, &machine.ImageServicePullRequest{
		Containerd: &common.ContainerdInstance{
			Driver:    common.ContainerDriver_CRI,
			Namespace: common.ContainerdNamespace_NS_CRI,
		},
		ImageRef: image,
	})
	suite.Require().NoError(err)

	var pulledImage string

	for {
		msg, err := rcv.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			suite.Require().NoError(err)
		}

		// ignore progress messages, but the last message should contain the image name
		pulledImage = msg.GetName()
	}

	suite.Require().NotEmpty(pulledImage, "expected pulled image name in the response")
	suite.Assert().Equal(image, pulledImage, "pulled image name should match requested image")
}

//go:embed testdata/pause.tar
var pauseImageTar []byte

// TestImportRemove tests ImageService.Import() and ImageService.Remove().
func (suite *ImagesSuite) TestImportRemove() {
	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	suite.T().Logf("using node %s", node)

	// we can only import the image if it matches Talos architecture
	versionResp, err := suite.Client.Version(ctx)
	suite.Require().NoError(err)
	suite.Require().Len(versionResp.GetMessages(), 1)

	arch := versionResp.GetMessages()[0].GetVersion().GetArch()
	if arch != "amd64" {
		suite.T().Skipf("skipping import test on unsupported architecture %q", arch)
	}

	rcv, err := suite.Client.ImageClient.Import(ctx)
	suite.Require().NoError(err)

	suite.Require().NoError(rcv.Send(&machine.ImageServiceImportRequest{
		Request: &machine.ImageServiceImportRequest_Containerd{
			Containerd: &common.ContainerdInstance{
				Driver:    common.ContainerDriver_CRI,
				Namespace: common.ContainerdNamespace_NS_CRI,
			},
		},
	}))

	const chunkSize = 4 * 1024

	for offset := 0; offset < len(pauseImageTar); offset += chunkSize {
		end := min(offset+chunkSize, len(pauseImageTar))

		suite.Require().NoError(rcv.Send(&machine.ImageServiceImportRequest{
			Request: &machine.ImageServiceImportRequest_ImageChunk{
				ImageChunk: &common.Data{
					Bytes: pauseImageTar[offset:end],
				},
			},
		}))
	}

	suite.Require().NoError(rcv.CloseSend())

	resp, err := rcv.CloseAndRecv()
	suite.Require().NoError(err)
	suite.Require().NotEmpty(resp.GetName(), "expected imported image name in the response")
	suite.Assert().Equal("registry.k8s.io/pause:i-was-a-digest", resp.GetName(), "imported image name should match expected")

	// now remove the imported image
	_, err = suite.Client.ImageClient.Remove(ctx, &machine.ImageServiceRemoveRequest{
		Containerd: &common.ContainerdInstance{
			Driver:    common.ContainerDriver_CRI,
			Namespace: common.ContainerdNamespace_NS_CRI,
		},
		ImageRef: resp.GetName(),
	})
	suite.Require().NoError(err)

	// try once again
	_, err = suite.Client.ImageClient.Remove(ctx, &machine.ImageServiceRemoveRequest{
		Containerd: &common.ContainerdInstance{
			Driver:    common.ContainerDriver_CRI,
			Namespace: common.ContainerdNamespace_NS_CRI,
		},
		ImageRef: resp.GetName(),
	})
	suite.Require().Error(err)
	suite.Assert().Equal(status.Code(err), codes.NotFound)
}

func init() {
	allSuites = append(allSuites, new(ImagesSuite))
}
