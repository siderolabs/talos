// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package nodedrain provides reusable Kubernetes node drain, cordon, and uncordon
// operations for use by talosctl commands (upgrade, reboot).
package nodedrain

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/go-kubernetes/kubernetes/nodedrain"
	"k8s.io/client-go/kubernetes"

	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/reporter"
)

const (
	// DefaultDrainTimeout is the default maximum time to wait for the node to be drained.
	DefaultDrainTimeout = 5 * time.Minute
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
	nodenameRes, err := safe.StateGetByID[*k8s.Nodename](
		ctx,
		c.COSI,
		k8s.NodenameID,
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
	report(reporter.Update{
		Message: fmt.Sprintf("%s: cordoning node", nodeName),
		Status:  reporter.StatusRunning,
	})

	if err := nodedrain.Cordon(ctx, clientset, nodeName); err != nil {
		report(reporter.Update{
			Message: fmt.Sprintf("%s: error cordoning node: %v", nodeName, err),
			Status:  reporter.StatusError,
		})

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

	dopts := nodedrain.DrainOptions{
		Timeout: opts.DrainTimeout,
		Progress: func(s string) {
			report(reporter.Update{
				Message: fmt.Sprintf("%s: %s", nodeName, s),
				Status:  reporter.StatusRunning,
			})
		},
	}

	if err := nodedrain.Drain(ctx, clientset, nodeName, dopts); err != nil {
		report(reporter.Update{
			Message: fmt.Sprintf("%s: error cordoning node: %v", nodeName, err),
			Status:  reporter.StatusError,
		})

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
	return nodedrain.WaitForNodeReady(ctx, clientset, nodeName, timeout)
}

// Uncordon marks the Kubernetes node as schedulable again.
func Uncordon(ctx context.Context, clientset kubernetes.Interface, nodeName string, report ReportFunc) error {
	report(reporter.Update{
		Message: fmt.Sprintf("%s: uncordoning node", nodeName),
		Status:  reporter.StatusRunning,
	})

	if err := nodedrain.Uncordon(ctx, clientset, nodeName); err != nil {
		report(reporter.Update{
			Message: fmt.Sprintf("%s: error uncordoning node: %v", nodeName, err),
			Status:  reporter.StatusError,
		})

		return fmt.Errorf("error uncordoning node %q: %w", nodeName, err)
	}

	report(reporter.Update{
		Message: fmt.Sprintf("%s: node uncordoned", nodeName),
		Status:  reporter.StatusSucceeded,
	})

	return nil
}
