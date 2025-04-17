// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create //nolint:testpackage

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/siderolabs/go-procfs/procfs"
	sideronet "github.com/siderolabs/net"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/provision"
)

type testProvisioner struct {
	provision.Provisioner
}

func prepTest() (testClusterMaker, qemuOps) { //nolint:unparam
	cm := getTestClustermaker()

	return cm, qemuOps{}
}

func getGenOpts(t *testing.T, finalizedClusterMaker testClusterMaker) generate.Options {
	cm := finalizedClusterMaker
	result, err := generate.NewInput(cm.finalReq.Name, "cluster.endpoint", "k8sv1", cm.genOpts...)
	assert.NoError(t, err)

	return result.Options
}

/*
// Generate options.
*/
func TestNodeInstallImageOption(t *testing.T) {
	cm, ops := prepTest()
	ops.nodeInstallImage = "test-image"

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)
	options := getGenOpts(t, cm)

	assert.Equal(t, "test-image", options.InstallImage)
}

func TestBootloaderEnabledOptionTrue(t *testing.T) {
	cm, ops := prepTest()
	ops.bootloaderEnabled = true

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)
	genOpts := getGenOpts(t, cm)
	provisionOpts, err := cm.getProvisionOpts()
	assert.NoError(t, err)

	assert.Equal(t, "", genOpts.Sysctls["kernel.kexec_load_disabled"])
	assert.Equal(t, true, provisionOpts.BootloaderEnabled)
}

func TestBootloaderEnabledOptionFalse(t *testing.T) {
	cm, ops := prepTest()
	ops.bootloaderEnabled = false

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)
	genOpts := getGenOpts(t, cm)
	provisionOpts, err := cm.getProvisionOpts()
	assert.NoError(t, err)

	assert.Equal(t, "1", genOpts.Sysctls["kernel.kexec_load_disabled"])
	assert.Equal(t, false, provisionOpts.BootloaderEnabled)
}

func TestEncriptionOptions(t *testing.T) {
	cm, ops := prepTest()
	ops.encryptEphemeralPartition = true
	ops.encryptStatePartition = true
	ops.diskEncryptionKeyTypes = []string{"kms", "uuid"}

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)
	genOpts := getGenOpts(t, cm)

	assert.Equal(t, 2, len(genOpts.SystemDiskEncryptionConfig.EphemeralPartition.EncryptionKeys))
	assert.Equal(t, 2, len(genOpts.SystemDiskEncryptionConfig.StatePartition.EncryptionKeys))
}

/*
// Provision options.
*/
func TestTargetArchOption(t *testing.T) {
	cm, ops := prepTest()
	ops.targetArch = "arm64"

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)
	provisionOpts, err := cm.getProvisionOpts()
	assert.NoError(t, err)

	assert.Equal(t, "arm64", provisionOpts.TargetArch)
}

func TestWithIOMMUOption(t *testing.T) {
	cm, ops := prepTest()
	ops.withIOMMU = true

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)
	provisionOpts, err := cm.getProvisionOpts()
	assert.NoError(t, err)

	assert.Equal(t, true, provisionOpts.IOMMUEnabled)
}

func TestUefiEnabledOption(t *testing.T) {
	cm, ops := prepTest()
	ops.uefiEnabled = true

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)
	provisionOpts, err := cm.getProvisionOpts()
	assert.NoError(t, err)

	assert.Equal(t, true, provisionOpts.UEFIEnabled)
}

func TestExtraUEFISearchPathsOption(t *testing.T) {
	cm, ops := prepTest()
	ops.extraUEFISearchPaths = []string{"/test-1", "test-2"}

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)
	provisionOpts, err := cm.getProvisionOpts()
	assert.NoError(t, err)

	assert.Equal(t, []string{"/test-1", "test-2"}, provisionOpts.ExtraUEFISearchPaths)
}

func TestTpm2EnabledOption(t *testing.T) {
	cm, ops := prepTest()
	ops.tpm2Enabled = true

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)
	provisionOpts, err := cm.getProvisionOpts()
	assert.NoError(t, err)

	assert.Equal(t, true, provisionOpts.TPM2Enabled)
}

