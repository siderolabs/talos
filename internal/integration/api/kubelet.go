// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	mathrand "math/rand/v2"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// KubeletSuite verifies kubelet service lifecycle: that projected volumes still receive updates
// after the kubelet service is restarted, and that kubelet configuration changes are picked up.
type KubeletSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *KubeletSuite) SuiteName() string {
	return "api.KubeletSuite"
}

// SetupTest ...
func (suite *KubeletSuite) SetupTest() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 5*time.Minute)

	suite.AssertClusterHealthy(suite.ctx)
}

// TearDownTest ...
func (suite *KubeletSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestCPUManagerPolicyChange enables the static CPU manager policy, changes its settings and disables it back,
// asserting that kubelet is restarted and picks up the new configuration each time.
//
// Kubelet refuses to start if the persisted CPU manager state doesn't match the configuration, so Talos
// should remove the state file before starting kubelet with the changed configuration.
func (suite *KubeletSuite) TestCPUManagerPolicyChange() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	cfg, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	// the test patches the KubeletConfig document, which a configuration still carrying
	// .machine.kubelet won't accept alongside it
	if !slices.ContainsFunc(cfg.Documents(), func(doc config.Document) bool {
		return doc.Kind() == k8s.KubeletConfig
	}) {
		suite.T().Skipf("the machine configuration on node %s has no %s document", node, k8s.KubeletConfig)
	}

	if _, ok := cfg.K8sKubeletConfig().ExtraConfig()["cpuManagerPolicy"]; ok {
		suite.T().Skip("cpuManagerPolicy is already set on the node")
	}

	onlineCPUs, err := cpuset.Parse(suite.ReadFile(nodeCtx, "/sys/devices/system/cpu/online"))
	suite.Require().NoError(err)

	suite.T().Logf("using node %s with online CPUs %s", node, onlineCPUs)

	// keys set in the KubeletConfig document, to be removed on cleanup (removing a key which is not set fails the patch)
	var extraConfigKeys []string

	defer func() {
		suite.T().Log("disabling the CPU manager static policy")

		deleteExtraConfig := map[string]any{}

		for _, key := range extraConfigKeys {
			deleteExtraConfig[key] = map[string]any{"$patch": "delete"}
		}

		suite.patchKubeletExtraConfig(node, deleteExtraConfig)

		suite.assertCPUManagerState(nodeCtx, "none", cpuset.New())
	}()

	suite.T().Log("enabling the CPU manager static policy")

	extraConfigKeys = append(extraConfigKeys, "cpuManagerPolicy")

	suite.patchKubeletExtraConfig(node, map[string]any{
		"cpuManagerPolicy": "static",
	})

	// without strict CPU reservation, the default (shared) cpuset includes all CPUs
	suite.assertCPUManagerState(nodeCtx, "static", onlineCPUs)

	if onlineCPUs.Size() < 2 {
		suite.T().Log("skipping the strict CPU reservation step, as the node has a single CPU")

		return
	}

	suite.T().Log("enabling strict CPU reservation")

	reservedCPUs := cpuset.New(onlineCPUs.List()[0])

	extraConfigKeys = append(extraConfigKeys, "cpuManagerPolicyOptions", "reservedSystemCPUs")

	suite.patchKubeletExtraConfig(node, map[string]any{
		"cpuManagerPolicyOptions": map[string]any{
			"strict-cpu-reservation": "true",
		},
		"reservedSystemCPUs": reservedCPUs.String(),
	})

	// with strict CPU reservation, kubelet refuses to load the previous state (which includes the reserved CPUs in the default cpuset),
	// so Talos should have removed it before starting kubelet
	suite.assertCPUManagerState(nodeCtx, "static", onlineCPUs.Difference(reservedCPUs))
}

