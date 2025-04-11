// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"io"
	"time"

	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-retry/retry"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/talos/internal/integration/base"
)

// ExtensionsSuiteNVIDIA verifies Talos is securebooted.
type ExtensionsSuiteNVIDIA struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *ExtensionsSuiteNVIDIA) SuiteName() string {
	return "api.ExtensionsSuiteNVIDIA"
}

// SetupTest ...
func (suite *ExtensionsSuiteNVIDIA) SetupTest() {
	if !suite.ExtensionsNvidia {
		suite.T().Skip("skipping as nvidia extensions test are not enabled")
	}

	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 5*time.Minute)
}

// TearDownTest ...
func (suite *ExtensionsSuiteNVIDIA) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestExtensionsNVIDIA verifies that a cuda workload can be run.
//
//nolint:gocyclo
func (suite *ExtensionsSuiteNVIDIA) TestExtensionsNVIDIA() {
	expectedModulesModDep := map[string]string{
		"nvidia":         "nvidia.ko",
		"nvidia_uvm":     "nvidia-uvm.ko",
		"nvidia_drm":     "nvidia-drm.ko",
		"nvidia_modeset": "nvidia-modeset.ko",
	}

	// if we're testing NVIDIA stuff we need to get the nodes having NVIDIA GPUs
	// we query k8s to get the nodes having the label node.kubernetes.io/instance-type.
	// this label is set by the cloud provider and it's value is the instance type.
	// the nvidia e2e-aws tests creates gpu nodes one with g4dn.xlarge and another
	// with p4d.24xlarge
	for _, nvidiaNode := range suite.getNVIDIANodes("node.kubernetes.io/instance-type in (g4dn.xlarge, p4d.24xlarge)") {
		suite.AssertExpectedModules(suite.ctx, nvidiaNode, expectedModulesModDep)
	}

	nodes := suite.getNVIDIANodes("node.kubernetes.io/instance-type=g4dn.xlarge")
	for _, node := range nodes {
		suite.AssertServicesRunning(suite.ctx, node, map[string]string{
			"ext-nvidia-persistenced": "Running",
		})
	}

	// nodes = suite.getNVIDIANodes("node.kubernetes.io/instance-type=p4d.24xlarge")
	// for _, node := range nodes {
	// 	suite.testServicesRunning(node, map[string]string{
	// 		"ext-nvidia-persistenced":  "Running",
	// 		"ext-nvidia-fabricmanager": "Running",
	// 	})
	// }

	_, err := suite.Clientset.NodeV1().RuntimeClasses().Create(suite.ctx, &nodev1.RuntimeClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nvidia",
		},
		Handler: "nvidia",
	}, metav1.CreateOptions{})
	defer suite.Clientset.NodeV1().RuntimeClasses().Delete(suite.ctx, "nvidia", metav1.DeleteOptions{}) //nolint:errcheck

	suite.Require().NoError(err)

	_, err = suite.Clientset.AppsV1().DaemonSets("kube-system").Create(suite.ctx, nvidiaDevicePluginDaemonSetSpec(), metav1.CreateOptions{})
	defer suite.Clientset.AppsV1().DaemonSets("kube-system").Delete(suite.ctx, "nvidia-device-plugin", metav1.DeleteOptions{}) //nolint:errcheck

	suite.Require().NoError(err)

	// now we can create a cuda test job
	_, err = suite.Clientset.BatchV1().Jobs("default").Create(suite.ctx, nvidiaCUDATestJob("nvcr.io/nvidia/k8s/cuda-sample:vectoradd-cuda11.7.1"), metav1.CreateOptions{})
	defer suite.Clientset.BatchV1().Jobs("default").Delete(suite.ctx, "cuda-test", metav1.DeleteOptions{}) //nolint:errcheck

	suite.Require().NoError(err)

	// delete all pods with label app.kubernetes.io/name=cuda-test
	defer func() {
		podList, listErr := suite.GetPodsWithLabel(suite.ctx, "default", "app.kubernetes.io/name=cuda-test")
		if listErr != nil {
			err = listErr
		}

		for _, pod := range podList.Items {
			err = suite.Clientset.CoreV1().Pods("default").Delete(suite.ctx, pod.Name, metav1.DeleteOptions{})
		}
	}()

	// wait for the pods to be completed
	suite.Require().NoError(retry.Constant(4*time.Minute, retry.WithUnits(time.Second*10)).Retry(
		func() error {
			podList, listErr := suite.GetPodsWithLabel(suite.ctx, "default", "app.kubernetes.io/name=cuda-test")
			if listErr != nil {
				return retry.ExpectedErrorf("error getting pod: %s", listErr)
			}

			for _, pod := range podList.Items {
				if pod.Status.Phase == corev1.PodFailed {
					logData := suite.getPodLogs("default", pod.Name)

					suite.T().Logf("pod %s logs:\n%s", pod.Name, logData)
				}
			}

			if len(podList.Items) != 1 {
				return retry.ExpectedErrorf("expected 1 pod, got %d", len(podList.Items))
			}

			for _, pod := range podList.Items {
				if pod.Status.Phase != corev1.PodSucceeded {
					return retry.ExpectedErrorf("%s is not completed yet: %s", pod.Name, pod.Status.Phase)
				}
			}

			return nil
		},
	))

	// now we can check the logs
	podList, err := suite.GetPodsWithLabel(suite.ctx, "default", "app.kubernetes.io/name=cuda-test")
	suite.Require().NoError(err)

	suite.Require().Len(podList.Items, 1)

	for _, pod := range podList.Items {
		logData := suite.getPodLogs("default", pod.Name)

		suite.Require().Contains(logData, "Test PASSED")
	}
}

