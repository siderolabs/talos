// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"regexp"
	"strings"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// ImageSuite verifies the image command.
type ImageSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *ImageSuite) SuiteName() string {
	return "cli.ImageSuite"
}

// TestDefault verifies default Talos list of images.
func (suite *ImageSuite) TestDefault() {
	suite.RunCLI([]string{"image", "k8s-bundle"},
		base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta("registry.k8s.io/kube-apiserver"))),
	)
}

var versionRe = regexp.MustCompile(`^[v]?(\d+\.\d+\.\d+(?:-(?:alpha|beta|rc)\.\d+)?)`)

func normalizeTag(tag string) string {
	m := versionRe.FindStringSubmatch(tag)
	if len(m) == 0 {
		return tag
	}

	return m[1]
}

// TestSourceBundle verifies talos-bundle Talos list of images.
func (suite *ImageSuite) TestSourceBundle() {
	out := `ghcr.io/siderolabs/installer:v1.11.2
ghcr.io/siderolabs/installer-base:v1.11.2
ghcr.io/siderolabs/imager:v1.11.2
ghcr.io/siderolabs/talos:v1.11.2
ghcr.io/siderolabs/talosctl-all:v1.11.2
ghcr.io/siderolabs/overlays:v1.11.2
ghcr.io/siderolabs/extensions:v1.11.2
ghcr.io/siderolabs/amazon-ena:2.15.0-v1.11.2@sha256:e21baee86adfb637b113751a678e9a57e970d3d597a2e488fc13440db1437732
ghcr.io/siderolabs/amd-ucode:20250917@sha256:9b1a5fd03b9c5bc685d3a363ddc471ff6092a304f5fdfe67aa1f4626e34d72eb
ghcr.io/siderolabs/amdgpu:20250917-v1.11.2@sha256:09899678ea287b01ff7104f4a0a7c731fd688d9c0047c58a535dfd1d68875364
ghcr.io/siderolabs/binfmt-misc:v1.11.2@sha256:2da530ba50ca5053636405620d91764d1debacbfd7ecd9c5fe91be0d3d4f90ca
ghcr.io/siderolabs/bnx2-bnx2x:20250917@sha256:ac6aaaa0d3312e72279a5cde7de0d71fb61774aa2f97a4e56dd914a9f1dde4d1
ghcr.io/siderolabs/btrfs:v1.11.2@sha256:f1d9351b6627807cd8923204662c0f876b3db70a5b57e5363aadf399bea4a8ed
ghcr.io/siderolabs/chelsio-drivers:v1.11.2@sha256:d1800384e3a55b8cb1d82cab086c6dbe5af28e1d56f18fd763fa0aabbde538f8
ghcr.io/siderolabs/chelsio-firmware:20250917@sha256:f54cea7351565001abf9a26794dc1c96ba8182150b2c18777e9eed04dc6d3d8d
ghcr.io/siderolabs/cloudflared:2024.12.1@sha256:ab2792aca1108b22a2f59a9852edf6f77fbb337ecf5bedcedfa0ae5a12bd0e41
ghcr.io/siderolabs/crun:1.24@sha256:157f1c563931275443dd46fbc44c854c669f5cf4bbc285356636a57c6d33caed
ghcr.io/siderolabs/ctr:v2.1.4@sha256:9291fccaf2dcb05a053dd20472437ce69ef3784c95f633a5278758d24eb407a3
ghcr.io/siderolabs/drbd:9.2.14-v1.11.2@sha256:4e8a32cbff6d96c6a61e523636f7c11f5a72984097e3e4a68f9f5354ac070b99
ghcr.io/siderolabs/dvb-cx23885:v1.11.2@sha256:631785a877a36adf2c7d26db5a2b29f64bcf0ea2356b07aa50a5be8493acc33e
ghcr.io/siderolabs/dvb-m88ds3103:v1.11.2@sha256:6cb61a2d0b3257168906b6eeda023b60ae96e16d5a31d549486f7f6ca39c5cab
ghcr.io/siderolabs/ecr-credential-provider:v1.34.0@sha256:ca6754837ecb19aba419ba2e9fe558a3aff6b2d897a93f897ac44aa01053fab6
ghcr.io/siderolabs/fuse3:3.17.4@sha256:a4770b619691b64e516fbe65e57e7ca54f2132f230d41c1e3bbce6ae6c14547b
ghcr.io/siderolabs/gasket-driver:5815ee3-v1.11.2@sha256:430397008e0f214391a44a2748c5009d373dce5a5d689b99d817a6d4372b922b
ghcr.io/siderolabs/glibc:2.41@sha256:7b70eccac7f5ee44e2d7558bc21da67988ca36413be45de5dba2679a12317ce8
ghcr.io/siderolabs/gvisor-debug:v1.0.0@sha256:cc443f896ce6f822fc74234cd285e9ecf39f809654040576c98166819b5ef0a6
ghcr.io/siderolabs/gvisor:20250707.0@sha256:3f43bb94c1b09caa4a82559478b88c30b0b735303e37129e6e7bdc2cefe3ebc7
ghcr.io/siderolabs/hailort:4.21.0@sha256:bcdd3088158b7c71cd852ccea0945d5074040669155f43e33c159c567a23cdb8
ghcr.io/siderolabs/hello-world-service:v1.0.0@sha256:7e5de44b094bbe24d5c22cac45828975dc654c4ff4c1169482fe3671b2e490c9
ghcr.io/siderolabs/i915:20250917-v1.11.2@sha256:46251cb415b5036d7ebb460b0babe98f855930ef2cdc4709acdfa6e379278dd6
ghcr.io/siderolabs/intel-ice-firmware:20250917@sha256:c25225c371e81485c64f339864ede410b560f07eb0fc2702a73315e977a6323d
ghcr.io/siderolabs/intel-ucode:20250812@sha256:84ada70546d2f8d28d209ccfc7895c2cd0fa5f623815dcd45e6b118a06ad0959
ghcr.io/siderolabs/iscsi-tools:v0.2.0@sha256:ef6e8038ddaca1faad2d0e5b87ef4696fa6359f287682c9492462abfa1e26906
ghcr.io/siderolabs/kata-containers:3.20.0@sha256:4d8a59c058eb9678385b868e38a21dcd531dad64e708c282c7ef5d06dc27c98b
ghcr.io/siderolabs/lldpd:1.0.20@sha256:01fc489e506117b157e45f5ec77183a8984bb538c4223855ce1d8102e32c5d3e
ghcr.io/siderolabs/mdadm:v4.3@sha256:70576ecb239156c822b419d3169a478baebafb71432a4961cb71305bcddb7a36
ghcr.io/siderolabs/mei:v1.11.2@sha256:ec5978f641f6db18248f452343e93912852ef45c256dc1b836af7f40783786b6
ghcr.io/siderolabs/metal-agent:v0.1.3@sha256:0bb8dfd62e058af4dd85deed4864e7628e2ac5d7705d711570abf1be19a2f507
ghcr.io/siderolabs/nebula:1.9.6@sha256:e234e575cfffd6cc67ce1b67ba1b72d2827ca26793d82459ed389952e842b4c4
ghcr.io/siderolabs/newt:1.4.4@sha256:9226d5c591cafa714743f3f5519b19997202078704fcb7e1a1264c1613c59bff
ghcr.io/siderolabs/nfsd:v1.11.2@sha256:faa93d11292e96ee9c64209178fb641b49a89b600b605b5c2b0283a850a28a3f
ghcr.io/siderolabs/nfsrahead:2.8.3@sha256:9f140fd2735dfef1793acc7afaec57fdd547b497b559c12fb088ed3ee449c439
ghcr.io/siderolabs/nonfree-kmod-nvidia-lts:535.247.01-v1.11.2@sha256:af6b123c5269f47aa828c9e472d482fca69efb714b9e256a13b2384ea2b13549
ghcr.io/siderolabs/nonfree-kmod-nvidia-production:570.172.08-v1.11.2@sha256:162ade2a1a826403b5d7d1121c744db13825fc1238f380d3b98c74cf6d233916
ghcr.io/siderolabs/nut-client:2.8.4@sha256:6555859aba70aa4f20bd98171123da6f1ef76060d43d283178c05fe99d536715
ghcr.io/siderolabs/nvidia-container-toolkit-lts:535.247.01-v1.17.8@sha256:e425765c607b34029b61b16add2c455492059e360f73057f54e6a1a3d60279ad
ghcr.io/siderolabs/nvidia-container-toolkit-production:570.172.08-v1.17.8@sha256:eff6eb15bb5ceabe84904a6d62f872d5547bb98fdf78f119297f739fd775973d
ghcr.io/siderolabs/nvidia-fabricmanager-lts:535.247.01@sha256:aaf3d09e87ef9a0bfa2522775ba7518b30ccac4b909d8d2d739a24410daba769
ghcr.io/siderolabs/nvidia-fabricmanager-production:570.172.08@sha256:43d2649dc55582e1922408663425f7bd14f277839831c3601667d588ffcdfdf3
ghcr.io/siderolabs/nvidia-open-gpu-kernel-modules-lts:535.247.01-v1.11.2@sha256:69fafd7fa4b708226672d2334670bfbb0004fced548d6a0b2fab9807517c7971
ghcr.io/siderolabs/nvidia-open-gpu-kernel-modules-production:570.172.08-v1.11.2@sha256:3b68c873acf144048fd28e43e3054131d777b34101cd3cf9ce1b0d90c77da114
ghcr.io/siderolabs/nvme-cli:v2.14@sha256:6d5052488d524ec1791a6d6c3150e5cff3c88a774f8d828a4553d42deece56f7
ghcr.io/siderolabs/panfrost:20250917-v1.11.2@sha256:8339980ff926f3de05df90008643458234769bb6a68351e255030fce8ec08739
ghcr.io/siderolabs/qemu-guest-agent:10.0.2@sha256:9720300de00544eca155bc19369dfd7789d39a0e23d72837a7188f199e13dc6c
ghcr.io/siderolabs/qlogic-firmware:20250917@sha256:9a62f7562ebf07392a1184e6f9354eaea786072bdd8b76ef85a08528ad3a7c53
ghcr.io/siderolabs/realtek-firmware:20250917@sha256:6710269640d8684dfcf782b638b6c4e34896df136185218d8fdb0312a2bf00b0
ghcr.io/siderolabs/revpi-firmware:v1.0.0@sha256:9649e42c71862b9c4b9f0887d0929297e78530854ae507436344cecd6176debe
ghcr.io/siderolabs/sbc-allwinner:v0.1.2@sha256:c662949ad20bb37a20bc96c21187fefdcce8c6543769d993656f40d82fd88e2b
ghcr.io/siderolabs/sbc-jetson:v0.1.2@sha256:896d043ad0780cccbcd6f7984f3db5685d63f2659eb9dc7023542b72639599d6
ghcr.io/siderolabs/sbc-raspberrypi:v0.1.5@sha256:70a1b174a5bddd57da33e377af324c1f93d7ed87b9bc8f6cdefe5ec179abb4c9
ghcr.io/siderolabs/sbc-rockchip:v0.1.5@sha256:406c0ed9708772d72f0a0afc2998e14e673bf889135d2b47d92771c0d3566ff5
ghcr.io/siderolabs/spin:v0.21.0@sha256:b405524de2a2826b22354962d48688237ff4f5f7cf79f80c27d08f057c51b505
ghcr.io/siderolabs/stargz-snapshotter:v0.17.0@sha256:10280ee408e2e73a6d6f96f64efac3a3ef93bf3874e353a515cf2aa34ce7f2a5
ghcr.io/siderolabs/tailscale:1.88.1@sha256:36e484cbd93340b10e4c0d9ffef5b626e48ba435295936167d2711dd21ed4c3b
ghcr.io/siderolabs/tenstorrent:1.34@sha256:ea51e09b201548a3856783828b47be0d76e912f7ee7b02ced6b6dfde9ba1f1c2
ghcr.io/siderolabs/thunderbolt:v1.11.2@sha256:bf6482be7df448317443bfcd8b66bf68c6231e86b85d69c1e7c92ddb8aaf70dc
ghcr.io/siderolabs/uinput:v1.11.2@sha256:435b1187a17e1c1153b8374cfcd47ca24d4faab9d6efa0949894d85ade8ff3b7
ghcr.io/siderolabs/usb-modem-drivers:v1.11.2@sha256:c9ee048484cb0d2609f746dc7d90b213d26ad01e07342771bdb10841ec09b886
ghcr.io/siderolabs/util-linux-tools:2.41.1@sha256:bcd353bb7635d6dc9e6e913abdbff5b73d00829e3fd14e549b50533df8aa4faf
ghcr.io/siderolabs/v4l-uvc-drivers:v1.11.2@sha256:ae29326c8c3d34d1f1bc5aedf9f79f7b16c833722520c9763d7dd76212d041bd
ghcr.io/siderolabs/vc4:v1.11.2@sha256:f5c81fbd7cab1b1eecf904a93e501c7334a47db0dcc12232d2b76295d29c537d
ghcr.io/siderolabs/vmtoolsd-guest-agent:v1.4.0@sha256:06cc9885433eb9ca8148e76b38f9b7a1b85e375864b2f0bab5da75e9389ad027
ghcr.io/siderolabs/wasmedge:v0.6.0@sha256:aa255c9d3d5c61010943fd4fc9e3818739605243f4288ef2166915e57267b6d9
ghcr.io/siderolabs/xdma-driver:aefa9a1-v1.11.2@sha256:b65cb2033d46a7c88d317cb29a87741f8c72d768869bbfae832cb76121e4ab14
ghcr.io/siderolabs/xen-guest-agent:0.4.0-g5c274e6@sha256:91e08d9ae45e325bf20da77f251e265d0e72cb38751a6dcee930bf21c9adacc1
ghcr.io/siderolabs/youki:0.5.5@sha256:562ceabb69570203024dbb9b8673ba485af1ffdd082210656573e22557235372
ghcr.io/siderolabs/zerotier:1.16.0@sha256:9444baa3acdc665dba56ed16c8a983c81c3f37fc73877be8fd882f9cf8c9fa5a
ghcr.io/siderolabs/zfs:2.3.3-v1.11.2@sha256:73782571f334b18995ddf324d24b86ea9a11aa37661a468b4e077da63e0d9714`

	suite.RunCLI([]string{"image", "talos-bundle", "v1.11.2"},
		base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta(out))),
	)

	tag, err := semver.ParseTolerant(normalizeTag(version.Tag))
	assert.NoError(suite.T(), err)

	suite.T().Log(normalizeTag(version.Tag))
	suite.T().Log(version.Tag)

	if strings.TrimLeft(version.Tag, "v") == normalizeTag(version.Tag) {
		suite.T().Skip("skipping the test for the exact version tag")
	}

	suite.RunCLI([]string{"image", "talos-bundle", "v" + tag.String()},
		base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta("ghcr.io/siderolabs/talos:v"+tag.String()))),
	)

	suite.RunCLI([]string{"image", "talos-bundle", tag.String()},
		base.StdoutEmpty(),
		base.ShouldFail(),
	)

	tag.Patch = 0
	assert.NoError(suite.T(), tag.IncrementMinor())
	suite.RunCLI([]string{"image", "talos-bundle", "v" + tag.FinalizeVersion()},
		base.StdoutEmpty(),
		base.ShouldFail(),
	)
}