/*
// Cluster request options.
*/
func TestSimplePathOptions(t *testing.T) {
	cm, ops := prepTest()
	ops.nodeVmlinuzPath = "/test-path-kernel"
	ops.nodeInitramfsPath = "/test-path-initramfs"
	ops.nodeISOPath = "/test-path-iso"
	ops.nodeUSBPath = "/test-path-usb"
	ops.nodeUKIPath = "/test-path-uki"
	ops.nodeDiskImagePath = "/test-path-disk-img"

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)

	assert.Equal(t, "/test-path-kernel", cm.finalReq.KernelPath)
	assert.Equal(t, "/test-path-initramfs", cm.finalReq.InitramfsPath)
	assert.Equal(t, "/test-path-iso", cm.finalReq.ISOPath)
	assert.Equal(t, "/test-path-usb", cm.finalReq.USBPath)
	assert.Equal(t, "/test-path-uki", cm.finalReq.UKIPath)
	assert.Equal(t, "/test-path-disk-img", cm.finalReq.DiskImagePath)
}

func TestNodeIPXEBootScriptOption(t *testing.T) {
	cm, ops := prepTest()
	ops.nodeIPXEBootScript = "test-script"

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)

	assert.Equal(t, "test-script", cm.finalReq.IPXEBootScript)
}

func TestExtraBootKernelArgsOption(t *testing.T) {
	cm, ops := prepTest()
	ops.extraBootKernelArgs = "test=arg"

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)

	assert.Equal(t, "test=arg", cm.finalReq.Nodes[0].ExtraKernelArgs.String())
	assert.Equal(t, "test=arg", cm.finalReq.Nodes[3].ExtraKernelArgs.String())
}

func TestConfigInjectionMethodOption(t *testing.T) {
	cm, ops := prepTest()
	ops.configInjectionMethodFlagVal = "metal-iso"

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)

	assert.Equal(t, provision.ConfigInjectionMethodMetalISO, cm.finalReq.Nodes[0].ConfigInjectionMethod)
	assert.Equal(t, provision.ConfigInjectionMethodMetalISO, cm.finalReq.Nodes[3].ConfigInjectionMethod)
}

func TestWithSiderolinkAgentOption(t *testing.T) {
	cm, ops := prepTest()
	ops.withSiderolinkAgent = 1
	gatewayIP, err := netip.ParseAddr("10.50.0.0")
	assert.NoError(t, err)

	cm.partialReq.Network.GatewayAddrs = []netip.Addr{gatewayIP}

	err = _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)
	provisionOpts, err := cm.getProvisionOpts()
	assert.NoError(t, err)

	assert.Contains(t, getKernelArgs(t, cm.finalReq.Nodes[0].ExtraKernelArgs), "apiUrl: grpc://10.50.0.0")
	assert.Contains(t, getKernelArgs(t, cm.finalReq.Nodes[3].ExtraKernelArgs), "apiUrl: grpc://10.50.0.0")
	assert.True(t, provisionOpts.SiderolinkEnabled)
}

func getKernelArgs(t *testing.T, extraKernelArgs *procfs.Cmdline) string {
	first := extraKernelArgs.Parameters[0].First()
	extraKernelArgsDecoded, err := base64.StdEncoding.DecodeString(*first)
	assert.NoError(t, err)
	zr, err := zstd.NewReader(bytes.NewReader([]byte(extraKernelArgsDecoded)))
	defer zr.Close()
	assert.NoError(t, err)

	extraKernelArgsStr, err := io.ReadAll(zr)
	assert.NoError(t, err)
	return string(extraKernelArgsStr)
}

func TestWithUUIDHostnames(t *testing.T) {
	cm, ops := prepTest()
	ops.withUUIDHostnames = true

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)

	assert.Contains(t, cm.finalReq.Nodes[0].Name, "machine-")
	assert.Equal(t, 6, len(strings.Split(cm.finalReq.Nodes[0].Name, "-")))

	assert.Contains(t, cm.finalReq.Nodes[3].Name, "machine-")
	assert.Equal(t, 6, len(strings.Split(cm.finalReq.Nodes[3].Name, "-")))
}

func TestNoMasqueradeCIDRsOption(t *testing.T) {
	cm, ops := prepTest()
	ops.networkNoMasqueradeCIDRs = []string{"10.50.0.0/32"}

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)

	expected, err := netip.ParsePrefix("10.50.0.0/32")
	if err != nil {
		panic(err)
	}

	assert.Equal(t, 1, len(cm.finalReq.Network.NoMasqueradeCIDRs))
	assert.Equal(t, expected, cm.finalReq.Network.NoMasqueradeCIDRs[0])
}

func TestDhcpSkipHostnameOption(t *testing.T) {
	cm, ops := prepTest()
	ops.dhcpSkipHostname = true

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)

	assert.Equal(t, true, cm.finalReq.Network.DHCPSkipHostname)
}

