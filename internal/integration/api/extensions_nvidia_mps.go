// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/siderolabs/go-retry/retry"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/siderolabs/talos/pkg/machinery/client"
)

//go:embed testdata/nvidia-mps-device-plugin-config.yaml
var nvidiaMPSDevicePluginConfig string

const (
	nvidiaGPUOperatorNamespace = "gpu-operator"
	nvidiaClusterPolicyName    = "cluster-policy"
	nvidiaMPSConfigMapName     = "talos-nvidia-mps"
	nvidiaMPSConfigName        = "mps"
	nvidiaMPSClients           = 2
	nvidiaMPSClientNamespace   = "default"
	nvidiaMPSRunLabel          = "talos.dev/nvidia-mps-run"

	nvidiaDevicePluginDaemon = "nvidia-device-plugin-daemonset"
	nvidiaDevicePluginCtr    = "nvidia-device-plugin"

	// The MPS control daemon mounts the host MPS root (/run/nvidia/mps) at /mps, with a pipe directory per resource.
	nvidiaMPSPipeDirectory = "/mps/nvidia.com/gpu/pipe"
	nvidiaMPSControlDaemon = "nvidia-device-plugin-mps-control-daemon"
	nvidiaMPSControlCtr    = "mps-control-daemon-ctr"
)

var nvidiaClusterPolicyGVR = schema.GroupVersionResource{
	Group:    "nvidia.com",
	Version:  "v1",
	Resource: "clusterpolicies",
}

// podUIDInCgroupRegexp matches the pod UID in both cgroupfs (pod<uid>) and systemd (-pod<uid_with_underscores>.slice) kubepods paths.
var podUIDInCgroupRegexp = regexp.MustCompile(`pod([0-9a-f]{8}[-_][0-9a-f]{4}[-_][0-9a-f]{4}[-_][0-9a-f]{4}[-_][0-9a-f]{12})`)

type mpsClientPod struct {
	name string
	uid  types.UID
}

