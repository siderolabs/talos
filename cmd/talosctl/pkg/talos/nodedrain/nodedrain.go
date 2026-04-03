// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package nodedrain provides reusable Kubernetes node drain, cordon, and uncordon
// operations for use by talosctl commands (upgrade, reboot).
package nodedrain

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/drain"

	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/reporter"
)

const (
	// DefaultDrainTimeout is the default maximum time to wait for the node to be drained.
	DefaultDrainTimeout = 5 * time.Minute

	// nodeReadyPollInterval is how often to poll for node readiness.
	nodeReadyPollInterval = 5 * time.Second
)

// ReportFunc is a callback for reporting drain progress updates.
// It decouples drain operations from the reporter, allowing callers to route
// updates through a channel (for multi-node aggregation) or directly to a reporter.
type ReportFunc func(reporter.Update)

// Options configures the drain behavior.
type Options struct {
	// DrainTimeout is the maximum time to wait for pod evictions to complete.
	DrainTimeout time.Duration
}

// GetKubernetesNodeName resolves the Kubernetes node name from a Talos node
// by reading the Nodename COSI resource.
//
// The context must target a single node (via client.WithNode) because COSI
// State/Get does not support one-to-many proxying.
func GetKubernetesNodeName(ctx context.Context, c *client.Client) (string, error) {
	nodenameRes, err := safe.StateGet[*k8s.Nodename](
		ctx,
		c.COSI,
		resource.NewMetadata(k8s.NamespaceName, k8s.NodenameType, k8s.NodenameID, resource.VersionUndefined),
	)
	if err != nil {
		return "", fmt.Errorf("error getting Kubernetes node name from Talos API: %w", err)
	}

	return nodenameRes.TypedSpec().Nodename, nil
}

// CordonAndDrain cordons the Kubernetes node (marks it unschedulable) and evicts
// all pods. It uses the kubectl drain library for proper PDB handling, eviction
// API support, and pod filtering.
func CordonAndDrain(ctx context.Context, clientset kubernetes.Interface, nodeName string, opts Options, report ReportFunc) error {
	timeout := opts.DrainTimeout
	if timeout == 0 {
		timeout = DefaultDrainTimeout
	}

	drainCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	node, err := clientset.CoreV1().Nodes().Get(drainCtx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting node %q: %w", nodeName, err)
	}

	// Set up the drain helper with sensible defaults covering 90% of cases.
	drainer := &drain.Helper{
		Ctx:                 drainCtx,
		Client:              clientset,
		Force:               true, // handle unmanaged pods
		GracePeriodSeconds:  -1,   // use pod's own terminationGracePeriodSeconds
		IgnoreAllDaemonSets: true, // DaemonSet pods are re-created by the DS controller
		DeleteEmptyDirData:  true, // node is rebooting, local data is lost anyway
		Timeout:             timeout,
		Out:                 io.Discard,
		ErrOut:              io.Discard,
		OnPodDeletionOrEvictionStarted: func(pod *corev1.Pod, usingEviction bool) {
			action := "deleting"
			if usingEviction {
				action = "evicting"
			}

			report(reporter.Update{
				Message: fmt.Sprintf("%s: %s pod %s/%s", nodeName, action, pod.Namespace, pod.Name),
				Status:  reporter.StatusRunning,
			})
		},
		OnPodDeletionOrEvictionFinished: func(pod *corev1.Pod, usingEviction bool, err error) {
			if err != nil {
				report(reporter.Update{
					Message: fmt.Sprintf("%s: failed to evict pod %s/%s: %v", nodeName, pod.Namespace, pod.Name, err),
					Status:  reporter.StatusError,
				})

				return
			}

			report(reporter.Update{
				Message: fmt.Sprintf("%s: evicted pod %s/%s", nodeName, pod.Namespace, pod.Name),
				Status:  reporter.StatusRunning,
			})
		},
	}

	report(reporter.Update{
		Message: fmt.Sprintf("%s: cordoning node", nodeName),
		Status:  reporter.StatusRunning,
	})

	if err := drain.RunCordonOrUncordon(drainer, node, true); err != nil {
		return fmt.Errorf("error cordoning node %q: %w", nodeName, err)
	}

	report(reporter.Update{
		Message: fmt.Sprintf("%s: node cordoned", nodeName),
		Status:  reporter.StatusRunning,
	})

	report(reporter.Update{
		Message: fmt.Sprintf("%s: draining node", nodeName),
		Status:  reporter.StatusRunning,
	})

	if err := drain.RunNodeDrain(drainer, nodeName); err != nil {
		return fmt.Errorf("error draining node %q: %w", nodeName, err)
	}

	report(reporter.Update{
		Message: fmt.Sprintf("%s: node drained", nodeName),
		Status:  reporter.StatusSucceeded,
	})

	return nil
}

// WaitForNodeReady polls the Kubernetes API until the node reports a Ready condition
// with status True.
func WaitForNodeReady(ctx context.Context, clientset kubernetes.Interface, nodeName string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, nodeReadyPollInterval, timeout, true, func(ctx context.Context) (bool, error) {
		node, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			// Transient errors are expected while the node is rebooting.
			return false, nil //nolint:nilerr
		}

		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady {
				return cond.Status == corev1.ConditionTrue, nil
			}
		}

		return false, nil
	})
}

// Uncordon marks the Kubernetes node as schedulable again.
func Uncordon(ctx context.Context, clientset kubernetes.Interface, nodeName string, report ReportFunc) error {
	node, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting node %q for uncordon: %w", nodeName, err)
	}

	drainer := &drain.Helper{
		Ctx:    ctx,
		Client: clientset,
	}

	report(reporter.Update{
		Message: fmt.Sprintf("%s: uncordoning node", nodeName),
		Status:  reporter.StatusRunning,
	})

	if err := drain.RunCordonOrUncordon(drainer, node, false); err != nil {
		return fmt.Errorf("error uncordoning node %q: %w", nodeName, err)
	}

	report(reporter.Update{
		Message: fmt.Sprintf("%s: node uncordoned", nodeName),
		Status:  reporter.StatusSucceeded,
	})

	return nil
}