func TestNameserverIPsOption(t *testing.T) {
	cm, ops := prepTest()
	ops.nameservers = []string{"1.1.1.1", "2.2.2.2"}

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)

	assert.Equal(t, 2, len(cm.finalReq.Network.Nameservers))
	assert.Equal(t, "1.1.1.1", cm.finalReq.Network.Nameservers[0].String())
	assert.Equal(t, "2.2.2.2", cm.finalReq.Network.Nameservers[1].String())
}

func TestBadRTCOption(t *testing.T) {
	cm, ops := prepTest()
	ops.badRTC = true

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)

	assert.True(t, cm.finalReq.Nodes[0].BadRTC)
	assert.True(t, cm.finalReq.Nodes[3].BadRTC)
}

func TestNetworkChaosOptions(t *testing.T) {
	cm, ops := prepTest()
	ops.networkChaos = true
	ops.jitter = time.Hour
	ops.latency = time.Millisecond
	ops.packetLoss = 1
	ops.packetReorder = 2
	ops.packetCorrupt = 3
	ops.bandwidth = 4

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)

	assert.Equal(t, true, cm.finalReq.Network.NetworkChaos)
	assert.Equal(t, time.Hour, cm.finalReq.Network.Jitter)
	assert.Equal(t, time.Millisecond, cm.finalReq.Network.Latency)
	assert.EqualValues(t, 1, cm.finalReq.Network.PacketLoss)
	assert.EqualValues(t, 2, cm.finalReq.Network.PacketReorder)
	assert.EqualValues(t, 3, cm.finalReq.Network.PacketCorrupt)
	assert.EqualValues(t, 4, cm.finalReq.Network.Bandwidth)
}

func TestCNIOptions(t *testing.T) {
	cm, ops := prepTest()
	ops.cniBinPath = []string{"/test-path"}
	ops.cniBundleURL = "bundle.url"
	ops.cniCacheDir = "/test-cache-dir"
	ops.cniConfDir = "/cni-conf"

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)

	assert.Equal(t, []string{"/test-path"}, cm.finalReq.Network.CNI.BinPath)
	assert.Equal(t, "bundle.url", cm.finalReq.Network.CNI.BundleURL)
	assert.Equal(t, "/test-cache-dir", cm.finalReq.Network.CNI.CacheDir)
	assert.Equal(t, "/cni-conf", cm.finalReq.Network.CNI.ConfDir)
}

/*
// Other options.
*/

func (testProvisioner) GetFirstInterface() v1alpha1.IfaceSelector {
	return v1alpha1.IfaceByName("eth0")
}

func TestVIPOption(t *testing.T) {
	cm, ops := prepTest()
	ops.useVIP = true

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)
	provisionOpts, err := cm.getProvisionOpts()
	assert.NoError(t, err)
	genOpts := getGenOpts(t, cm)
	assert.NoError(t, err)

	expectedIP, err := sideronet.NthIPInNetwork(cm.cidr4, vipOffset)
	assert.NoError(t, err)

	assert.Equal(t, "https://"+expectedIP.String()+":0", provisionOpts.KubernetesEndpoint)

	netcfg := v1alpha1.NetworkConfig{}
	for _, o := range genOpts.NetworkConfigOptions {
		err := o(machine.TypeControlPlane, &netcfg)
		assert.NoError(t, err)
	}

	assert.NotNil(t, netcfg.NetworkInterfaces[0].DeviceVIPConfig)
}

func TestWithFirewallOption(t *testing.T) {
	cm, ops := prepTest()
	gatewayIP, err := netip.ParseAddr("10.50.0.0")
	assert.NoError(t, err)

	cm.partialReq.Network.GatewayAddrs = []netip.Addr{gatewayIP}
	ops.withFirewall = "block"

	err = _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)
	cfgBudnle := cm.getCfgBundleOpts(t)

	assert.Equal(t, 1, len(cfgBudnle.PatchesControlPlane))
	assert.Equal(t, 1, len(cfgBudnle.PatchesWorker))
}

func TestDebugShellEnabledOptionTrue(t *testing.T) {
	cm, ops := prepTest()
	ops.debugShellEnabled = true

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)

	assert.False(t, cm.postCreateCalled)
}

func TestDebugShellEnabledOptionFalse(t *testing.T) {
	cm, ops := prepTest()
	ops.debugShellEnabled = false

	err := _createQemuCluster(context.Background(), ops, commonOps{}, testProvisioner{}, &cm)
	assert.NoError(t, err)

	assert.True(t, cm.postCreateCalled)
}
