// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

// Package provision provides integration tests which rely on provisioning cluster per test.
package provision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-blockdevice/v2/encryption"
	"github.com/siderolabs/go-kubernetes/kubernetes/ssa"
	"github.com/siderolabs/go-kubernetes/kubernetes/upgrade"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/siderolabs/go-retry/retry"
	sideronet "github.com/siderolabs/net"
	"github.com/stretchr/testify/suite"
	"go.yaml.in/yaml/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/cluster/check"
	"github.com/siderolabs/talos/pkg/cluster/hydrophone"
	"github.com/siderolabs/talos/pkg/cluster/kubernetes"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/version"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/access"
	"github.com/siderolabs/talos/pkg/provision/providers/qemu"
)

var allSuites []suite.TestingSuite

// GetAllSuites returns all the suites for provision test.
//
// Depending on build tags, this might return different lists.
func GetAllSuites() []suite.TestingSuite {
	return allSuites
}

// Settings for provision tests.
type Settings struct {
	// CIDR to use for provisioned clusters
	CIDR string
	// Registry mirrors to push to Talos config, in format `host=endpoint`
	RegistryMirrors base.StringList
	// MTU for the network.
	MTU int
	// VM parameters
	CPUs   int64
	MemMB  int64
	DiskGB uint64
	// Node count for the tests
	ControlplaneNodes int
	WorkerNodes       int
	// Target installer image registry
	TargetInstallImageRegistry string
	// Current version of the cluster (built in the CI pass)
	CurrentVersion string
	// Custom CNI URL to use.
	CustomCNIURL string
	// CNI bundle for QEMU provisioner.
	CNIBundleURL string
}

// DefaultSettings filled in by test runner.
var DefaultSettings = Settings{
	CIDR:                       "172.21.0.0/24",
	MTU:                        1500,
	CPUs:                       4,
	MemMB:                      3 * 1024,
	DiskGB:                     12,
	ControlplaneNodes:          3,
	WorkerNodes:                1,
	TargetInstallImageRegistry: "ghcr.io",
	CNIBundleURL:               fmt.Sprintf("https://github.com/siderolabs/talos/releases/download/%s/talosctl-cni-bundle-%s.tar.gz", trimVersion(version.Tag), constants.ArchVariable),
}

func trimVersion(version string) string {
	// remove anything extra after semantic version core, `v0.3.2-1-abcd` -> `v0.3.2`
	return regexp.MustCompile(`(-\d+-g[0-9a-f]+)$`).ReplaceAllString(version, "")
}

var defaultNameservers = []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("1.1.1.1")}

// BaseSuite provides base features for provision tests.
type BaseSuite struct {
	suite.Suite
	base.TalosSuite

	provisioner provision.Provisioner

	configBundle *bundle.Bundle

	clusterAccess        *access.Adapter
	controlPlaneEndpoint string

	//nolint:containedctx
	ctx       context.Context
	ctxCancel context.CancelFunc

	stateDir string
	cniDir   string
}

// SetupSuite ...
func (suite *BaseSuite) SetupSuite() {
	// timeout for the whole test
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), time.Hour)

	var err error

	suite.provisioner, err = qemu.NewProvisioner(suite.ctx)
	suite.Require().NoError(err)
}

// TearDownSuite ...
func (suite *BaseSuite) TearDownSuite() {
	if suite.clusterAccess != nil {
		suite.Assert().NoError(suite.clusterAccess.Close())
	}

	if suite.Cluster != nil {
		suite.Assert().NoError(suite.provisioner.Destroy(suite.ctx, suite.Cluster))
	}

	suite.ctxCancel()

	if suite.stateDir != "" {
		suite.Assert().NoError(os.RemoveAll(suite.stateDir))
	}

	if suite.provisioner != nil {
		suite.Assert().NoError(suite.provisioner.Close())
	}
}

// waitForClusterHealth asserts cluster health after any change.
func (suite *BaseSuite) waitForClusterHealth() {
	runs := 1

	singleNodeCluster := len(suite.Cluster.Info().Nodes) == 1
	if singleNodeCluster {
		// run health check several times for single node clusters,
		// as self-hosted control plane is not stable after reboot
		runs = 3
	}

	for run := range runs {
		if run > 0 {
			time.Sleep(15 * time.Second)
		}

		checkCtx, checkCtxCancel := context.WithTimeout(suite.ctx, 15*time.Minute)
		defer checkCtxCancel()

		suite.Require().NoError(
			check.Wait(
				checkCtx,
				suite.clusterAccess,
				check.DefaultClusterChecks(),
				check.StderrReporter(),
			),
		)
	}
}