// patchKubeletExtraConfig patches the extra kubelet configuration on the node and waits for kubelet
// to be restarted and healthy.
//
// The settings go into the KubeletConfig document rather than into the deprecated
// .machine.kubelet.extraConfig: a configuration carrying both is rejected outright, so patching the
// v1alpha1 field fails on any cluster already using the document.
func (suite *KubeletSuite) patchKubeletExtraConfig(node string, extraConfig map[string]any) {
	nodeCtx := client.WithNode(suite.ctx, node)

	lastKubeletEvent := suite.LatestServiceEventTimestamp(suite.ctx, node, "kubelet")

	// the document is merged into the one already on the node, so the image and the rest of the
	// kubelet configuration are left alone
	suite.PatchMachineConfig(nodeCtx, map[string]any{
		"apiVersion": "v1alpha1",
		"kind":       k8s.KubeletConfig,
		"config":     extraConfig,
	})

	suite.AssertServiceEventsInOrder(suite.ctx, node, "kubelet", lastKubeletEvent, []string{
		"Stopping",
		"Finished",
		"Starting",
		"Waiting",
		"Preparing",
		"Running",
	})

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI,
		"kubelet",
		func(svc *v1alpha1.Service, asrt *assert.Assertions) {
			asrt.True(svc.TypedSpec().Healthy)
			asrt.True(svc.TypedSpec().Running)
		},
	)
}

// assertCPUManagerState asserts that the CPU manager state written by kubelet has the expected policy and default cpuset.
func (suite *KubeletSuite) assertCPUManagerState(nodeCtx context.Context, expectedPolicy string, expectedDefaultCPUs cpuset.CPUSet) {
	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		asrt := assert.New(collect)

		reader, err := suite.Client.Read(nodeCtx, "/var/lib/kubelet/cpu_manager_state")
		if !asrt.NoError(err) {
			return
		}

		defer reader.Close() //nolint:errcheck

		body, err := io.ReadAll(reader)
		if !asrt.NoError(err) {
			return
		}

		var state struct {
			PolicyName    string `json:"policyName"`
			DefaultCPUSet string `json:"defaultCpuSet"`
		}

		if !asrt.NoError(json.Unmarshal(body, &state), "failed to parse the CPU manager state %q", string(body)) {
			return
		}

		asrt.Equal(expectedPolicy, state.PolicyName)

		defaultCPUs, err := cpuset.Parse(state.DefaultCPUSet)
		if !asrt.NoError(err) {
			return
		}

		asrt.True(expectedDefaultCPUs.Equals(defaultCPUs), "expected default cpuset %q, got %q", expectedDefaultCPUs, defaultCPUs)
	}, time.Minute, time.Second, "CPU manager state should match the configuration")
}

