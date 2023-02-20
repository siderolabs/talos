// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"fmt"
	"io"

	"github.com/siderolabs/go-kubernetes/kubernetes/upgrade"
	appsv1 "k8s.io/api/apps/v1"
)

const (
	namespace = "kube-system"

	kubeAPIServer         = "kube-apiserver"
	kubeControllerManager = "kube-controller-manager"
	kubeScheduler         = "kube-scheduler"
	kubeProxy             = "kube-proxy"
)

// UpgradeOptions represents Kubernetes control plane upgrade settings.
type UpgradeOptions struct {
	Path *upgrade.Path

	ControlPlaneEndpoint string
	LogOutput            io.Writer
	UpgradeKubelet       bool
	DryRun               bool

	extraUpdaters     []daemonsetUpdater
	controlPlaneNodes []string
	workerNodes       []string
}

// Log writes the line to logger or to stdout if no logger was provided.
func (options *UpgradeOptions) Log(line string, args ...interface{}) {
	if options.LogOutput != nil {
		options.LogOutput.Write([]byte(fmt.Sprintf(line, args...))) //nolint:errcheck

		return
	}

	fmt.Printf(line+"\n", args...)
}

type daemonsetUpdater func(ds string, daemonset *appsv1.DaemonSet) error