// testCUDAMPS switches the GPU Operator device plugin to CUDA MPS sharing and verifies that
// two pods sharing a single GPU are both connected as clients to the same MPS server.
//
// The device plugin config lives in the cluster-wide ClusterPolicy, so every GPU node running the
// device plugin is switched to MPS for the duration of the test; all of them must have MIG disabled.
// It expects the GPU Operator to be already installed by TestExtensionsNVIDIA.
func (suite *ExtensionsSuiteNVIDIA) testCUDAMPS() {
	if !suite.ExtensionsNvidiaMPS {
		suite.T().Skip("skipping as nvidia MPS test is not enabled")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	suite.T().Cleanup(cancel)

	suite.requireMIGDisabled(ctx)
	suite.enableMPSSharing(ctx)
	suite.waitForMPSControlDaemon(ctx)

	node := suite.waitForMPSNode(ctx)
	nodeIP := nodeInternalIP(node)
	hostname := node.Labels[corev1.LabelHostname]

	suite.Require().NotEmpty(nodeIP, "node %s has no internal IP", node.Name)
	suite.Require().NotEmpty(hostname, "node %s has no %s label", node.Name, corev1.LabelHostname)

	daemonPod := suite.mpsControlDaemonPod(ctx, node.Name)
	pods := suite.launchMPSClients(ctx, hostname)

	suite.Require().NoError(retry.Constant(5*time.Minute, retry.WithUnits(10*time.Second)).RetryWithContext(ctx, func(ctx context.Context) error {
		if err := suite.assertMPSClientsRunning(ctx, pods); err != nil {
			return err
		}

		return suite.findMPSServerWithClients(client.WithNode(ctx, nodeIP), daemonPod, pods)
	}))
}

// requireMIGDisabled fails before any cluster mutation if a GPU served by the device plugin has MIG enabled, as MPS does not support MIG.
func (suite *ExtensionsSuiteNVIDIA) requireMIGDisabled(ctx context.Context) {
	pods, err := suite.GetPodsWithLabel(ctx, nvidiaGPUOperatorNamespace, "app="+nvidiaDevicePluginDaemon)
	suite.Require().NoError(err)
	suite.Require().NotEmpty(pods.Items, "no NVIDIA device plugin pods found")

	for _, pod := range pods.Items {
		suite.Require().Equal(corev1.PodRunning, pod.Status.Phase, "device plugin pod %s is not running", pod.Name)

		stdout, stderr, err := suite.ExecInPod(ctx, pod.Namespace, pod.Name, nvidiaDevicePluginCtr,
			"nvidia-smi --query-gpu=mig.mode.current --format=csv,noheader")
		suite.Require().NoError(err, "querying MIG mode on node %s failed, stderr: %s", pod.Spec.NodeName, stderr)

		enabled, err := parseMIGModeEnabled(stdout)
		suite.Require().NoError(err, "node %s", pod.Spec.NodeName)
		suite.Require().False(enabled, "node %s has MIG enabled, which is not supported by the MPS test", pod.Spec.NodeName)
	}
}

// enableMPSSharing points the ClusterPolicy device plugin config at an owned ConfigMap with an MPS sharing config,
// restoring the previous device plugin config and removing the ConfigMap on cleanup.
func (suite *ExtensionsSuiteNVIDIA) enableMPSSharing(ctx context.Context) {
	t := suite.T()

	configMaps := suite.Clientset.CoreV1().ConfigMaps(nvidiaGPUOperatorNamespace)

	configMap, err := configMaps.Create(ctx, &corev1.ConfigMap{
		Name: nvidiaMPSConfigMapName,
		Data: map[string]string{
			nvidiaMPSConfigName: nvidiaMPSDevicePluginConfig,
		},
	}, metav1.CreateOptions{})
	suite.Require().NoError(err)

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()

		if err := configMaps.Delete(cleanupCtx, configMap.Name, metav1.DeleteOptions{
			Preconditions: metav1.NewUIDPreconditions(string(configMap.UID)),
		}); err != nil && !apierrors.IsNotFound(err) {
			t.Errorf("failed to delete MPS config map: %v", err)
		}
	})

	policies := suite.DynamicClient.Resource(nvidiaClusterPolicyGVR)

	policy, err := policies.Get(ctx, nvidiaClusterPolicyName, metav1.GetOptions{})
	suite.Require().NoError(err)

	previousConfig, hadConfig, err := unstructured.NestedFieldCopy(policy.Object, "spec", "devicePlugin", "config")
	suite.Require().NoError(err)

	// the test operation makes the patch fail if the config changed since the snapshot was taken
	var ops []map[string]any

	if hadConfig {
		ops = append(ops, map[string]any{"op": "test", "path": "/spec/devicePlugin/config", "value": previousConfig})
	}

	ops = append(ops, map[string]any{
		"op": "add", "path": "/spec/devicePlugin/config", "value": map[string]string{"name": configMap.Name, "default": nvidiaMPSConfigName},
	})

	patch, err := json.Marshal(ops)
	suite.Require().NoError(err)

	_, err = policies.Patch(ctx, nvidiaClusterPolicyName, types.JSONPatchType, patch, metav1.PatchOptions{})
	suite.Require().NoError(err)

	restore := map[string]any{"op": "remove", "path": "/spec/devicePlugin/config"}
	if hadConfig {
		restore = map[string]any{"op": "replace", "path": "/spec/devicePlugin/config", "value": previousConfig}
	}

	restorePatch, err := json.Marshal([]map[string]any{restore})
	suite.Require().NoError(err)

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()

		if _, err := policies.Patch(cleanupCtx, nvidiaClusterPolicyName, types.JSONPatchType, restorePatch, metav1.PatchOptions{}); err != nil {
			t.Errorf("failed to restore ClusterPolicy device plugin config: %v", err)
		}
	})
}

