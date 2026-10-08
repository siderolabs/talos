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
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// SELinuxSuite ...
type SELinuxSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *SELinuxSuite) SuiteName() string {
	return "api.SELinuxSuite"
}

// SetupTest ...
func (suite *SELinuxSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 10*time.Minute)

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
	workers := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeWorker)
	controlplanes := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane)

	expectedLabelsWorker := map[string]string{
		// Mounts
		constants.SystemPath:          constants.SystemSelinuxLabel,
		constants.EphemeralMountPoint: constants.EphemeralSelinuxLabel,
		constants.StateMountPoint:     constants.SystemSelinuxLabel,
		constants.SystemVarPath:       constants.SystemVarSelinuxLabel,
		constants.RunPath:             constants.RunSelinuxLabel,
		"/run/containerd":             "system_u:object_r:pod_containerd_run_t:s0",
		"/run/lock":                   "system_u:object_r:var_lock_t:s0",
		"/run/lock/lvm":               "system_u:object_r:var_lock_t:s0",
		constants.SystemRunPath:       "system_u:object_r:system_run_t:s0",
		"/var/run":                    constants.RunSelinuxLabel,
		// Runtime files
		constants.APIRuntimeSocketPath:  constants.APIRuntimeSocketLabel,
		constants.DBusClientSocketPath:  constants.DBusClientSocketLabel,
		constants.UdevRulesPath:         constants.UdevRulesLabel,
		constants.DBusServiceSocketPath: constants.DBusServiceSocketLabel,
		constants.MachineSocketPath:     constants.MachineSocketLabel,
		// Overlays
		"/etc/cni":                        constants.CNISELinuxLabel,
		constants.KubernetesConfigBaseDir: constants.KubernetesConfigSELinuxLabel,
		"/opt":                            constants.OptSELinuxLabel,
		"/opt/cni":                        "system_u:object_r:cni_plugin_t:s0",
		"/opt/containerd":                 "system_u:object_r:containerd_plugin_t:s0",
		// Directories
		"/var/lib/containerd":           "system_u:object_r:containerd_state_t:s0",
		"/var/lib/cni":                  "system_u:object_r:cni_state_t:s0",
		"/var/lib/kubelet":              "system_u:object_r:kubelet_state_t:s0",
		"/var/lib/kubelet/seccomp":      "system_u:object_r:seccomp_profile_t:s0",
		constants.LogMountPoint:         "system_u:object_r:var_log_t:s0",
		"/var/log/audit":                "system_u:object_r:audit_log_t:s0",
		constants.KubernetesAuditLogDir: "system_u:object_r:kube_log_t:s0",
		"/var/log/containers":           "system_u:object_r:containers_log_t:s0",
		"/var/log/pods":                 "system_u:object_r:pods_log_t:s0",
		// Mounts and runtime-generated files
		"/etc":                  constants.EtcSelinuxLabel,
		constants.SystemEtcPath: constants.EtcSelinuxLabel,
		// Build-time files
		"/usr/share/containers/selinux/contexts": "system_u:object_r:usr_t:s0",
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

	for _, node := range append(slices.Clone(workers), controlplanes...) {
		nodeCtx := client.WithNode(suite.ctx, node)

		if !suite.criLabelsContainers(nodeCtx) {
			continue
		}

		for _, dir := range kubeletPluginDirs {
			expectedLabelsWorker[dir] = "system_u:object_r:pod_file_t:s0"
		}

		// the audit log of the kube-apiserver carries the fixed level of the pod, the same at every boot
		expectedLabelsControlPlane[filepath.Join(constants.KubernetesAuditLogDir, "kube-apiserver.log")] = "system_u:object_r:kube_log_t:" + constants.KubernetesAPIServerSELinuxLevel

		// the kubelet relabels its directories at every start, from pod_file_t itself once they carry it
		_, err := suite.Client.ServiceRestart(nodeCtx, "kubelet")
		suite.Require().NoError(err)

		rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, "kubelet", func(svc *v1alpha1.Service, asrt *assert.Assertions) {
			asrt.True(svc.TypedSpec().Healthy && svc.TypedSpec().Running)
		})
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

		extensions, err := safe.StateListAll[*runtimeres.ExtensionStatus](nodeCtx, suite.Client.COSI)
		suite.Require().NoError(err)

		if extensions.Len() > 0 {
			suite.T().Skip("skipping SELinux test since extensions are running")
		}

		for path, label := range expectedLabels {
			req := &machineapi.ListRequest{
				Root:         path,
				ReportXattrs: true,
			}

			stream, err := suite.Client.LS(nodeCtx, req)

			suite.Require().NoError(err)

			err = helpers.ReadGRPCStream(stream, func(info *machineapi.FileInfo, node string, multipleNodes bool) error {
				// E.g. /var/lib should inherit /var label, while /var/run is a new mountpoint
				if slices.Contains(paths, info.Name) && info.Name != path {
					return nil
				}

				if slices.Contains(
					append([]string{
						constants.RunPath,
						constants.SystemRunPath,
						"/run/containerd",
						"/var/run",
						"/var/log/containers",
					}, kubeletPluginDirs...),
					path,
				) && info.Name != path {
					return nil
				}

				// these are symlinks that comes from files from extensions, and we don't set xattrs for extensions yet
				// TODO(frezbo): update the test to check for correct labels once we set xattrs for extensions
				switch info.Name {
				case "/etc/ld.so.conf", "/etc/ld.so.cache":
					return nil
				case "/usr/bin/nvidia-smi":
					return nil
				case "/usr/bin/nvidia-ctk":
					return nil
				case "/usr/bin/nvidia-cdi-hook":
					return nil
				case "/usr/bin/nvme":
					return nil
				}

				suite.Require().NotNil(info.Xattrs, "expected %s to have xattrs (checking %s)", info.Name, path)

				found := false

				for _, l := range info.Xattrs {
					if l.Name == "security.selinux" {
						got := string(bytes.Trim(l.Data, "\x00\n"))
						suite.Require().Contains(got, label, "expected %s to have label %s, got %s (checking %s)", info.Name, label, got, path)

						found = true

						break
					}
				}

				suite.Require().True(found, "expected to find security.selinux xattr for %s (checking %s)", info.Name, path)

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

type podRunner interface {
	Name() string
	Create(ctx context.Context, waitTimeout time.Duration) error
	Delete(ctx context.Context) error
	Exec(ctx context.Context, command string) (string, string, error)
}

func (suite *SELinuxSuite) readStream(stream client.MachineStream) string {
	reader, err := client.ReadStream(stream)
	suite.Require().NoError(err)

	body, err := io.ReadAll(reader)
	suite.Require().NoError(err)
	suite.Require().NoError(reader.Close())

	return string(body)
}

func (suite *SELinuxSuite) denials(node, subject string) int {
	stream, err := suite.Client.Logs(client.WithNode(suite.ctx, node), constants.SystemContainerdNamespace, common.ContainerDriver_CONTAINERD, "auditd", false, -1)
	suite.Require().NoError(err)

	return strings.Count(suite.readStream(stream), " scontext=system_u:system_r:"+subject+":")
}

var kubeletPluginDirs = []string{"/var/lib/kubelet/plugins", "/var/lib/kubelet/plugins_registry", "/var/lib/kubelet/device-plugins"}

// podDomains are the workload domains of the base policy.
var podDomains = []string{"pod_t", "pod_privileged_t"}

// criLabelsContainers reports whether the node's CRI labels containers, which the kubelet mounts reflect.
func (suite *SELinuxSuite) criLabelsContainers(nodeCtx context.Context) bool {
	spec, err := safe.StateGetByID[*k8s.KubeletSpec](nodeCtx, suite.Client.COSI, k8s.KubeletID)
	suite.Require().NoError(err)

	return slices.ContainsFunc(spec.TypedSpec().ExtraMounts, func(mount specs.Mount) bool { return mount.Destination == "/sys/fs/selinux" })
}

// labelingNode returns the address and the name of a node whose CRI labels containers, and skips the test without one,
// or in permissive mode when the test checks the denials.
func (suite *SELinuxSuite) labelingNode(enforcing bool) (string, string) {
	if enforcing && !suite.SelinuxEnforcing {
		suite.T().Skip("skipping SELinux negative tests in permissive mode")
	}

	ip := suite.RandomDiscoveredNodeInternalIP()

	if !suite.criLabelsContainers(client.WithNode(suite.ctx, ip)) {
		suite.T().Skip("skipping SELinux pod domain tests since the CRI does not label containers")
	}

	node, err := suite.GetK8sNodeByInternalIP(suite.ctx, ip)
	suite.Require().NoError(err)

	return ip, node.Name
}

func (suite *SELinuxSuite) kubeletMetric(nodeName, metric string) float64 {
	body, err := suite.Clientset.CoreV1().RESTClient().Get().Resource("nodes").Name(nodeName).SubResource("proxy").Suffix("metrics").DoRaw(suite.ctx)
	suite.Require().NoError(err)

	for line := range strings.SplitSeq(string(body), "\n") {
		if value, ok := strings.CutPrefix(line, metric+" "); ok {
			parsed, err := strconv.ParseFloat(value, 64)
			suite.Require().NoError(err)

			return parsed
		}
	}

	return 0
}

// exec creates the pod, runs the command in it and deletes it.
func (suite *SELinuxSuite) exec(pod podRunner, command string) (string, string, error) {
	suite.Require().NoError(pod.Create(suite.ctx, 5*time.Minute))

	defer pod.Delete(suite.ctx) //nolint:errcheck

	return pod.Exec(suite.ctx, command)
}

// TestPodDomains verifies the domains pods land in when the CRI labels containers, and that a type the policy does not
// let pods enter fails the container.
func (suite *SELinuxSuite) TestPodDomains() {
	_, node := suite.labelingNode(false)

	for _, test := range []struct {
		name       string
		privileged bool
		options    *corev1.SELinuxOptions
		label      string // empty when the container must fail
		enforcing  bool   // a refusal permissive mode does not enforce
	}{
		{name: "selinux-pod", label: `^system_u:system_r:pod_t:s0:c\d+,c\d+$`},
		{name: "selinux-privileged", privileged: true, label: `^system_u:system_r:pod_privileged_t:s0$`},
		{name: "selinux-spc", options: &corev1.SELinuxOptions{Type: "spc_t", Level: "s0"}, label: `^system_u:system_r:pod_privileged_t:s0$`},
		{name: "selinux-unknown", options: &corev1.SELinuxOptions{Type: "unknown_t"}},
		{name: "selinux-kubelet", options: &corev1.SELinuxOptions{Type: "kubelet_t"}, enforcing: true},
	} {
		if test.enforcing && !suite.SelinuxEnforcing {
			continue
		}

		newPod := suite.NewPod
		if test.privileged {
			newPod = suite.NewPrivilegedPod
		}

		podDef, err := newPod(test.name)
		suite.Require().NoError(err)

		pod := podDef.WithQuiet(true).WithNamespace("kube-system").WithNodeName(node).WithSELinuxOptions(test.options)

		if test.label == "" {
			suite.Assert().Error(pod.Create(suite.ctx, 20*time.Second), test.name)
			suite.Require().NoError(pod.Delete(suite.ctx))

			continue
		}

		stdout, stderr, err := suite.exec(pod, "cat /proc/self/attr/current")
		suite.Require().NoError(err, test.name)
		suite.Assert().Empty(stderr, test.name)
		suite.Assert().Regexp(test.label, strings.TrimRight(stdout, "\n\x00"), test.name)
	}
}

// TestPodMCSIsolation checks the MCS categories on a hostPath: a pod reads the files of another pod only at the same
// fixed level, which outlives the pods.
func (suite *SELinuxSuite) TestPodMCSIsolation() {
	_, node := suite.labelingNode(true)

	file := "/data/mcs-" + strconv.FormatInt(time.Now().UnixNano(), 36)

	// one pod after the other: the files outlive the pod that wrote them
	for _, step := range []struct {
		name    string
		options *corev1.SELinuxOptions
		command string
		want    string // the output, or a permission denied when empty
	}{
		{"selinux-mcs-writer", nil, "echo secret > " + file + " && echo written", "written\n"},
		{"selinux-mcs-reader", nil, "cat " + file, ""},
		{"selinux-level-first", &corev1.SELinuxOptions{Level: "s0:c600,c601"}, "echo secret > " + file + "-fixed && echo written", "written\n"},
		{"selinux-level-second", &corev1.SELinuxOptions{Level: "s0:c600,c601"}, "cat " + file + "-fixed", "secret\n"},
		{"selinux-level-other", &corev1.SELinuxOptions{Level: "s0:c602,c603"}, "cat " + file + "-fixed", ""},
	} {
		podDef, err := suite.NewPod(step.name)
		suite.Require().NoError(err)

		pod := podDef.WithQuiet(true).WithNamespace("kube-system").WithNodeName(node).WithHostVolumeMount("/var/selinux-test", "/data").
			WithSELinuxOptions(step.options)

		stdout, stderr, err := suite.exec(pod, step.command)

		if step.want == "" {
			suite.Assert().Error(err, step.name)
			suite.Assert().Contains(stderr, "Permission denied", step.name)
		} else {
			suite.Assert().NoError(err, step.name)
			suite.Assert().Equal(step.want, stdout, step.name)
		}
	}
}

// TestPrivilegedHostAccess runs a host binary from a privileged pod the way TopoLVM does and serves a socket to a sidecar, without denials.
func (suite *SELinuxSuite) TestPrivilegedHostAccess() {
	ip, node := suite.labelingNode(true)

	before := suite.denials(ip, "pod_privileged_t")

	podDef, err := suite.NewPrivilegedPod("selinux-nsenter")
	suite.Require().NoError(err)

	podDef = podDef.WithQuiet(true).WithNodeName(node).WithHostPID()

	suite.Require().NoError(podDef.Create(suite.ctx, 5*time.Minute))

	defer podDef.Delete(suite.ctx) //nolint:errcheck

	stdout, stderr, err := podDef.Exec(suite.ctx, "nsenter -t 1 -m -- /usr/bin/lvm version")
	suite.Require().NoError(err)
	suite.Assert().Empty(stderr, "stderr: %s", stderr)
	suite.Assert().Contains(stdout, "LVM version")

	_, _, err = podDef.Exec(suite.ctx, "apk add --update socat && (socat UNIX-LISTEN:/host/var/selinux-test/"+podDef.Name()+",fork EXEC:cat >/dev/null 2>&1 &) && sleep 1")
	suite.Require().NoError(err)

	sidecar, err := suite.NewPod("selinux-socket-client")
	suite.Require().NoError(err)

	sidecar = sidecar.WithQuiet(true).WithNamespace("kube-system").WithNodeName(node).WithHostVolumeMount("/var/selinux-test", "/data")

	stdout, _, err = suite.exec(sidecar, "apk add --update socat >/dev/null && echo hello | socat - UNIX-CONNECT:/data/"+podDef.Name())
	suite.Require().NoError(err)
	suite.Assert().Equal("hello\n", stdout)
	suite.Assert().Equal(before, suite.denials(ip, "pod_privileged_t"))
}

// TestPodProcAccess reads the /proc of every host process from a pod sharing the host PID namespace: pod_t is denied and
// the denial audited.
func (suite *SELinuxSuite) TestPodProcAccess() {
	ip, node := suite.labelingNode(true)

	for _, test := range []struct {
		domain  string
		command string
		want    string // a regexp the output matches, empty when the command must be denied and audited
	}{
		// the shell glob drops the entries it cannot search, hence the explicit loop
		{domain: "pod_t", command: "rc=0; for p in $(ls /proc | grep -E '^[0-9]+$'); do cat /proc/$p/comm >/dev/null || rc=1; done; exit $rc"},
	} {
		podDef, err := suite.NewPod("selinux-proc-" + strings.ReplaceAll(test.domain, "_", "-"))
		suite.Require().NoError(err)

		pod := podDef.WithQuiet(true).WithNamespace("kube-system").WithNodeName(node).WithHostPID().
			WithSELinuxOptions(&corev1.SELinuxOptions{Type: test.domain})

		before := suite.denials(ip, test.domain)
		stdout, stderr, err := suite.exec(pod, test.command)
		denied := suite.denials(ip, test.domain) - before

		if test.want == "" {
			suite.Assert().Error(err, test.domain)
			suite.Assert().Contains(stderr, "Permission denied", test.domain)
			suite.Assert().Positive(denied, test.domain)
		} else {
			suite.Assert().NoError(err, test.domain)
			suite.Assert().Regexp(test.want, stdout, test.domain)
			suite.Assert().Zero(denied, test.domain)
		}
	}
}

// TestKubeletVolumeLabels checks the kubelet sees SELinux when the CRI labels containers and computes the mount label of a ReadWriteOncePod volume.
func (suite *SELinuxSuite) TestKubeletVolumeLabels() {
	_, nodeName := suite.labelingNode(false)

	name := "selinux-test-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	driver := name + ".csi.talos.dev"
	metric := `volume_manager_selinux_volumes_admitted_total{access_mode="RWOP",volume_plugin="kubernetes.io/csi/` + driver + `"}`

	_, err := suite.Clientset.StorageV1().CSIDrivers().Create(suite.ctx, &storagev1.CSIDriver{
		Name: driver,
		Spec: storagev1.CSIDriverSpec{AttachRequired: new(false), SELinuxMount: new(true)},
	}, metav1.CreateOptions{})
	suite.Require().NoError(err)

	defer suite.Clientset.StorageV1().CSIDrivers().Delete(suite.ctx, driver, metav1.DeleteOptions{}) //nolint:errcheck

	_, err = suite.Clientset.CoreV1().PersistentVolumes().Create(suite.ctx, &corev1.PersistentVolume{
		Name: name,
		Spec: corev1.PersistentVolumeSpec{
			Capacity:               corev1.ResourceList{corev1.ResourceStorage: apiresource.MustParse("1Mi")},
			AccessModes:            []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOncePod},
			StorageClassName:       name,
			PersistentVolumeSource: corev1.PersistentVolumeSource{CSI: &corev1.CSIPersistentVolumeSource{Driver: driver, VolumeHandle: name}},
		},
	}, metav1.CreateOptions{})
	suite.Require().NoError(err)

	defer suite.Clientset.CoreV1().PersistentVolumes().Delete(suite.ctx, name, metav1.DeleteOptions{}) //nolint:errcheck

	_, err = suite.Clientset.CoreV1().PersistentVolumeClaims("kube-system").Create(suite.ctx, &corev1.PersistentVolumeClaim{
		Name: name,
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOncePod},
			StorageClassName: new(name),
			VolumeName:       name,
			Resources:        corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: apiresource.MustParse("1Mi")}},
		},
	}, metav1.CreateOptions{})
	suite.Require().NoError(err)

	defer suite.Clientset.CoreV1().PersistentVolumeClaims("kube-system").Delete(suite.ctx, name, metav1.DeleteOptions{}) //nolint:errcheck

	before := suite.kubeletMetric(nodeName, metric)

	podDef, err := suite.NewPod("selinux-pvc")
	suite.Require().NoError(err)

	podDef = podDef.WithQuiet(true).WithNamespace("kube-system").WithNodeName(nodeName).
		WithSELinuxOptions(&corev1.SELinuxOptions{Level: "s0:c600,c601"}).WithPersistentVolumeClaim(name, "/data")

	// no driver serves the volume, the pod never runs
	suite.Assert().Error(podDef.Create(suite.ctx, 20*time.Second))

	defer podDef.Delete(suite.ctx) //nolint:errcheck

	suite.Require().NoError(retry.Constant(time.Minute, retry.WithUnits(time.Second)).Retry(func() error {
		if suite.kubeletMetric(nodeName, metric) <= before {
			return retry.ExpectedErrorf("the kubelet did not admit the volume with a SELinux label")
		}

		return nil
	}))
}