// TestList verifies listing images in the CRI.
func (suite *ImageSuite) TestList() {
	suite.RunCLI([]string{"image", "ls", "--nodes", suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)},
		base.StdoutShouldMatch(regexp.MustCompile(`IMAGE\s+DIGEST\s+SIZE`)),
		base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta("registry.k8s.io/kube-apiserver"))),
	)

	suite.RunCLI([]string{"image", "ls", "--namespace", "system", "--nodes", suite.RandomDiscoveredNodeInternalIP()},
		base.StdoutShouldMatch(regexp.MustCompile(`IMAGE\s+DIGEST\s+SIZE`)),
		base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta("ghcr.io/siderolabs/kubelet:"))),
	)
}

// TestPull verifies pulling images to the CRI.
func (suite *ImageSuite) TestPull() {
	const image = "registry.k8s.io/kube-apiserver:v1.27.0" // sync this to e2e.sh `build_image_cache`

	node := suite.RandomDiscoveredNodeInternalIP()

	if stdout, _ := suite.RunCLI([]string{"get", "imagecacheconfig", "--nodes", node, "--output", "jsonpath='{.spec.status}'"}); strings.Contains(stdout, "ready") {
		suite.T().Logf("skipping as the image cache is present")

		return
	}

	suite.RunCLI([]string{"image", "pull", "--nodes", node, image},
		base.StdoutEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(regexp.QuoteMeta("pulled image registry.k8s.io/kube-apiserver:v1.27.0"))),
	)

	// verify that pulled image appeared, also image aliases should appear
	suite.RunCLI([]string{"image", "ls", "--nodes", node},
		base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta(image))),
		base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta("sha256:89b8d9dbef2b905b7d028ca8b7f79d35ebd9baa66b0a3ee2ddd4f3e0e2804b45"))),
		base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta("registry.k8s.io/kube-apiserver@sha256:89b8d9dbef2b905b7d028ca8b7f79d35ebd9baa66b0a3ee2ddd4f3e0e2804b45"))),
	)

	// remove the image
	suite.RunCLI([]string{"image", "remove", "--nodes", node, image},
		base.StdoutEmpty(),
	)
}

