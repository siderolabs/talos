// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"bytes"
	"context"
	"io"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// SELinuxSuite ...
type SELinuxSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *SELinuxSuite) SuiteName() string {
	return "api.SELinuxSuite"
}

// SetupTest ...
func (suite *SELinuxSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 15*time.Second)

	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping SELinux test since provisioner is not qemu")
	}
}

// TearDownTest ...
func (suite *SELinuxSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

func (suite *SELinuxSuite) getLabel(nodeCtx context.Context, pid int32) string {
	r, err := suite.Client.Read(nodeCtx, filepath.Join("/proc", strconv.Itoa(int(pid)), "attr/current"))
	suite.Require().NoError(err)

	value, err := io.ReadAll(r)
	suite.Require().NoError(err)

	suite.Require().NoError(r.Close())

	return string(bytes.TrimSpace(value))
}

// TestFileMountLabels reads labels of runtime-created files and mounts from xattrs
// to ensure SELinux labels for files are set when they are created and FS's are mounted with correct labels.
// FIXME: cancel the test in case system was upgraded.
func (suite *SELinuxSuite) TestFileMountLabels() {
	suite.T().Skip("skipping this test until it becomes stable enough")

	workers := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeWorker)
	controlplanes := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane)

	expectedLabelsWorker := map[string]string{
		// Mounts
		constants.SystemPath:          constants.SystemSelinuxLabel,
		constants.EphemeralMountPoint: constants.EphemeralSelinuxLabel,
		constants.StateMountPoint:     constants.StateSelinuxLabel,
		constants.SystemVarPath:       constants.SystemVarSelinuxLabel,
		constants.RunPath:             constants.RunSelinuxLabel,
		"/var/run":                    constants.RunSelinuxLabel,
		// Runtime files
		constants.APIRuntimeSocketPath:  constants.APIRuntimeSocketLabel,
		constants.APISocketPath:         constants.APISocketLabel,
		constants.DBusClientSocketPath:  constants.DBusClientSocketLabel,
		constants.UdevRulesPath:         constants.UdevRulesLabel,
		constants.DBusServiceSocketPath: constants.DBusServiceSocketLabel,
		constants.MachineSocketPath:     constants.MachineSocketLabel,
		// Overlays
		"/etc/cni":                        constants.CNISELinuxLabel,
		constants.KubernetesConfigBaseDir: constants.KubernetesConfigSELinuxLabel,
		"/usr/libexec/kubernetes":         constants.KubeletPluginsSELinuxLabel,
		"/opt":                            constants.OptSELinuxLabel,
		"/opt/cni":                        "system_u:object_r:cni_plugin_t:s0",
		"/opt/containerd":                 "system_u:object_r:containerd_plugin_t:s0",
		// Directories
		"/var/lib/containerd": "system_u:object_r:containerd_state_t:s0",
		"/var/lib/kubelet":    "system_u:object_r:kubelet_state_t:s0",
		// Mounts and runtime-generated files
		constants.SystemEtcPath: constants.EtcSelinuxLabel,
		"/etc":                  constants.EtcSelinuxLabel,
	}

	// Only running on controlplane
	expectedLabelsControlPlane := map[string]string{
		constants.EtcdPKIPath:                           constants.EtcdPKISELinuxLabel,
		constants.EtcdDataPath:                          constants.EtcdDataSELinuxLabel,
		constants.KubernetesAPIServerConfigDir:          constants.KubernetesAPIServerConfigDirSELinuxLabel,
		constants.KubernetesAPIServerSecretsDir:         constants.KubernetesAPIServerSecretsDirSELinuxLabel,
		constants.KubernetesControllerManagerSecretsDir: constants.KubernetesControllerManagerSecretsDirSELinuxLabel,
		constants.KubernetesSchedulerConfigDir:          constants.KubernetesSchedulerConfigDirSELinuxLabel,
		constants.KubernetesSchedulerSecretsDir:         constants.KubernetesSchedulerSecretsDirSELinuxLabel,
		constants.TrustdRuntimeSocketPath:               constants.TrustdRuntimeSocketLabel,
	}
	maps.Copy(expectedLabelsControlPlane, expectedLabelsWorker)

	// Devices labeled by subsystems, labeled by udev
	expectedLabelsDevices := map[string]string{
		"/dev/rtc0":      "system_u:object_r:rtc_device_t:s0",
		"/dev/tpm0":      "system_u:object_r:tpm_device_t:s0",
		"/dev/tpmrm0":    "system_u:object_r:tpm_device_t:s0",
		"/dev/watchdog":  "system_u:object_r:wdt_device_t:s0",
		"/dev/watchdog0": "system_u:object_r:wdt_device_t:s0",
		"/dev/null":      "system_u:object_r:null_device_t:s0",
		"/dev/zero":      "system_u:object_r:null_device_t:s0",
	}

	suite.checkFileLabels(workers, expectedLabelsWorker, false)
	suite.checkFileLabels(controlplanes, expectedLabelsControlPlane, false)
	suite.checkFileLabels(workers, expectedLabelsDevices, true)
	suite.checkFileLabels(controlplanes, expectedLabelsDevices, true)
}