// TestNoHostDenials checks that the policy defines every kernel permission and that AVC denials only ever hit pods, never the host domains.
func (suite *SELinuxSuite) TestNoHostDenials() {
	if !suite.SelinuxEnforcing {
		suite.T().Skip("skipping SELinux negative tests in permissive mode")
	}

	for _, node := range suite.DiscoverNodeInternalIPs(suite.ctx) {
		nodeCtx := client.WithNode(suite.ctx, node)

		dmesg, err := suite.Client.Dmesg(nodeCtx, false, false)
		suite.Require().NoError(err)

		audit, err := suite.Client.Logs(nodeCtx, constants.SystemContainerdNamespace, common.ContainerDriver_CONTAINERD, "auditd", false, -1)
		suite.Require().NoError(err)

		for _, stream := range []client.MachineStream{dmesg, audit} {
			for line := range strings.SplitSeq(suite.readStream(stream), "\n") {
				suite.Assert().NotContains(line, "not defined in policy")

				if !strings.Contains(line, "avc:  denied") {
					continue
				}

				_, scontext, _ := strings.Cut(line, " scontext=")
				scontext, _, _ = strings.Cut(scontext, " ")

				fields := strings.SplitN(scontext, ":", 4)
				suite.Require().Len(fields, 4, "unexpected AVC record: %s", line)
				suite.Assert().True(slices.Contains(podDomains, fields[2]) || strings.Contains(fields[3], ":c"), "host domain denied: %s", line)
			}
		}
	}
}