func (suite *ExtensionsSuiteNVIDIA) waitForMPSControlDaemon(ctx context.Context) {
	suite.Require().NoError(retry.Constant(10*time.Minute, retry.WithUnits(10*time.Second)).RetryWithContext(ctx, func(ctx context.Context) error {
		ds, err := suite.Clientset.AppsV1().DaemonSets(nvidiaGPUOperatorNamespace).Get(ctx, nvidiaMPSControlDaemon, metav1.GetOptions{})
		if err != nil {
			return retry.ExpectedError(err)
		}

		if ds.Status.DesiredNumberScheduled == 0 || ds.Status.NumberReady != ds.Status.DesiredNumberScheduled {
			return retry.ExpectedErrorf("MPS control daemon has %d/%d pods ready", ds.Status.NumberReady, ds.Status.DesiredNumberScheduled)
		}

		return nil
	}))
}

// waitForMPSNode returns a node which advertises enough MPS replicas for all clients.
func (suite *ExtensionsSuiteNVIDIA) waitForMPSNode(ctx context.Context) *corev1.Node {
	var node *corev1.Node

	suite.Require().NoError(retry.Constant(10*time.Minute, retry.WithUnits(10*time.Second)).RetryWithContext(ctx, func(ctx context.Context) error {
		nodes, err := suite.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("nvidia.com/gpu.sharing-strategy=mps,nvidia.com/gpu.replicas=%d", nvidiaMPSClients),
		})
		if err != nil {
			return retry.ExpectedError(err)
		}

		idx := slices.IndexFunc(nodes.Items, func(n corev1.Node) bool {
			allocatable := n.Status.Allocatable[corev1.ResourceName("nvidia.com/gpu")]

			return allocatable.Value() >= nvidiaMPSClients
		})
		if idx == -1 {
			return retry.ExpectedErrorf("no node advertises %d MPS replicas of nvidia.com/gpu yet", nvidiaMPSClients)
		}

		node = &nodes.Items[idx]

		return nil
	}))

	suite.T().Logf("running MPS clients on node %s", node.Name)

	return node
}

// launchMPSClients creates the client pods and waits for them to run; cleanup deletes only the created pods and waits for them to be gone,
// and is registered after the sharing config cleanup, so it runs before the device plugin config is restored.
func (suite *ExtensionsSuiteNVIDIA) launchMPSClients(ctx context.Context, hostname string) []mpsClientPod {
	t := suite.T()
	runID := uuid.NewString()
	podsClient := suite.Clientset.CoreV1().Pods(nvidiaMPSClientNamespace)

	pods := make([]mpsClientPod, 0, nvidiaMPSClients)

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cleanupCancel()

		for _, pod := range pods {
			if err := suite.deleteOwnedPod(cleanupCtx, pod); err != nil {
				t.Errorf("failed to delete MPS client pod %s: %v", pod.name, err)
			}
		}
	})

	for range nvidiaMPSClients {
		created, err := podsClient.Create(ctx, nvidiaMPSClientPod(runID, hostname), metav1.CreateOptions{})
		suite.Require().NoError(err)

		pods = append(pods, mpsClientPod{name: created.Name, uid: created.UID})
	}

	for _, pod := range pods {
		suite.Require().NoError(suite.WaitForPodToBeRunning(ctx, 10*time.Minute, nvidiaMPSClientNamespace, pod.name))
	}

	return pods
}

func (suite *ExtensionsSuiteNVIDIA) deleteOwnedPod(ctx context.Context, pod mpsClientPod) error {
	podsClient := suite.Clientset.CoreV1().Pods(nvidiaMPSClientNamespace)

	err := podsClient.Delete(ctx, pod.name, metav1.DeleteOptions{
		GracePeriodSeconds: new(int64(0)),
		Preconditions:      metav1.NewUIDPreconditions(string(pod.uid)),
	})
	if apierrors.IsNotFound(err) || apierrors.IsConflict(err) {
		return nil
	}

	if err != nil {
		return err
	}

	return retry.Constant(2*time.Minute, retry.WithUnits(2*time.Second)).RetryWithContext(ctx, func(ctx context.Context) error {
		current, err := podsClient.Get(ctx, pod.name, metav1.GetOptions{})

		switch {
		case apierrors.IsNotFound(err):
			return nil
		case err != nil:
			return retry.ExpectedError(err)
		case current.UID != pod.uid:
			return nil
		default:
			return retry.ExpectedErrorf("pod %s is still present", pod.name)
		}
	})
}