func (suite *BaseSuite) untaint(name string) {
	client, err := suite.clusterAccess.K8sClient(suite.ctx)
	suite.Require().NoError(err)

	n, err := client.CoreV1().Nodes().Get(suite.ctx, name, metav1.GetOptions{})
	suite.Require().NoError(err)

	oldData, err := json.Marshal(n)
	suite.Require().NoError(err)

	k := 0

	for _, taint := range n.Spec.Taints {
		if taint.Key != constants.LabelNodeRoleControlPlane {
			n.Spec.Taints[k] = taint
			k++
		}
	}

	n.Spec.Taints = n.Spec.Taints[:k]

	newData, err := json.Marshal(n)
	suite.Require().NoError(err)

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, corev1.Node{})
	suite.Require().NoError(err)

	_, err = client.CoreV1().Nodes().Patch(
		suite.ctx,
		n.Name,
		types.StrategicMergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	suite.Require().NoError(err)
}

func (suite *BaseSuite) assertSameVersionCluster(client *talosclient.Client, expectedVersion string) {
	nodes := xslices.Map(suite.Cluster.Info().Nodes, func(node provision.NodeInfo) string { return node.IPs[0].String() })
	ctx := talosclient.WithNodes(suite.ctx, nodes...)

	var v *machineapi.VersionResponse

	err := retry.Constant(
		time.Minute,
	).Retry(
		func() error {
			var e error

			v, e = client.Version(ctx)

			return retry.ExpectedError(e)
		},
	)

	suite.Require().NoError(err)

	suite.Require().Len(v.Messages, len(nodes))

	for _, version := range v.Messages {
		suite.Assert().Equal(expectedVersion, version.Version.Tag)
	}
}

func (suite *BaseSuite) assertCmdlineContains(client *talosclient.Client, node string, expectedCmdlineContains string) {
	ctx := talosclient.WithNode(suite.ctx, node)

	cmdline, err := safe.ReaderGetByID[*runtime.KernelCmdline](ctx, client.COSI, runtime.KernelCmdlineID)
	suite.Require().NoError(err)

	suite.Assert().NotEmpty(cmdline, "expected cmdline to be not empty")

	suite.Assert().Contains(cmdline.TypedSpec().Cmdline, expectedCmdlineContains, "expected cmdline to contain %q", expectedCmdlineContains)
}

func (suite *BaseSuite) readVersion(nodeCtx context.Context, client *talosclient.Client) (
	version string,
	err error,
) {
	var v *machineapi.VersionResponse

	v, err = client.Version(nodeCtx)
	if err != nil {
		return version, err
	}

	version = v.Messages[0].Version.Tag

	return version, err
}

type upgradeOptions struct {
	TargetInstallerImage string
	// Deprecated: staged upgrades are not supported by the new LifecycleService API.
	// Use the legacy MachineService.Upgrade path instead.
	UpgradeStage  bool
	TargetVersion string
}

//nolint:gocyclo,cyclop
func (suite *BaseSuite) upgradeNode(client *talosclient.Client, node provision.NodeInfo, options upgradeOptions) {
	suite.T().Logf("upgrading node %s", node.IPs[0])

	ctx, cancel := context.WithCancel(suite.ctx)
	defer cancel()

	nodeCtx := talosclient.WithNodes(ctx, node.IPs[0].String())

	// Staged upgrades are not supported by the new LifecycleService API,
	// so skip straight to the legacy path.
	if !options.UpgradeStage {
		if suite.tryUpgradeViaLifecycleService(nodeCtx, client, node, options) {
			// LifecycleService.Upgrade succeeded — trigger reboot and wait.
			suite.T().Logf("upgrade via LifecycleService succeeded, rebooting node %s", node.IPs[0])

			suite.Require().NoError(client.Reboot(nodeCtx))
			suite.waitForUpgrade(nodeCtx, client, node, options)

			return
		}

		suite.T().Logf("LifecycleService.Upgrade not available, falling back to legacy MachineService.Upgrade")
	}

	// Legacy path: MachineService.Upgrade (handles image pull, install, and reboot in one call).
	suite.upgradeNodeLegacy(nodeCtx, client, options)
	suite.waitForUpgrade(nodeCtx, client, node, options)
}