func (suite *ExtensionsSuiteNVIDIA) getPodLogs(namespace, name string) string {
	res := suite.Clientset.CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{})
	stream, err := res.Stream(suite.ctx)
	suite.Require().NoError(err)

	defer stream.Close() //nolint:errcheck

	logData, err := io.ReadAll(stream)
	suite.Require().NoError(err)

	return string(logData)
}

func (suite *ExtensionsSuiteNVIDIA) getNVIDIANodes(labelQuery string) []string {
	nodes, err := suite.Clientset.CoreV1().Nodes().List(suite.ctx, metav1.ListOptions{
		LabelSelector: labelQuery,
	})
	suite.Require().NoError(err)

	// if we don't have any node with NVIDIA GPUs we fail the test
	// since we explicitly asked for them
	suite.Require().NotEmpty(nodes.Items, "no nodes with NVIDIA GPUs matching label selector '%s' found", labelQuery)

	nodeList := make([]string, len(nodes.Items))

	for i, node := range nodes.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				nodeList[i] = addr.Address
			}
		}
	}

	return nodeList
}

func nvidiaDevicePluginDaemonSetSpec() *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nvidia-device-plugin",
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": "nvidia-device-plugin",
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name": "nvidia-device-plugin",
					},
				},
				Spec: corev1.PodSpec{
					PriorityClassName: "system-node-critical",
					RuntimeClassName:  pointer.To("nvidia"),
					Containers: []corev1.Container{
						{
							Name:  "nvidia-device-plugin-ctr",
							Image: "nvcr.io/nvidia/k8s-device-plugin:v0.14.1",
							Env: []corev1.EnvVar{
								{
									Name:  "NVIDIA_MIG_MONITOR_DEVICES",
									Value: "all",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{"SYS_ADMIN"},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "device-plugin",
									MountPath: "/var/lib/kubelet/device-plugins",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "device-plugin",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/kubelet/device-plugins",
								},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      "CriticalAddonsOnly",
							Operator: corev1.TolerationOpExists,
						},
						{
							Effect:   corev1.TaintEffectNoSchedule,
							Key:      "nvidia.com/gpu",
							Operator: corev1.TolerationOpExists,
						},
					},
				},
			},
		},
	}
}

func nvidiaCUDATestJob(image string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cuda-test",
		},
		Spec: batchv1.JobSpec{
			Completions: pointer.To[int32](1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cuda-test",
					Labels: map[string]string{
						"app.kubernetes.io/name": "cuda-test",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "cuda-test",
							Image: image,
						},
					},
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "node.kubernetes.io/instance-type",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"g4dn.xlarge", "p4d.24xlarge"},
											},
										},
									},
								},
							},
						},
					},
					RestartPolicy:    corev1.RestartPolicyNever,
					RuntimeClassName: pointer.To("nvidia"),
				},
			},
		},
	}
}

func init() {
	allSuites = append(allSuites, &ExtensionsSuiteNVIDIA{})
}