// assertMPSClientsRunning fails without retrying if an owned client pod is gone or not running, as a finished client never reconnects to MPS.
func (suite *ExtensionsSuiteNVIDIA) assertMPSClientsRunning(ctx context.Context, pods []mpsClientPod) error {
	for _, owned := range pods {
		pod, err := suite.Clientset.CoreV1().Pods(nvidiaMPSClientNamespace).Get(ctx, owned.name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("MPS client %s: %w", owned.name, err)
		}

		if pod.UID != owned.uid {
			return fmt.Errorf("MPS client %s was replaced (UID %s, expected %s)", owned.name, pod.UID, owned.uid)
		}

		if pod.Status.Phase != corev1.PodRunning {
			logs, logErr := suite.podLogs(ctx, pod.Namespace, pod.Name)
			if logErr != nil {
				logs = fmt.Sprintf("failed to get logs: %v", logErr)
			}

			return fmt.Errorf("MPS client %s is %s, logs:\n%s", pod.Name, pod.Status.Phase, logs)
		}
	}

	return nil
}

// findMPSServerWithClients succeeds if a single MPS server has client processes from every owned pod.
//
// Client PIDs are host PIDs (the control daemon runs with hostPID), attributed to pods via their /proc/<pid>/cgroup read from the node.
func (suite *ExtensionsSuiteNVIDIA) findMPSServerWithClients(nodeCtx context.Context, daemonPod string, pods []mpsClientPod) error {
	servers, err := suite.mpsControl(nodeCtx, daemonPod, "get_server_list")
	if err != nil {
		return retry.ExpectedError(err)
	}

	for _, server := range servers {
		clients, err := suite.mpsControl(nodeCtx, daemonPod, fmt.Sprintf("get_client_list %d", server))
		if err != nil {
			return retry.ExpectedError(err)
		}

		connected := map[types.UID]int{}

		for _, clientPID := range clients {
			cgroup, err := suite.readNodeFile(nodeCtx, fmt.Sprintf("/proc/%d/cgroup", clientPID))
			if err != nil {
				suite.T().Logf("failed to read cgroup of MPS client %d: %v", clientPID, err)

				continue
			}

			if uid, ok := parsePodUIDFromCgroup(cgroup); ok {
				connected[uid] = clientPID
			}
		}

		suite.T().Logf("MPS server %d has client PIDs %v, attributed to pods %v", server, clients, connected)

		if allPodsConnected(pods, connected) {
			return nil
		}
	}

	return retry.ExpectedErrorf("no MPS server has clients from all %d test pods (servers: %v)", len(pods), servers)
}

func allPodsConnected(pods []mpsClientPod, connected map[types.UID]int) bool {
	for _, pod := range pods {
		if _, ok := connected[pod.uid]; !ok {
			return false
		}
	}

	return len(pods) > 0
}

func (suite *ExtensionsSuiteNVIDIA) mpsControlDaemonPod(ctx context.Context, nodeName string) string {
	pods, err := suite.GetPodsWithLabel(ctx, nvidiaGPUOperatorNamespace, "app="+nvidiaMPSControlDaemon)
	suite.Require().NoError(err)

	idx := slices.IndexFunc(pods.Items, func(pod corev1.Pod) bool { return pod.Spec.NodeName == nodeName })
	suite.Require().NotEqual(-1, idx, "no MPS control daemon pod on node %s", nodeName)

	return pods.Items[idx].Name
}

