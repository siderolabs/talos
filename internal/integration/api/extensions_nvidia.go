// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/go-retry/retry"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

//go:embed testdata/nvidia-gpu-operator.yaml
var nvidiaGPUOperatorHelmChartValues []byte

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
//nolint:gocyclo,cyclop,dupl
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
	for _, nvidiaNode := range suite.getNVIDIANodes("node.kubernetes.io/instance-type in (g4dn.xlarge, p4d.24xlarge, g5g.xlarge)") {
		suite.AssertExpectedModules(suite.ctx, nvidiaNode, expectedModulesModDep)
	}

	nodes := suite.getNVIDIANodes("node.kubernetes.io/instance-type in (g4dn.xlarge, p4d.24xlarge, g5g.xlarge)")
	for _, node := range nodes {
		suite.AssertServicesRunning(suite.ctx, node, map[string]string{
			"ext-nvidia-persistenced": "Running",
			"ext-nvidia-cdi-gen":      "Finished",
		})
	}

	missingCDIFilesData := map[string]map[string]int{
		"amd64": {
			"nvidia-open-gpu-kernel-modules-production": 13,
			"nvidia-open-gpu-kernel-modules-lts":        9,
			"nonfree-kmod-nvidia-production":            13,
			"nonfree-kmod-nvidia-lts":                   9,
		},
		"arm64": {
			"nvidia-open-gpu-kernel-modules-production": 11,
			"nvidia-open-gpu-kernel-modules-lts":        9,
			"nonfree-kmod-nvidia-production":            11,
			"nonfree-kmod-nvidia-lts":                   9,
		},
	}

	for _, node := range nodes {
		nodeCtx := client.WithNode(suite.ctx, node)

		versionInfo, err := suite.Client.Version(nodeCtx)
		suite.Require().NoError(err)

		suite.Require().NotNil(versionInfo.GetMessages(), "version info messages should not be nil")

		extInfo := missingCDIFilesData[versionInfo.GetMessages()[0].Version.Arch]

		list, err := safe.StateListAll[*runtime.ExtensionStatus](nodeCtx, suite.Client.COSI)
		suite.Require().NoError(err)

		extensionsList := safe.ToSlice(list, func(info *runtime.ExtensionStatus) string {
			return info.TypedSpec().Metadata.Name
		})

		var expectedCount int

		for _, name := range extensionsList {
			if count, exists := extInfo[name]; exists {
				expectedCount = count

				break
			}
		}

		suite.Require().NotZero(expectedCount, "did not find any matching nvidia extension in the list of extensions: %v", extensionsList)

		logsStream, err := suite.Client.Logs(
			nodeCtx,
			constants.SystemContainerdNamespace,
			common.ContainerDriver_CONTAINERD,
			"ext-nvidia-cdi-gen",
			false,
			-1,
		)
		suite.Require().NoError(err)

		logReader, err := client.ReadStream(logsStream)
		suite.Require().NoError(err)

		defer logReader.Close() //nolint:errcheck

		var buffer bytes.Buffer

		_, err = io.Copy(&buffer, logReader)
		suite.Require().NoError(err)

		logData := buffer.String()

		// we know as baseline we have different number of missing files that are not present in the extension
		// and manually verified, if some new files are not found we want to fix the extension
		// Adding an example of the current log message for reference:
		// ❯ talosctl -n 172.16.15.116 logs ext-nvidia-cdi-gen | grep "Could not"
		// msg="Could not locate libnvidia-vulkan-producer.so.580.126.20: libnvidia-vulkan-producer.so.580.126.20: not found\nlibnvidia-vulkan-producer.so.580.126.20: not found"
		// msg="Could not locate X11/xorg.conf.d/10-nvidia.conf: X11/xorg.conf.d/10-nvidia.conf: not found"
		// msg="Could not locate X11/xorg.conf.d/nvidia-drm-outputclass.conf: X11/xorg.conf.d/nvidia-drm-outputclass.conf: not found"
		// msg="Could not locate vulkan/implicit_layer.d/nvidia_layers.json: vulkan/implicit_layer.d/nvidia_layers.json: not found\nvulkan/implicit_layer.d/nvidia_layers.json: not found"
		// msg="Could not locate vulkan/icd.d/nvidia_icd.x86_64.json: vulkan/icd.d/nvidia_icd.x86_64.json: not found\nvulkan/icd.d/nvidia_icd.x86_64.json: not found"
		// msg="Could not locate /nvidia-fabricmanager/socket: /nvidia-fabricmanager/socket: not found"
		// msg="Could not locate /tmp/nvidia-mps: /tmp/nvidia-mps: not found"
		// msg="Could not locate nvidia-imex: nvidia-imex: not found"
		// msg="Could not locate nvidia-imex-ctl: nvidia-imex-ctl: not found"
		suite.Assert().Equal(
			expectedCount,
			strings.Count(logData, "Could not locate"),
			"expected exactly %d 'Could not locate' in the logs, got %d. Logs:\n%s",
			expectedCount,
			strings.Count(logData, "Could not"),
			logData,
		)
	}

	// nodes = suite.getNVIDIANodes("node.kubernetes.io/instance-type=p4d.24xlarge")
	// for _, node := range nodes {
	// 	suite.testServicesRunning(node, map[string]string{
	// 		"ext-nvidia-persistenced":  "Running",
	// 		"ext-nvidia-fabricmanager": "Running",
	// 	})
	// }

	_, err := suite.Clientset.CoreV1().Namespaces().Create(suite.ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gpu-operator",
			Labels: map[string]string{
				"pod-security.kubernetes.io/enforce": "privileged",
			},
		},
	}, metav1.CreateOptions{})
	defer suite.Clientset.CoreV1().Namespaces().Delete(suite.ctx, "gpu-operator", metav1.DeleteOptions{}) //nolint:errcheck

	suite.Require().NoError(err)

	suite.Require().NoError(suite.HelmInstall(
		suite.ctx,
		"gpu-operator",
		"https://helm.ngc.nvidia.com/nvidia",
		NvidiaGPUOperatorChartVersion,
		"gpu-operator",
		"gpu-operator",
		nvidiaGPUOperatorHelmChartValues,
	))

	suite.Run("CUDA test", func() {
		// now we can create a cuda test job
		_, err = suite.Clientset.BatchV1().Jobs("default").Create(suite.ctx, nvidiaCUDATestJob(), metav1.CreateOptions{})
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
	})

	suite.Run("CUDA CDI test", func() {
		// test CDI code path by requesting nvidia.com/gpu resource limits
		_, err = suite.Clientset.BatchV1().Jobs("default").Create(suite.ctx, nvidiaCDITestJob(), metav1.CreateOptions{})
		defer suite.Clientset.BatchV1().Jobs("default").Delete(suite.ctx, "cuda-cdi-test", metav1.DeleteOptions{}) //nolint:errcheck

		suite.Require().NoError(err)

		defer func() {
			cdiPodList, listErr := suite.GetPodsWithLabel(suite.ctx, "default", "app.kubernetes.io/name=cuda-cdi-test")
			if listErr != nil {
				err = listErr
			}

			for _, pod := range cdiPodList.Items {
				err = suite.Clientset.CoreV1().Pods("default").Delete(suite.ctx, pod.Name, metav1.DeleteOptions{})
			}
		}()

		suite.Require().NoError(retry.Constant(4*time.Minute, retry.WithUnits(time.Second*10)).Retry(
			func() error {
				cdiPodList, listErr := suite.GetPodsWithLabel(suite.ctx, "default", "app.kubernetes.io/name=cuda-cdi-test")
				if listErr != nil {
					return retry.ExpectedErrorf("error getting pod: %s", listErr)
				}

				for _, pod := range cdiPodList.Items {
					if pod.Status.Phase == corev1.PodFailed {
						logData := suite.getPodLogs("default", pod.Name)

						suite.T().Logf("pod %s logs:\n%s", pod.Name, logData)
					}
				}

				if len(cdiPodList.Items) != 1 {
					return retry.ExpectedErrorf("expected 1 pod, got %d", len(cdiPodList.Items))
				}

				for _, pod := range cdiPodList.Items {
					if pod.Status.Phase != corev1.PodSucceeded {
						return retry.ExpectedErrorf("%s is not completed yet: %s", pod.Name, pod.Status.Phase)
					}
				}

				return nil
			},
		))

		cdiPodList, err := suite.GetPodsWithLabel(suite.ctx, "default", "app.kubernetes.io/name=cuda-cdi-test")
		suite.Require().NoError(err)

		suite.Require().Len(cdiPodList.Items, 1)

		for _, pod := range cdiPodList.Items {
			logData := suite.getPodLogs("default", pod.Name)

			suite.Require().Contains(logData, "Test PASSED")
		}
	})
}