// tryUpgradeViaLifecycleService attempts to upgrade via the new streaming
// LifecycleService.Upgrade API. It pre-pulls the installer image, then calls
// the streaming RPC. Returns true on success, false if the server returned
// codes.Unimplemented (indicating the API is not available).
//
//nolint:gocyclo
func (suite *BaseSuite) tryUpgradeViaLifecycleService(
	nodeCtx context.Context,
	c *talosclient.Client,
	node provision.NodeInfo,
	options upgradeOptions,
) bool {
	// Step 1: Pre-pull the installer image into the system containerd namespace.
	suite.T().Logf("pre-pulling installer image %q on node %s", options.TargetInstallerImage, node.IPs[0])

	containerdInstance := &common.ContainerdInstance{
		Driver:    common.ContainerDriver_CONTAINERD,
		Namespace: common.ContainerdNamespace_NS_SYSTEM,
	}

	pullStream, err := c.ImageClient.Pull(nodeCtx, &machineapi.ImageServicePullRequest{
		Containerd: containerdInstance,
		ImageRef:   options.TargetInstallerImage,
	})
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			return false
		}

		suite.Require().NoError(err, "failed to start image pull stream")
	}

	// Drain the pull stream to completion.
	for {
		_, pullErr := pullStream.Recv()
		if pullErr != nil {
			if errors.Is(pullErr, io.EOF) {
				break
			}

			if status.Code(pullErr) == codes.Unimplemented {
				return false
			}

			suite.Require().NoError(pullErr, "error during image pull")
		}
	}

	// Step 2: Call LifecycleService.Upgrade (streaming).
	stream, err := c.LifecycleClient.Upgrade(nodeCtx, &machineapi.LifecycleServiceUpgradeRequest{
		Containerd: containerdInstance,
		Source: &machineapi.InstallArtifactsSource{
			ImageName: options.TargetInstallerImage,
		},
	})
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			return false
		}

		suite.Require().NoError(err, "failed to start LifecycleService.Upgrade stream")
	}

	var exitCode int32

	for {
		resp, recvErr := stream.Recv()
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				break
			}

			if status.Code(recvErr) == codes.Unimplemented {
				return false
			}

			suite.Require().NoError(recvErr, "error receiving LifecycleService.Upgrade response")
		}

		switch payload := resp.GetProgress().GetResponse().(type) {
		case *machineapi.LifecycleServiceInstallProgress_Message:
			suite.T().Logf("upgrade log: %s", payload.Message)
		case *machineapi.LifecycleServiceInstallProgress_ExitCode:
			exitCode = payload.ExitCode
		default:
			suite.Failf("unexpected response type from LifecycleService.Upgrade", "got %T", payload)
		}
	}

	suite.Require().Equal(int32(0), exitCode, "LifecycleService.Upgrade exited with non-zero code")

	return true
}

// upgradeNodeLegacy performs an upgrade using the legacy (deprecated) MachineService.Upgrade
// unary API, which handles image pull, install, and reboot in a single call.
//
//nolint:gocyclo
func (suite *BaseSuite) upgradeNodeLegacy(
	nodeCtx context.Context,
	c *talosclient.Client,
	options upgradeOptions,
) {
	var (
		resp *machineapi.UpgradeResponse
		err  error
	)

	err = retry.Constant(time.Minute, retry.WithUnits(10*time.Second)).Retry(
		func() error {
			resp, err = c.Upgrade( //nolint:staticcheck // using deprecated API for testing backward compatibility
				nodeCtx,
				options.TargetInstallerImage,
				options.UpgradeStage,
				false,
			)
			if err != nil {
				if strings.Contains(err.Error(), "leader changed") {
					return retry.ExpectedError(err)
				}

				if strings.Contains(err.Error(), "failed to acquire upgrade lock") {
					return retry.ExpectedError(err)
				}

				return err
			}

			return nil
		},
	)

	suite.Require().NoError(err)
	suite.Require().Equal("Upgrade request received", resp.Messages[0].Ack)

	actorID := resp.Messages[0].ActorId

	eventCh := make(chan talosclient.EventResult)

	// watch for events
	suite.Require().NoError(c.EventsWatchV2(nodeCtx, eventCh, talosclient.WithActorID(actorID), talosclient.WithTailEvents(-1)))

	waitTimer := time.NewTimer(5 * time.Minute)
	defer waitTimer.Stop()

waitLoop:
	for {
		select {
		case ev := <-eventCh:
			suite.Require().NoError(ev.Error)

			switch msg := ev.Event.Payload.(type) {
			case *machineapi.SequenceEvent:
				if msg.Error != nil {
					suite.FailNow("upgrade failed", "%s: %s", msg.Error.Message, msg.Error.Code)
				}
			case *machineapi.PhaseEvent:
				if msg.Action == machineapi.PhaseEvent_START && msg.Phase == "kexec" {
					// about to be rebooted
					break waitLoop
				}

				if msg.Action == machineapi.PhaseEvent_STOP {
					suite.T().Logf("upgrade phase %q finished", msg.Phase)
				}
			}
		case <-waitTimer.C:
			suite.FailNow("timeout waiting for upgrade to finish")
		case <-nodeCtx.Done():
			suite.FailNow("context canceled")
		}
	}
}

