// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"crypto/rand"
	"fmt"
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

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// KubeletSuite verifies that projected volumes still receive updates
// after the kubelet service is restarted.
//
// Regression test for https://github.com/siderolabs/talos/issues/13352.
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
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":     podName,
				"version": "v1",
			},
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
					VolumeSource: corev1.VolumeSource{
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