// TODO: test labels for unconfined system extensions

// TestNoPtrace confirms ptracing system processes is prohibited in enforcing mode.
func (suite *SELinuxSuite) TestNoPtrace() {
	if !suite.SelinuxEnforcing {
		suite.T().Skip("skipping SELinux negative tests in permissive mode")
	}

	podDef, err := suite.NewPrivilegedPod("pid1-ptrace-test")
	suite.Require().NoError(err)

	podDef = podDef.WithQuiet(true)

	suite.Require().NoError(podDef.Create(suite.ctx, 5*time.Minute))

	defer podDef.Delete(suite.ctx) //nolint:errcheck

	_, stderr, err := podDef.Exec(
		suite.ctx,
		"apk add --update strace",
	)

	suite.Assert().NoError(err)
	suite.Assert().Empty(stderr, "stderr: %s", stderr)

	// if attached, timeout
	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	_, stderr, err = podDef.Exec(
		ctx,
		"strace -p 1",
	)

	// in case of successful attach it will be context.DeadlineExceeded
	suite.Require().Error(err)
	suite.Assert().ErrorContains(err, "command terminated with exit code 1")
	// strace first tests ptrace against itself, which we also deny currently
	suite.Assert().Contains(stderr, "strace: do_test_ptrace_get_syscall_info: PTRACE_TRACEME: Permission denied")
	suite.Assert().Contains(stderr, "strace: attach: ptrace(PTRACE_SEIZE, 1): Permission denied")
	suite.Assert().NotContains(stderr, "attached")
}

