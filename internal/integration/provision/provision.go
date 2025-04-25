// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

// Package provision provides integration tests which rely on provisioning cluster per test.
package provision

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-blockdevice/v2/encryption"
	"github.com/siderolabs/go-kubernetes/kubernetes/upgrade"
	"github.com/siderolabs/go-retry/retry"
	sideronet "github.com/siderolabs/net"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/cluster/check"
	"github.com/siderolabs/talos/pkg/cluster/hydrophone"
	"github.com/siderolabs/talos/pkg/cluster/kubernetes"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
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

func (suite *BaseSuite) readVersion(nodeCtx context.Context, client *talosclient.Client) (
	version string,
	err error,
) {
	var v *machineapi.VersionResponse

	v, err = client.Version(nodeCtx)
	if err != nil {
		return
	}

	version = v.Messages[0].Version.Tag

	return
}

type upgradeOptions struct {
	TargetInstallerImage string
	UpgradeStage         bool
	TargetVersion        string
}

//nolint:gocyclo
func (suite *BaseSuite) upgradeNode(client *talosclient.Client, node provision.NodeInfo, options upgradeOptions) {
	suite.T().Logf("upgrading node %s", node.IPs[0])

	ctx, cancel := context.WithCancel(suite.ctx)
	defer cancel()

	nodeCtx := talosclient.WithNodes(ctx, node.IPs[0].String())

	var (
		resp *machineapi.UpgradeResponse
		err  error
	)

	err = retry.Constant(time.Minute, retry.WithUnits(10*time.Second)).Retry(
		func() error {
			resp, err = client.Upgrade(
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
	suite.Require().NoError(client.EventsWatchV2(nodeCtx, eventCh, talosclient.WithActorID(actorID), talosclient.WithTailEvents(-1)))

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
		case <-ctx.Done():
			suite.FailNow("context canceled")
		}
	}

	// wait for the apid to be shut down
	time.Sleep(10 * time.Second)

	// wait for the version to be equal to target version
	suite.Require().NoError(
		retry.Constant(10 * time.Minute).Retry(
			func() error {
				var version string

				version, err = suite.readVersion(nodeCtx, client)
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
	}

	suite.Require().NoError(kubernetes.Upgrade(suite.ctx, suite.clusterAccess, options))
}

type clusterOptions struct {
	ClusterName string

	ControlplaneNodes int
	WorkerNodes       int

	SourceKernelPath     string
	SourceInitramfsPath  string
	SourceDiskImagePath  string
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

	if options.SourceDiskImagePath != "" {
		request.DiskImagePath = options.SourceDiskImagePath
	} else {
		request.KernelPath = options.SourceKernelPath
		request.InitramfsPath = options.SourceInitramfsPath
	}

	suite.controlPlaneEndpoint = suite.provisioner.GetExternalKubernetesControlPlaneEndpoint(request.Network, constants.DefaultControlPlanePort)

	genOptions := suite.provisioner.GenOptions(request.Network)

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

	if options.WithEncryption {
		genOptions = append(
			genOptions, generate.WithSystemDiskEncryption(
				&v1alpha1.SystemDiskEncryptionConfig{
					StatePartition: &v1alpha1.EncryptionConfig{
						EncryptionProvider: encryption.LUKS2,
						EncryptionKeys: []*v1alpha1.EncryptionKey{
							{
								KeyNodeID: &v1alpha1.EncryptionKeyNodeID{},
								KeySlot:   0,
							},
						},
					},
					EphemeralPartition: &v1alpha1.EncryptionConfig{
						EncryptionProvider: encryption.LUKS2,
						EncryptionKeys: []*v1alpha1.EncryptionKey{
							{
								KeyNodeID: &v1alpha1.EncryptionKeyNodeID{},
								KeySlot:   0,
							},
						},
					},
				},
			),
		)
	}

	versionContract, err := config.ParseContractFromVersion(options.SourceVersion)
	suite.Require().NoError(err)

	suite.configBundle, err = bundle.NewBundle(
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
				Config: suite.configBundle.ControlPlane(),
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
				Config: suite.configBundle.Worker(),
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