// waitForUpgrade waits for the node to come back up after a reboot with the
// expected target version, then verifies cluster health. This is shared by
// both the new LifecycleService and the legacy MachineService upgrade paths.
func (suite *BaseSuite) waitForUpgrade(
	nodeCtx context.Context,
	c *talosclient.Client,
	node provision.NodeInfo,
	options upgradeOptions,
) {
	// wait for the apid to be shut down
	time.Sleep(10 * time.Second)

	// wait for the version to be equal to target version
	var err error

	suite.Require().NoError(
		retry.Constant(10 * time.Minute).Retry(
			func() error {
				var version string

				version, err = suite.readVersion(nodeCtx, c)
				if err != nil {
					// API might be unresponsive during upgrade
					return retry.ExpectedError(err)
				}

				if version != options.TargetVersion {
					// upgrade not finished yet
					return retry.ExpectedErrorf(
						"node %q version doesn't match expected: expected %q, got %q",
						node.IPs[0].String(),
						options.TargetVersion,
						version,
					)
				}

				return nil
			},
		),
	)

	suite.waitForClusterHealth()
}

func (suite *BaseSuite) upgradeKubernetes(fromVersion, toVersion string, skipKubeletUpgrade bool) {
	if fromVersion == toVersion {
		suite.T().Logf("skipping Kubernetes upgrade, as versions are equal %q -> %q", fromVersion, toVersion)

		return
	}

	suite.T().Logf("upgrading Kubernetes: %q -> %q", fromVersion, toVersion)

	path, err := upgrade.NewPath(fromVersion, toVersion)
	suite.Require().NoError(err)

	options := kubernetes.UpgradeOptions{
		Path: path,

		ControlPlaneEndpoint: suite.controlPlaneEndpoint,

		UpgradeKubelet: !skipKubeletUpgrade,
		PrePullImages:  true,

		KubeletImage:           constants.KubeletImage,
		APIServerImage:         constants.KubernetesAPIServerImage,
		ControllerManagerImage: constants.KubernetesControllerManagerImage,
		SchedulerImage:         constants.KubernetesSchedulerImage,
		ProxyImage:             constants.KubeProxyImage,

		EncoderOpt: encoder.WithComments(encoder.CommentsAll),

		InventoryPolicy:  ssa.InventoryPolicyAdoptIfNoInventory,
		ReconcileTimeout: 3 * time.Minute,
	}

	suite.Require().NoError(kubernetes.Upgrade(suite.ctx, suite.clusterAccess, options))
}

type clusterOptions struct {
	ClusterName string

	ControlplaneNodes int
	WorkerNodes       int

	InjectExtraKernelArgs *procfs.Cmdline

	SourceKernelPath     string
	SourceInitramfsPath  string
	SourceDiskImagePath  string
	SourceISOPath        string
	SourceInstallerImage string
	SourceVersion        string
	SourceK8sVersion     string

	WithEncryption  bool
	WithBios        bool
	WithApplyConfig bool
}