// TestNoMachineSocketAccess confirms pods cannot reach machined socket (not apid, but unsecured one).
func (suite *SELinuxSuite) TestNoMachineSocketAccess() {
	if !suite.SelinuxEnforcing {
		suite.T().Skip("skipping SELinux negative tests in permissive mode")
	}

	podDef, err := suite.NewPrivilegedPod("pid1-socket-test")
	suite.Require().NoError(err)

	podDef = podDef.WithQuiet(true)

	suite.Require().NoError(podDef.Create(suite.ctx, 5*time.Minute))

	defer podDef.Delete(suite.ctx) //nolint:errcheck

	_, stderr, err := podDef.Exec(
		suite.ctx,
		"apk add --update socat",
	)

	suite.Assert().NoError(err)
	suite.Assert().Empty(stderr, "stderr: %s", stderr)

	// if attached, timeout
	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	_, stderr, err = podDef.Exec(
		ctx,
		"socat - UNIX-CONNECT:/host/system/run/machined/machine.sock",
	)

	// in case of successful attach it will be context.DeadlineExceeded
	suite.Require().Error(err)
	suite.Assert().ErrorContains(err, "command terminated with exit code 1")
	suite.Assert().Contains(stderr, "Permission denied")
}

// TestNoStateAccess verifies mounting STATE does not allow /system/state/config.yaml access.
//
// STATE carries no xattr labels and machined only mounts it transiently with context=system_state_t, so a
// mount of the device from a pod shows its files as unlabeled_t, which no pod domain may read. The
// system_state_t type itself is a neverallow for every pod domain, checked by secilc at compile time.
func (suite *SELinuxSuite) TestNoStateAccess() {
	if !suite.SelinuxEnforcing {
		suite.T().Skip("skipping SELinux negative tests in permissive mode")
	}

	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	state, err := safe.StateGetByID[*block.VolumeStatus](nodeCtx, suite.Client.COSI, "STATE")
	suite.Assert().NoError(err)

	podDef, err := suite.NewPrivilegedPod("system-state-test")
	suite.Require().NoError(err)

	podDef = podDef.WithQuiet(true)

	suite.Require().NoError(podDef.Create(suite.ctx, 5*time.Minute))

	defer podDef.Delete(suite.ctx) //nolint:errcheck

	_, stderr, err := podDef.Exec(
		suite.ctx,
		"mount "+state.TypedSpec().MountLocation+" /mnt",
	)

	suite.Assert().NoError(err)
	suite.Assert().Empty(stderr, "stderr: %s", stderr)

	_, stderr, err = podDef.Exec(
		suite.ctx,
		"cat /mnt/config.yaml",
	)

	suite.Require().Error(err)
	suite.Assert().ErrorContains(err, "command terminated with exit code 1")
	suite.Assert().Contains(stderr, "cat: can't open '/mnt/config.yaml': Permission denied")
}

func init() {
	allSuites = append(allSuites, new(SELinuxSuite))
}