//nolint:gocyclo
func (suite *SELinuxSuite) checkFileLabels(nodes []string, expectedLabels map[string]string, allowMissing bool) {
	paths := make([]string, 0, len(expectedLabels))
	for k := range expectedLabels {
		paths = append(paths, k)
	}

	for _, node := range nodes {
		nodeCtx := client.WithNode(suite.ctx, node)
		cmdline := suite.ReadCmdline(nodeCtx)

		seLinuxEnabled := pointer.SafeDeref(procfs.NewCmdline(cmdline).Get(constants.KernelParamSELinux).First()) != ""
		if !seLinuxEnabled {
			suite.T().Skip("skipping SELinux test since SELinux is disabled")
		}

		// We should check both folders and their contents for proper labels
		for _, dir := range []bool{true, false} {
			for path, label := range expectedLabels {
				req := &machineapi.ListRequest{
					Root:         path,
					ReportXattrs: true,
				}
				if dir {
					req.Types = []machineapi.ListRequest_Type{machineapi.ListRequest_DIRECTORY}
				}

				stream, err := suite.Client.LS(nodeCtx, req)

				suite.Require().NoError(err)

				err = helpers.ReadGRPCStream(stream, func(info *machineapi.FileInfo, node string, multipleNodes bool) error {
					// E.g. /var/lib should inherit /var label, while /var/run is a new mountpoint
					if slices.Contains(paths, info.Name) && info.Name != path {
						return nil
					}

					suite.Require().NotNil(info.Xattrs)

					found := false

					for _, l := range info.Xattrs {
						if l.Name == "security.selinux" {
							got := string(bytes.Trim(l.Data, "\x00\n"))
							suite.Require().Contains(got, label, "expected %s to have label %s, got %s", path, label, got)

							found = true

							break
						}
					}

					suite.Require().True(found)

					return nil
				})

				if allowMissing {
					if err != nil {
						suite.Require().Contains(err.Error(), "lstat")
						suite.Require().Contains(err.Error(), "no such file or directory")
					}
				} else {
					suite.Require().NoError(err)
				}
			}
		}
	}
}

// TestProcessLabels reads labels of system processes from procfs
// to ensure SELinux labels for processes are correctly set
//
//nolint:gocyclo
func (suite *SELinuxSuite) TestProcessLabels() {
	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)

	for _, node := range nodes {
		nodeCtx := client.WithNode(suite.ctx, node)
		cmdline := suite.ReadCmdline(nodeCtx)

		seLinuxEnabled := pointer.SafeDeref(procfs.NewCmdline(cmdline).Get(constants.KernelParamSELinux).First()) != ""
		if !seLinuxEnabled {
			suite.T().Skip("skipping SELinux test since SELinux is disabled")
		}

		r, err := suite.Client.Processes(nodeCtx)
		suite.Require().NoError(err)

		for _, msg := range r.Messages {
			procs := msg.Processes

			for _, p := range procs {
				switch p.Command {
				case "systemd-udevd":
					suite.Require().Contains(
						suite.getLabel(nodeCtx, p.Pid),
						constants.SelinuxLabelUdevd,
					)
				case "dashboard":
					suite.Require().Contains(
						suite.getLabel(nodeCtx, p.Pid),
						constants.SelinuxLabelDashboard,
					)
				case "containerd":
					if strings.Contains(p.Args, "/system/run/containerd") {
						suite.Require().Contains(
							suite.getLabel(nodeCtx, p.Pid),
							constants.SelinuxLabelSystemRuntime,
						)
					} else {
						suite.Require().Contains(
							suite.getLabel(nodeCtx, p.Pid),
							constants.SelinuxLabelPodRuntime,
						)
					}
				case "init":
					suite.Require().Contains(
						suite.getLabel(nodeCtx, p.Pid),
						constants.SelinuxLabelMachined,
					)
				case "kubelet":
					suite.Require().Contains(
						suite.getLabel(nodeCtx, p.Pid),
						constants.SelinuxLabelKubelet,
					)
				case "apid":
					suite.Require().Contains(
						suite.getLabel(nodeCtx, p.Pid),
						constants.SelinuxLabelApid,
					)
				case "trustd":
					suite.Require().Contains(
						suite.getLabel(nodeCtx, p.Pid),
						constants.SelinuxLabelTrustd,
					)
				}
			}
		}
	}
}

// TestSecurityState validates SecurityState in accordance to -talos.enforcing.
func (suite *SELinuxSuite) TestSecurityState() {
	for _, node := range suite.DiscoverNodeInternalIPs(suite.ctx) {
		nodeCtx := client.WithNode(suite.ctx, node)
		cmdline := suite.ReadCmdline(nodeCtx)

		seLinuxEnabled := pointer.SafeDeref(procfs.NewCmdline(cmdline).Get(constants.KernelParamSELinux).First()) != ""
		if !seLinuxEnabled {
			continue
		}

		rtestutils.AssertResource(
			nodeCtx,
			suite.T(),
			suite.Client.COSI,
			runtimeres.SecurityStateID,
			func(state *runtimeres.SecurityState, asrt *assert.Assertions) {
				if suite.SelinuxEnforcing {
					asrt.Equal(runtimeres.SELinuxStateEnforcing, state.TypedSpec().SELinuxState)
				} else {
					asrt.Equal(runtimeres.SELinuxStatePermissive, state.TypedSpec().SELinuxState)
				}
			},
		)
	}
}

// TODO: test for system and CRI container labels
// TODO: test labels for unconfined system extensions, pods
// TODO: test for no avc denials in dmesg
// TODO: start a pod and ensure access to restricted resources is denied

func init() {
	allSuites = append(allSuites, new(SELinuxSuite))
}
