// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"fmt"
	"io"
	"time"

	"github.com/siderolabs/go-kubernetes/kubernetes/upgrade"
	"sigs.k8s.io/cli-utils/pkg/inventory"

	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
)

const (
	namespace = "kube-system"

	kubeAPIServer         = "kube-apiserver"
	kubeControllerManager = "kube-controller-manager"
	kubeScheduler         = "kube-scheduler"
)

// UpgradeOptions represents Kubernetes control plane upgrade settings.
type UpgradeOptions struct {
	Path *upgrade.Path

	DryRun               bool
	ControlPlaneEndpoint string
	LogOutput            io.Writer
	PrePullImages        bool
	UpgradeKubelet       bool
	EncoderOpt           encoder.Option

	KubeletImage           string
	APIServerImage         string
	ControllerManagerImage string
	SchedulerImage         string
	ProxyImage             string

	ForceConflicts   bool
	NoPrune          bool
	PruneTimeout     time.Duration
	ReconcileTimeout time.Duration
	InventoryPolicy  inventory.Policy

	controlPlaneNodes []string
	workerNodes       []string
}

// Log writes the line to logger or to stdout if no logger was provided.
func (options *UpgradeOptions) Log(line string, args ...any) {
	if options.LogOutput != nil {
		fmt.Fprintf(options.LogOutput, line, args...) //nolint:errcheck

		return
	}

	fmt.Printf(line+"\n", args...)
}
