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

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/security"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	securityres "github.com/siderolabs/talos/pkg/machinery/resources/security"
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

	const (
		image         = "registry.k8s.io/kube-apiserver:v1.27.1"
		digestedImage = "registry.k8s.io/kube-apiserver@sha256:a6daed8429c54f0008910fc4ecc17aefa1dfcd7cc2ff0089570854d4f95213ed"
	)

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
	// depending on whether the image verification is enabled or not, the pulled image ref can be either the original one (without digest) or the digested one, so we should accept both
	suite.Assert().Contains([]string{digestedImage, image}, pulledImage, "pulled image name should match requested image")
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

// TestVerify tests ImageService.Verify().
//
//nolint:gocyclo
func (suite *ImagesSuite) TestVerify() {
	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	suite.T().Logf("using node %s", node)

	// query the current image verification config to restore it after the test
	cfg, err := suite.ReadConfigFromNode(ctx)
	suite.Require().NoError(err)

	originalConfig := xslices.Filter(cfg.Documents(), func(doc config.Document) bool {
		return doc.Kind() == security.ImageVerificationConfigKind
	})

	if len(originalConfig) == 0 {
		// if the image verification hasn't been enabled, skip the test
		suite.T().Skip("skipping image verification test since no image verification config is present in the cluster")
	}

	// remove any existing image verification config to start with a clean slate
	suite.RemoveMachineConfigDocuments(ctx, security.ImageVerificationConfigKind)

	// our custom image verification config for this test
	imageVerificationConfig := security.NewImageVerificationConfigV1Alpha1()
	imageVerificationConfig.ConfigRules = security.ImageVerificationRules{
		{
			RuleImagePattern: "do.not.verify/*",
			RuleSkip:         new(true),
		},
		{
			RuleImagePattern: "registry.k8s.io/*",
			RuleKeylessVerifier: &security.ImageKeylessVerifierV1Alpha1{
				KeylessIssuer:  "https://accounts.google.com",
				KeylessSubject: "krel-trust@k8s-releng-prod.iam.gserviceaccount.com",
			},
		},
		{
			RuleImagePattern: "localhost:4444/*",
			RuleDeny:         new(true),
		},
	}

	suite.PatchMachineConfig(ctx, imageVerificationConfig)

	// wait for the configuration to be applied
	rtestutils.AssertResources(
		ctx, suite.T(), suite.Client.COSI,
		[]resource.ID{"0000", "0001", "0002"},
		func(rule *securityres.ImageVerificationRule, asrt *assert.Assertions) {
			switch rule.Metadata().ID() {
			case "0000":
				asrt.Equal("do.not.verify/*", rule.TypedSpec().ImagePattern)
			case "0001":
				asrt.Equal("registry.k8s.io/*", rule.TypedSpec().ImagePattern)
			case "0002":
				asrt.Equal("localhost:4444/*", rule.TypedSpec().ImagePattern)
			}
		},
	)

	const etcdImage = constants.EtcdImage + ":" + constants.DefaultEtcdVersion

	// run the tests, first with an etcd image, which should be in the image cache anyways
	resp, err := suite.Client.ImageClient.Verify(ctx, &machine.ImageServiceVerifyRequest{
		ImageRef: etcdImage, // this image is under registry.k8s.io
	})
	suite.Require().NoError(err)
	suite.Assert().True(resp.GetVerified(), "expected image to be verified according to our config")
	suite.Assert().Equal("verified via legacy signature (bundle verified true)", resp.GetMessage())
	suite.Assert().Contains(resp.GetDigestedImageRef(), constants.EtcdImage)
	suite.Assert().Contains(resp.GetDigestedImageRef(), "@sha256:")

	// now test with an image that should be skipped
	resp, err = suite.Client.ImageClient.Verify(ctx, &machine.ImageServiceVerifyRequest{
		ImageRef: "do.not.verify/myimage:latest",
	})
	suite.Require().NoError(err)
	suite.Assert().False(resp.GetVerified(), "expected image verification to be skipped according to our config")
	suite.Assert().Equal("verification skipped by matched rule (0000)", resp.GetMessage())

	// finally test with an image that should be denied
	_, err = suite.Client.ImageClient.Verify(ctx, &machine.ImageServiceVerifyRequest{
		ImageRef: "localhost:4444/myimage:latest",
	})
	suite.Require().Error(err)
	suite.Assert().Equal(codes.PermissionDenied, status.Code(err), "expected image verification to be denied according to our config")
	suite.Assert().Equal("verification denied by matched rule (0002)", status.Convert(err).Message())

	// now test via the image pull flow
	rcv, err := suite.Client.ImageClient.Pull(ctx, &machine.ImageServicePullRequest{
		Containerd: &common.ContainerdInstance{
			Driver:    common.ContainerDriver_CRI,
			Namespace: common.ContainerdNamespace_NS_SYSTEM,
		},
		ImageRef: etcdImage,
	})
	suite.Require().NoError(err)

	for {
		_, err = rcv.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			suite.Require().NoError(err)
		}
	}

	listResp, err := suite.Client.ImageClient.List(ctx, &machine.ImageServiceListRequest{
		Containerd: &common.ContainerdInstance{
			Driver:    common.ContainerDriver_CRI,
			Namespace: common.ContainerdNamespace_NS_SYSTEM,
		},
	})
	suite.Require().NoError(err)

	var found bool

	for {
		msg, err := listResp.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			suite.Require().NoError(err)
		}

		if msg.GetName() == etcdImage {
			found = true

			suite.Assert().Contains(msg.GetLabels(), constants.ImageLabelVerified)
		}
	}

	suite.Assert().True(found, "expected to find the pulled image in the list response")

	// remove our config
	suite.RemoveMachineConfigDocuments(ctx, security.ImageVerificationConfigKind)

	if len(originalConfig) > 0 {
		// put back original image verification config
		suite.PatchMachineConfig(ctx, xslices.Map(originalConfig, func(doc config.Document) any { return doc })...)
	}
}

func init() {
	allSuites = append(allSuites, new(ImagesSuite))
}