// TestProjectedVolumeUpdatesSurviveKubeletRestart creates a pod with a
// downwardAPI projected volume exposing the pod's labels, restarts the
// kubelet on the pod's node, then patches a label on the pod and asserts
// that the projected file inside the pod is updated.
//
// The bug from #13352 manifests as: after kubelet restart, the new kubelet
// writes projected-volume updates into a tmpfs that is invisible to running
// pods (because /var/lib/kubelet was bind-mounted without rbind/rshared),
// so the pod keeps reading the pre-restart value forever.
func (suite *KubeletSuite) TestProjectedVolumeUpdatesSurviveKubeletRestart() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	k8sNode, err := suite.GetK8sNodeByInternalIP(suite.ctx, node)
	suite.Require().NoError(err)

	randomSuffix := make([]byte, 4)
	_, err = rand.Read(randomSuffix)
	suite.Require().NoError(err)

	const namespace = "default"

	podName := fmt.Sprintf("kubelet-restart-%x", randomSuffix)

	pod := &corev1.Pod{
		Name:      podName,
		Namespace: namespace,
		Labels: map[string]string{
			"app":     podName,
			"version": "v1",
		},
		Spec: corev1.PodSpec{
			NodeName:      k8sNode.Name,
			RestartPolicy: corev1.RestartPolicyNever,
			Tolerations: []corev1.Toleration{
				{Operator: corev1.TolerationOpExists},
			},
			Containers: []corev1.Container{
				{
					Name:  "main",
					Image: "alpine",
					Command: []string{
						"/bin/sh",
						"-c",
						"--",
					},
					Args: []string{
						"trap : TERM INT; (tail -f /dev/null) & wait",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "podinfo",
							MountPath: "/etc/podinfo",
							ReadOnly:  true,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "podinfo",
					Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{
							{
								DownwardAPI: &corev1.DownwardAPIProjection{
									Items: []corev1.DownwardAPIVolumeFile{
										{
											Path: "labels",
											FieldRef: &corev1.ObjectFieldSelector{
												FieldPath: "metadata.labels",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	_, err = suite.Clientset.CoreV1().Pods(namespace).Create(suite.ctx, pod, metav1.CreateOptions{})
	suite.Require().NoError(err)

	defer func() {
		gracePeriod := int64(0)
		//nolint:errcheck
		suite.Clientset.CoreV1().Pods(namespace).Delete(
			context.Background(),
			podName,
			metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod},
		)
	}()

	suite.Require().NoError(suite.WaitForPodToBeRunning(suite.ctx, 2*time.Minute, namespace, podName))

	// Sanity check: the projected file should already reflect the initial label.
	suite.assertLabelEventually(namespace, podName, `version="v1"`, 30*time.Second,
		"initial projected volume should contain version=\"v1\"")

	// Restart kubelet on the node hosting the pod.
	suite.T().Logf("restarting kubelet on %s", node)

	_, err = suite.Client.ServiceRestart(nodeCtx, "kubelet")
	suite.Require().NoError(err)

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI,
		"kubelet",
		func(svc *v1alpha1.Service, asrt *assert.Assertions) {
			asrt.True(svc.TypedSpec().Healthy)
			asrt.True(svc.TypedSpec().Running)
		},
	)

	// Make sure the pod is still running (kubelet restart should not kill containerd-managed
	// containers, but if it did the test would be invalid).
	pollPod, err := suite.Clientset.CoreV1().Pods(namespace).Get(suite.ctx, podName, metav1.GetOptions{})
	suite.Require().NoError(err)
	suite.Require().Equal(corev1.PodRunning, pollPod.Status.Phase, "pod should still be Running after kubelet restart")

	// Patch the label that the projected volume exposes.
	patch := []byte(`{"metadata":{"labels":{"version":"v2"}}}`)

	_, err = suite.Clientset.CoreV1().Pods(namespace).Patch(
		suite.ctx, podName, types.StrategicMergePatchType, patch, metav1.PatchOptions{},
	)
	suite.Require().NoError(err)

	// Kubelet's default DownwardAPI sync interval is ~60s; allow generous slack.
	suite.assertLabelEventually(namespace, podName, `version="v2"`, 3*time.Minute,
		"projected volume should reflect updated label after kubelet restart")
}

// assertLabelEventually polls the projected /etc/podinfo/labels file inside the pod
// until it contains the expected substring, failing the test if the timeout elapses.
func (suite *KubeletSuite) assertLabelEventually(namespace, podName, want string, timeout time.Duration, msg string) {
	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		asrt := assert.New(collect)

		stdout, stderr, err := suite.execInPod(suite.ctx, namespace, podName, "cat /etc/podinfo/labels")
		if !asrt.NoError(err, "exec error (stderr=%q)", stderr) {
			return
		}

		asrt.Contains(stdout, want)
	}, timeout, 5*time.Second, msg)
}

// execInPod runs a command in the pod's main container and returns stdout/stderr.
func (suite *KubeletSuite) execInPod(ctx context.Context, namespace, podName, command string) (string, string, error) {
	req := suite.Clientset.CoreV1().RESTClient().Post().Resource("pods").Name(podName).
		Namespace(namespace).SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Command: []string{"/bin/sh", "-c", command},
		Stdout:  true,
		Stderr:  true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewWebSocketExecutor(suite.RestConfig, "GET", req.URL().String())
	if err != nil {
		return "", "", err
	}

	var stdout, stderr strings.Builder

	if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	}); err != nil {
		return stdout.String(), stderr.String(), err
	}

	return stdout.String(), stderr.String(), nil
}

// kubeletExtraConfigPatch preserves the node's kubelet configuration format and image.
func kubeletExtraConfigPatch(cfg config.K8sKubeletConfig, extraConfig map[string]any) any {
	if _, ok := cfg.(*k8s.KubeletConfigV1Alpha1); ok {
		patch := k8s.NewKubeletConfigV1Alpha1()
		patch.KubeletConfig.Object = extraConfig

		return patch
	}

	return map[string]any{
		"machine": map[string]any{
			"kubelet": map[string]any{
				"extraConfig": extraConfig,
			},
		},
	}
}

// TestKubepodsMemoryLimitEnforced verifies that the kernel enforces the kubepods limit on a worker
// pod which has no memory limit of its own: it is OOM-killed by the kubepods memcg, and only there.
//
//nolint:gocyclo,cyclop
func (suite *KubeletSuite) TestKubepodsMemoryLimitEnforced() {
	if !suite.Capabilities().RunsTalosKernel {
		suite.T().Skip("cgroups are nested in container mode, so the kubepods limit is inactive")
	}

	if suite.Cluster == nil {
		suite.T().Skip("cluster state is required to tell workers from control planes")
	}

	// Control planes are never used: their static pods live under kubepods too.
	workers := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeWorker)
	if len(workers) == 0 {
		suite.T().Skip("cluster has no worker nodes")
	}

	node := workers[mathrand.IntN(len(workers))]
	nodeCtx := client.WithNode(suite.ctx, node)

	originalBytes := suite.RequireNoWorkloadResourceConfig(nodeCtx, node)

	cfg, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	// The limit is sized for the kubelet reservation defaults, and the eviction settings are replaced
	// below; a node carrying its own values for either is not a fair fixture.
	if kubeletCfg := cfg.K8sKubeletConfig(); kubeletCfg != nil {
		for _, key := range []string{"evictionHard", "evictionSoft", "kubeReserved", "systemReserved", "mergeDefaultEvictionSettings"} {
			if _, set := kubeletCfg.ExtraConfig()[key]; set {
				suite.T().Skipf("node %s customizes kubelet %q, the test is sized for the kubelet defaults", node, key)
			}
		}
	}

	k8sNode, err := suite.GetK8sNodeByInternalIP(suite.ctx, node)
	suite.Require().NoError(err)

	memTotal := suite.ReadMemTotal(nodeCtx)

	// A quarter of the node, MiB-aligned: above the default 100Mi eviction threshold, and the
	// derived systemReserved (MemTotal - limit) stays positive.
	const alignment = uint64(1 << 20)

	limit := memTotal / 4 / alignment * alignment

	podsCharge, err := suite.ReadCgroupUint(nodeCtx, constants.CgroupKubepods, "memory.current")
	suite.Require().NoError(err)

	if podsCharge*2 > limit {
		suite.T().Skipf("worker %s already charges %d bytes to kubepods, too close to the %d byte test limit", node, podsCharge, limit)
	}

	kubepodsMax := suite.ReadFile(nodeCtx, filepath.Join(constants.CgroupMountPath, constants.CgroupKubepods, "memory.max"))
	suite.T().Logf("worker %s (%s): MemTotal %d, kubepods memory.max %s, pods charge %d, test limit %d", node, k8sNode.Name, memTotal, kubepodsMax, podsCharge, limit)

	nodeReady := func(status corev1.ConditionStatus) bool { return status == corev1.ConditionTrue }

	kubeletRestartSince := suite.LatestServiceEventTimestamp(suite.ctx, node, "kubelet")

	suite.RestoreMachineConfigOnCleanup(node, originalBytes, func(cleanupCtx context.Context) {
		suite.AssertCgroupFile(cleanupCtx, constants.CgroupKubepods, "memory.max", kubepodsMax)
		suite.Assert().NoError(suite.WaitForK8sNodeReadinessStatus(cleanupCtx, k8sNode.Name, nodeReady))
	})

	doc := runtimecfg.NewWorkloadResourceConfigV1Alpha1()
	doc.KubepodsConfig = workloadMemoryRoot(limit)

	// The kubelet watches memory.available against allocatable (limit - threshold) and evicts the
	// victim shortly before the charge reaches the kernel limit. Memory eviction is switched off for
	// the test, so the root's own limit is the only ceiling; the disk signals keep their defaults.
	// Both patches go in one apply, so the kubelet restarts once.
	kubeletPatch := kubeletExtraConfigPatch(cfg.K8sKubeletConfig(), map[string]any{
		"evictionHard": map[string]any{
			"memory.available": "0%",
		},
		"mergeDefaultEvictionSettings": true,
	})

	suite.PatchMachineConfig(nodeCtx, doc, kubeletPatch)

	// The kubelet, not Talos, writes the kubepods limit, so it has to restart first.
	suite.AssertServiceEventsInOrder(suite.ctx, node, "kubelet", kubeletRestartSince, []string{"Stopping", "Finished", "Starting", "Waiting", "Preparing", "Running"})
	suite.AssertCgroupFile(nodeCtx, constants.CgroupKubepods, "memory.max", strconv.FormatUint(limit, 10))
	suite.Require().NoError(suite.WaitForK8sNodeReadinessStatus(suite.ctx, k8sNode.Name, nodeReady))

	unrelated := []string{constants.CgroupTalosContainersRoot, constants.CgroupVirtualMachines, constants.CgroupSystem, constants.CgroupPodRuntimeRoot}
	before := suite.MemoryEventsSnapshot(nodeCtx, append([]string{constants.CgroupKubepods}, unrelated...)...)
	started := time.Now()

	// BestEffort (no requests, no limits), so the root's limit is the only ceiling. The memory-backed
	// emptyDir holds the whole write, so ENOSPC cannot end the run before the memcg does.
	victimBytes := 2 * limit

	randomSuffix := make([]byte, 4)
	_, err = rand.Read(randomSuffix)
	suite.Require().NoError(err)

	const namespace = "default"

	podName := fmt.Sprintf("kubepods-memcap-%x", randomSuffix)
	sizeLimit := resource.NewQuantity(int64(victimBytes+alignment), resource.BinarySI)

	victim := &corev1.Pod{
		Name:      podName,
		Namespace: namespace,
		Spec: corev1.PodSpec{
			NodeName:      k8sNode.Name,
			RestartPolicy: corev1.RestartPolicyNever,
			Tolerations:   []corev1.Toleration{{Operator: corev1.TolerationOpExists}},
			Containers: []corev1.Container{
				{
					Name:    "fill",
					Image:   "alpine",
					Command: []string{"/bin/sh", "-c", "--"},
					Args: []string{
						"dd if=/dev/zero of=/scratch/fill bs=1M count=" + strconv.FormatUint(victimBytes>>20, 10) + " && echo FILL_COMPLETED && sleep 3600",
					},
					VolumeMounts: []corev1.VolumeMount{{Name: "scratch", MountPath: "/scratch"}},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name:     "scratch",
					EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory, SizeLimit: sizeLimit},
				},
			},
		},
	}

	deletePod := func(ctx context.Context) error {
		err := suite.Clientset.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{GracePeriodSeconds: new(int64(0))})
		if apierrors.IsNotFound(err) {
			return nil
		}

		return err
	}

	// Registered after the config restore, so the victim is gone before the limit.
	suite.T().Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()

		suite.Assert().NoError(deletePod(cleanupCtx))
	})

	_, err = suite.Clientset.CoreV1().Pods(namespace).Create(suite.ctx, victim, metav1.CreateOptions{})
	suite.Require().NoError(err)

	after := suite.AssertMemcgOOMKill(nodeCtx, constants.CgroupKubepods, before[constants.CgroupKubepods], 3*time.Minute)

	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		pod, getErr := suite.Clientset.CoreV1().Pods(namespace).Get(suite.ctx, podName, metav1.GetOptions{})
		if !assert.NoError(collect, getErr) {
			return
		}

		if !assert.Len(collect, pod.Status.ContainerStatuses, 1, "pod phase %s", pod.Status.Phase) {
			return
		}

		if !assert.NotEqual(collect, "Evicted", pod.Status.Reason, "the kubelet evicted the victim instead of the kernel killing it: %s", pod.Status.Message) {
			return
		}

		terminated := pod.Status.ContainerStatuses[0].State.Terminated
		if !assert.NotNil(collect, terminated, "victim container not terminated yet, pod phase %s", pod.Status.Phase) {
			return
		}

		assert.Equal(collect, "OOMKilled", terminated.Reason, "victim terminated for another reason: %+v", terminated)
		assert.EqualValues(collect, 137, terminated.ExitCode)
	}, 2*time.Minute, 2*time.Second, "victim pod should be reported OOMKilled")

	logs, err := suite.Clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{}).DoRaw(suite.ctx)
	if suite.Assert().NoError(err) {
		suite.Assert().NotContains(string(logs), "FILL_COMPLETED", "victim completed its write despite the kubepods limit")
	}

	suite.AssertNoNewOOMKills(nodeCtx, before, unrelated...)
	suite.AssertNoUserspaceOOMSince(nodeCtx, started)

	suite.Require().NoError(suite.WaitForK8sNodeReadinessStatus(suite.ctx, k8sNode.Name, nodeReady))
	suite.Require().NoError(deletePod(suite.ctx))

	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		_, getErr := suite.Clientset.CoreV1().Pods(namespace).Get(suite.ctx, podName, metav1.GetOptions{})
		assert.True(collect, apierrors.IsNotFound(getErr), "victim pod still present: %v", getErr)
	}, time.Minute, time.Second)

	suite.T().Logf("kubepods on %s OOM-killed the victim %d time(s)", node,
		after.Hierarchical["oom_kill"]-before[constants.CgroupKubepods].Hierarchical["oom_kill"])
}

func init() {
	allSuites = append(allSuites, new(KubeletSuite))
}