func (suite *ExtensionsSuiteNVIDIA) getPodLogs(namespace, name string) string { //nolint:unparam
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

func nvidiaCUDATestJob() *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cuda-test",
		},
		Spec: batchv1.JobSpec{
			Completions: new(int32(1)),
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
							Image: fmt.Sprintf("nvcr.io/nvidia/k8s/cuda-sample:%s", NvidiaCUDATestImageVersion),
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
												Values:   []string{"g4dn.xlarge", "p4d.24xlarge", "g5g.xlarge"},
											},
										},
									},
								},
							},
						},
					},
					RestartPolicy:    corev1.RestartPolicyNever,
					RuntimeClassName: new("nvidia"),
				},
			},
		},
	}
}

// nvidiaCDITestJob creates a job that requests nvidia.com/gpu resource limits,
// exercising the CDI code path (as opposed to runtimeClassName alone).
func nvidiaCDITestJob() *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cuda-cdi-test",
		},
		Spec: batchv1.JobSpec{
			Completions: new(int32(1)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cuda-cdi-test",
					Labels: map[string]string{
						"app.kubernetes.io/name": "cuda-cdi-test",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "cuda-cdi-test",
							Image: fmt.Sprintf("nvcr.io/nvidia/k8s/cuda-sample:%s", NvidiaCUDATestImageVersion),
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceName("nvidia.com/gpu"): resource.MustParse("1"),
								},
							},
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
												Values:   []string{"g4dn.xlarge", "p4d.24xlarge", "g5g.xlarge"},
											},
										},
									},
								},
							},
						},
					},
					RestartPolicy:    corev1.RestartPolicyNever,
					RuntimeClassName: new("nvidia"),
				},
			},
		},
	}
}

func init() {
	allSuites = append(allSuites, &ExtensionsSuiteNVIDIA{})
}