// TestCacheCreateOCI verifies creating a cache tarball.
func (suite *ImageSuite) TestCacheCreateOCI() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	stdOut, _ := suite.RunCLI([]string{"image", "k8s-bundle"})

	imagesList := strings.Split(strings.Trim(stdOut, "\n"), "\n")

	imagesArgs := xslices.Map(imagesList[:2], func(image string) string {
		return "--images=" + image
	})

	cacheDir := suite.T().TempDir()

	args := []string{"image", "cache-create", "--image-cache-path", cacheDir, "--force", "--layout", "oci"}

	args = append(args, imagesArgs...)

	suite.RunCLI(args, base.StdoutEmpty(), base.StderrNotEmpty())

	assert.FileExistsf(suite.T(), cacheDir+"/index.json", "index.json should exist in the image cache directory")
}

// TestCacheCreateFlat verifies creating a cache directory.
func (suite *ImageSuite) TestCacheCreateFlat() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	stdOut, _ := suite.RunCLI([]string{"image", "k8s-bundle"})

	imagesList := strings.Split(strings.Trim(stdOut, "\n"), "\n")

	imagesArgs := xslices.Map(imagesList[:2], func(image string) string {
		return "--images=" + image
	})

	cacheDir := suite.T().TempDir()

	args := []string{"image", "cache-create", "--image-cache-path", cacheDir, "--force", "--layout", "flat"}

	args = append(args, imagesArgs...)

	suite.RunCLI(args, base.StdoutEmpty(), base.StderrNotEmpty())

	assert.DirExistsf(suite.T(), cacheDir+"/blob", "blob directory should exist in the image cache directory")
	assert.DirExistsf(suite.T(), cacheDir+"/manifests", "manifests directory should exist in the image cache directory")
}

func init() {
	allSuites = append(allSuites, new(ImageSuite))
}