// mpsControl runs an nvidia-cuda-mps-control command in the MPS control daemon container and parses the PID list it prints.
func (suite *ExtensionsSuiteNVIDIA) mpsControl(ctx context.Context, daemonPod, command string) ([]int, error) {
	stdout, stderr, err := suite.ExecInPod(ctx, nvidiaGPUOperatorNamespace, daemonPod, nvidiaMPSControlCtr,
		fmt.Sprintf("echo %q | CUDA_MPS_PIPE_DIRECTORY=%s nvidia-cuda-mps-control", command, nvidiaMPSPipeDirectory),
	)
	if err != nil {
		return nil, fmt.Errorf("%q failed: %w, stderr: %s", command, err, stderr)
	}

	return parseMPSPIDList(stdout)
}

func (suite *ExtensionsSuiteNVIDIA) readNodeFile(ctx context.Context, path string) (string, error) {
	reader, err := suite.Client.Read(ctx, path)
	if err != nil {
		return "", err
	}

	defer reader.Close() //nolint:errcheck

	data, err := io.ReadAll(reader)

	return string(data), err
}

func (suite *ExtensionsSuiteNVIDIA) podLogs(ctx context.Context, namespace, name string) (string, error) {
	stream, err := suite.Clientset.CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{}).Stream(ctx)
	if err != nil {
		return "", err
	}

	defer stream.Close() //nolint:errcheck

	data, err := io.ReadAll(stream)

	return string(data), err
}

// parseMPSPIDList parses the whitespace-separated PID list printed by get_server_list and get_client_list.
func parseMPSPIDList(output string) ([]int, error) {
	fields := strings.Fields(output)
	pids := make([]int, 0, len(fields))

	for _, field := range fields {
		pid, err := strconv.Atoi(field)
		if err != nil || pid <= 0 {
			return nil, fmt.Errorf("unexpected nvidia-cuda-mps-control output %q", output)
		}

		pids = append(pids, pid)
	}

	return pids, nil
}

// parsePodUIDFromCgroup extracts the pod UID from /proc/<pid>/cgroup contents; it reports false unless exactly one pod UID is found.
func parsePodUIDFromCgroup(contents string) (types.UID, bool) {
	var found types.UID

	for _, match := range podUIDInCgroupRegexp.FindAllStringSubmatch(contents, -1) {
		uid := types.UID(strings.ReplaceAll(match[1], "_", "-"))

		if found != "" && found != uid {
			return "", false
		}

		found = uid
	}

	return found, found != ""
}

// parseMIGModeEnabled parses `nvidia-smi --query-gpu=mig.mode.current --format=csv,noheader` output (one line per GPU).
func parseMIGModeEnabled(output string) (bool, error) {
	var gpus int

	for line := range strings.Lines(output) {
		switch mode := strings.TrimSpace(line); mode {
		case "":
			continue
		case "Enabled":
			return true, nil
		case "Disabled", "[N/A]":
			gpus++
		default:
			return false, fmt.Errorf("unexpected MIG mode %q", mode)
		}
	}

	if gpus == 0 {
		return false, errors.New("no GPUs reported by nvidia-smi")
	}

	return false, nil
}

func nodeInternalIP(node *corev1.Node) string {
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address
		}
	}

	return ""
}

// nvidiaMPSClientPod runs the CUDA n-body benchmark long enough to stay connected to the MPS server while the test inspects it.
func nvidiaMPSClientPod(runID, hostname string) *corev1.Pod {
	return &corev1.Pod{
		GenerateName: "cuda-mps-client-",
		Labels: map[string]string{
			"app.kubernetes.io/name": "cuda-mps-client",
			nvidiaMPSRunLabel:        runID,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "cuda-mps-client",
					Image: fmt.Sprintf("nvcr.io/nvidia/k8s/cuda-sample:%s", NvidiaCUDANBodyImageVersion),
					Args:  []string{"-benchmark", "-numbodies=16384", "-i=1000000"},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceName("nvidia.com/gpu"): resource.MustParse("1"),
						},
					},
				},
			},
			NodeSelector: map[string]string{
				corev1.LabelHostname: hostname,
			},
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: new(int64(0)),
		},
	}
}