// setupCluster provisions source clusters and waits for health.
//
//nolint:gocyclo
func (suite *BaseSuite) setupCluster(options clusterOptions) {
	defaultStateDir, err := clientconfig.GetTalosDirectory()
	suite.Require().NoError(err)

	suite.stateDir = filepath.Join(defaultStateDir, "clusters")
	suite.cniDir = filepath.Join(defaultStateDir, "cni")

	cidr, err := netip.ParsePrefix(DefaultSettings.CIDR)
	suite.Require().NoError(err)

	var gatewayIP netip.Addr

	gatewayIP, err = sideronet.NthIPInNetwork(cidr, 1)
	suite.Require().NoError(err)

	ips := make([]netip.Addr, options.ControlplaneNodes+options.WorkerNodes)

	for i := range ips {
		ips[i], err = sideronet.NthIPInNetwork(cidr, i+2)
		suite.Require().NoError(err)
	}

	suite.T().Logf("initializing provisioner with cluster name %q, state directory %q", options.ClusterName, suite.stateDir)

	request := provision.ClusterRequest{
		Name: options.ClusterName,

		Network: provision.NetworkRequest{
			Name:         options.ClusterName,
			CIDRs:        []netip.Prefix{cidr},
			GatewayAddrs: []netip.Addr{gatewayIP},
			MTU:          DefaultSettings.MTU,
			Nameservers:  defaultNameservers,
			CNI: provision.CNIConfig{
				BinPath:  []string{filepath.Join(suite.cniDir, "bin")},
				ConfDir:  filepath.Join(suite.cniDir, "conf.d"),
				CacheDir: filepath.Join(suite.cniDir, "cache"),

				BundleURL: DefaultSettings.CNIBundleURL,
			},
		},

		SelfExecutable: suite.TalosctlPath,
		StateDirectory: suite.stateDir,
	}

	switch {
	case options.SourceISOPath != "":
		request.ISOPath = options.SourceISOPath
	case options.SourceDiskImagePath != "":
		request.DiskImagePath = options.SourceDiskImagePath
	default:
		request.KernelPath = options.SourceKernelPath
		request.InitramfsPath = options.SourceInitramfsPath
	}

	suite.controlPlaneEndpoint = suite.provisioner.GetExternalKubernetesControlPlaneEndpoint(request.Network, constants.DefaultControlPlanePort)

	versionContract, err := config.ParseContractFromVersion(options.SourceVersion)
	suite.Require().NoError(err)

	genOptions, bundleOptions := suite.provisioner.GenOptions(request.Network, versionContract)

	for _, registryMirror := range DefaultSettings.RegistryMirrors {
		parts := strings.Split(registryMirror, "=")
		suite.Require().Len(parts, 2)

		genOptions = append(genOptions, generate.WithRegistryMirror(parts[0], parts[1]))
	}

	controlplaneEndpoints := make([]string, options.ControlplaneNodes)
	for i := range controlplaneEndpoints {
		controlplaneEndpoints[i] = ips[i].String()
	}

	if DefaultSettings.CustomCNIURL != "" {
		genOptions = append(
			genOptions, generate.WithClusterCNIConfig(
				&v1alpha1.CNIConfig{
					CNIName: constants.CustomCNI,
					CNIUrls: []string{DefaultSettings.CustomCNIURL},
				},
			),
		)
	}

	var extraPatches []configpatcher.Patch

	if options.WithEncryption {
		if versionContract.VolumeConfigEncryptionSupported() {
			// use modern encryption config
			stateCfg := block.NewVolumeConfigV1Alpha1()
			stateCfg.MetaName = constants.StatePartitionLabel
			stateCfg.EncryptionSpec.EncryptionProvider = blockres.EncryptionProviderLUKS2
			stateCfg.EncryptionSpec.EncryptionKeys = []block.EncryptionKey{
				{
					KeySlot:   0,
					KeyNodeID: &block.EncryptionKeyNodeID{},
				},
			}

			ephemeralCfg := block.NewVolumeConfigV1Alpha1()
			ephemeralCfg.MetaName = constants.EphemeralPartitionLabel
			ephemeralCfg.EncryptionSpec.EncryptionProvider = blockres.EncryptionProviderLUKS2
			ephemeralCfg.EncryptionSpec.EncryptionKeys = []block.EncryptionKey{
				{
					KeySlot:        0,
					KeyNodeID:      &block.EncryptionKeyNodeID{},
					KeyLockToSTATE: new(true),
				},
			}

			ctr, err := container.New(stateCfg, ephemeralCfg)
			suite.Require().NoError(err)

			extraPatches = append(extraPatches, configpatcher.NewStrategicMergePatch(ctr))
		} else {
			// use legacy encryption config
			diskEncryptionConfig := &v1alpha1.SystemDiskEncryptionConfig{
				StatePartition: &v1alpha1.EncryptionConfig{
					EncryptionProvider: encryption.LUKS2,
					EncryptionKeys: []*v1alpha1.EncryptionKey{
						{
							KeySlot:   0,
							KeyNodeID: &v1alpha1.EncryptionKeyNodeID{},
						},
					},
				},
				EphemeralPartition: &v1alpha1.EncryptionConfig{
					EncryptionProvider: encryption.LUKS2,
					EncryptionKeys: []*v1alpha1.EncryptionKey{
						{
							KeySlot:   0,
							KeyNodeID: &v1alpha1.EncryptionKeyNodeID{},
						},
					},
				},
			}

			patchRaw := map[string]any{
				"machine": map[string]any{
					"systemDiskEncryption": diskEncryptionConfig,
				},
			}

			patchData, err := yaml.Marshal(patchRaw)
			suite.Require().NoError(err)

			patch, err := configpatcher.LoadPatch(patchData)
			suite.Require().NoError(err)

			extraPatches = append(extraPatches, patch)
		}
	}

	suite.configBundle, err = bundle.NewBundle(
		append([]bundle.Option{
			bundle.WithInputOptions(
				&bundle.InputOptions{
					ClusterName: options.ClusterName,
					Endpoint:    suite.controlPlaneEndpoint,
					KubeVersion: options.SourceK8sVersion,
					GenOptions: append(
						genOptions,
						generate.WithEndpointList(controlplaneEndpoints),
						generate.WithInstallImage(options.SourceInstallerImage),
						generate.WithDNSDomain("cluster.local"),
						generate.WithVersionContract(versionContract),
					),
				},
			),
			bundle.WithPatch(extraPatches),
		},
			bundleOptions...,
		)...,
	)
	suite.Require().NoError(err)

	for i := range options.ControlplaneNodes {
		request.Nodes = append(
			request.Nodes,
			provision.NodeRequest{
				Name:     fmt.Sprintf("control-plane-%d", i+1),
				Type:     machine.TypeControlPlane,
				IPs:      []netip.Addr{ips[i]},
				Memory:   DefaultSettings.MemMB * 1024 * 1024,
				NanoCPUs: DefaultSettings.CPUs * 1000 * 1000 * 1000,
				Disks: []*provision.Disk{
					{
						Size: DefaultSettings.DiskGB * 1024 * 1024 * 1024,
					},
				},
				Config:           suite.configBundle.ControlPlane(),
				SDStubKernelArgs: options.InjectExtraKernelArgs,
			},
		)
	}

	for i := 1; i <= options.WorkerNodes; i++ {
		request.Nodes = append(
			request.Nodes,
			provision.NodeRequest{
				Name:     fmt.Sprintf("worker-%d", i),
				Type:     machine.TypeWorker,
				IPs:      []netip.Addr{ips[options.ControlplaneNodes+i-1]},
				Memory:   DefaultSettings.MemMB * 1024 * 1024,
				NanoCPUs: DefaultSettings.CPUs * 1000 * 1000 * 1000,
				Disks: []*provision.Disk{
					{
						Size: DefaultSettings.DiskGB * 1024 * 1024 * 1024,
					},
				},
				Config:           suite.configBundle.Worker(),
				SDStubKernelArgs: options.InjectExtraKernelArgs,
			},
		)
	}

	provisionerOptions := []provision.Option{
		provision.WithBootlader(true),
		provision.WithUEFI(!options.WithBios),
		provision.WithTalosConfig(suite.configBundle.TalosConfig()),
	}

	suite.Cluster, err = suite.provisioner.Create(
		suite.ctx, request,
		provisionerOptions...,
	)
	suite.Require().NoError(err)

	if options.WithApplyConfig {
		clusterAccess := access.NewAdapter(suite.Cluster, provisionerOptions...)
		defer clusterAccess.Close() //nolint:errcheck

		if err := clusterAccess.ApplyConfig(suite.ctx, request.Nodes, request.SiderolinkRequest, os.Stderr); err != nil {
			suite.FailNow("failed to apply config", err.Error())
		}
	}

	c, err := clientconfig.Open("")
	suite.Require().NoError(err)

	c.Merge(suite.configBundle.TalosConfig())

	suite.Require().NoError(c.Save(""))

	suite.clusterAccess = access.NewAdapter(suite.Cluster, provision.WithTalosConfig(suite.configBundle.TalosConfig()))

	suite.Require().NoError(suite.clusterAccess.Bootstrap(suite.ctx, os.Stdout))

	suite.waitForClusterHealth()
}

// runE2E runs e2e test on the cluster.
func (suite *BaseSuite) runE2E(k8sVersion string) {
	options := hydrophone.DefaultOptions()
	options.KubernetesVersion = k8sVersion

	suite.Assert().NoError(hydrophone.Run(suite.ctx, suite.clusterAccess, options))
}
