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
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
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

func init() {
	allSuites = append(allSuites, new(KubeletSuite))
}
