// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"strings"
	"time"

	"github.com/siderolabs/go-pointer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/talos/internal/integration/base"
)

// CommonSuite verifies some default settings such as ulimits.
type CommonSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *CommonSuite) SuiteName() string {
	return "api.CommonSuite"
}

// SetupTest ...
func (suite *CommonSuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 10*time.Minute)
}

// TearDownTest ...
func (suite *CommonSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestVirtioModulesLoaded verifies that the virtio modules are loaded.
func (suite *CommonSuite) TestVirtioModulesLoaded() {
	if suite.Cluster == nil || suite.Cluster.Provisioner() != "qemu" {
		suite.T().Skip("skipping virtio test since provisioner is not qemu")
	}

	expectedVirtIOModules := map[string]string{
		"virtio_balloon":        "virtio_balloon.ko",
		"virtio_pci":            "virtio_pci.ko",
		"virtio_pci_legacy_dev": "virtio_pci_legacy_dev.ko",
		"virtio_pci_modern_dev": "virtio_pci_modern_dev.ko",
	}

	node := suite.RandomDiscoveredNodeInternalIP()
	suite.AssertExpectedModules(suite.ctx, node, expectedVirtIOModules)
}

// TestCommonDefaults verifies that the default ulimits are set.
func (suite *CommonSuite) TestCommonDefaults() {
	if suite.Cluster != nil && suite.Cluster.Provisioner() == "docker" {
		suite.T().Skip("skipping ulimits test since provisioner is docker")
	}

	expectedUlimit := `
core file size (blocks)         (-c) 0
data seg size (kb)              (-d) unlimited
scheduling priority             (-e) 0
file size (blocks)              (-f) unlimited
max locked memory (kb)          (-l) 8192
max memory size (kb)            (-m) unlimited
open files                      (-n) 1048576
POSIX message queues (bytes)    (-q) 819200
real-time priority              (-r) 0
stack size (kb)                 (-s) 8192
cpu time (seconds)              (-t) unlimited
virtual memory (kb)             (-v) unlimited
file locks                      (-x) unlimited
`

	const (
		namespace = "default"
		pod       = "defaults-test"
	)

	_, err := suite.Clientset.CoreV1().Pods(namespace).Create(suite.ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: pod,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  pod,
					Image: "alpine",
					Command: []string{
						"tail",
						"-f",
						"/dev/null",
					},
				},
			},
			TerminationGracePeriodSeconds: pointer.To[int64](0),
		},
	}, metav1.CreateOptions{})

	suite.Require().NoError(err)

	defer suite.Clientset.CoreV1().Pods(namespace).Delete(suite.ctx, pod, metav1.DeleteOptions{}) //nolint:errcheck

	// wait for the pod to be ready
	suite.Require().NoError(suite.WaitForPodToBeRunning(suite.ctx, 10*time.Minute, namespace, pod))

	stdout, stderr, err := suite.ExecuteCommandInPod(suite.ctx, namespace, pod, "ulimit -c -d -e -f -l -m -n -q -r -s -t -v -x")
	suite.Require().NoError(err)

	suite.Require().Equal("", stderr)
	suite.Require().Equal(strings.TrimPrefix(expectedUlimit, "\n"), stdout)
}

// TestDNSResolver verifies that external DNS resolving works from a pod.
func (suite *CommonSuite) TestDNSResolver() {
	if suite.Cluster != nil {
		// cluster should be healthy for kube-dns resolving to work
		suite.AssertClusterHealthy(suite.ctx)
	}

	const (
		namespace = "default"
		pod       = "dns-test"
	)

	_, err := suite.Clientset.CoreV1().Pods(namespace).Create(suite.ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: pod,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  pod,
					Image: "alpine",
					Command: []string{
						"tail",
						"-f",
						"/dev/null",
					},
				},
			},
			TerminationGracePeriodSeconds: pointer.To[int64](0),
		},
	}, metav1.CreateOptions{})

	suite.Require().NoError(err)

	suite.T().Cleanup(func() {
		cleanUpCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()

		suite.Require().NoError(
			suite.Clientset.CoreV1().Pods(namespace).Delete(cleanUpCtx, pod, metav1.DeleteOptions{}),
		)
	})

	// wait for the pod to be ready
	suite.Require().NoError(suite.WaitForPodToBeRunning(suite.ctx, time.Minute, namespace, pod))

	stdout, stderr, err := suite.ExecuteCommandInPod(suite.ctx, namespace, pod, "wget -S https://www.google.com/")
	suite.Assert().NoError(err)
	suite.Assert().Equal("", stdout)
	suite.Assert().Contains(stderr, "'index.html' saved")

	if suite.T().Failed() {
		suite.LogPodLogsByLabel(suite.ctx, "kube-system", "k8s-app", "kube-dns")

		for _, node := range suite.DiscoverNodeInternalIPs(suite.ctx) {
			suite.DumpLogs(suite.ctx, node, "dns-resolve-cache", "google")
		}

		suite.T().FailNow()
	}

	_, stderr, err = suite.ExecuteCommandInPod(suite.ctx, namespace, pod, "apk add --update bind-tools")

	suite.Assert().NoError(err)
	suite.Assert().Empty(stderr, "stderr: %s", stderr)

	if suite.T().Failed() {
		suite.T().FailNow()
	}

	stdout, stderr, err = suite.ExecuteCommandInPod(suite.ctx, namespace, pod, "dig really-long-record.dev.siderolabs.io")

	suite.Assert().NoError(err)
	suite.Assert().Contains(stdout, "status: NOERROR")
	suite.Assert().Contains(stdout, "ANSWER: 34")
	suite.Assert().NotContains(stdout, "status: NXDOMAIN")
	suite.Assert().Equal(stderr, "")

	if suite.T().Failed() {
		suite.T().FailNow()
	}
}

func init() {
	allSuites = append(allSuites, &CommonSuite{})
}
