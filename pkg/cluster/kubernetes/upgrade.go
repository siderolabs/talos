// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"fmt"
	"io"

	"github.com/siderolabs/go-kubernetes/kubernetes/upgrade"

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

	ControlPlaneEndpoint string
	LogOutput            io.Writer
	PrePullImages        bool
	UpgradeKubelet       bool
	DryRun               bool
	EncoderOpt           encoder.Option

	KubeletImage           string
	APIServerImage         string
	ControllerManagerImage string
	SchedulerImage         string
	ProxyImage             string

	controlPlaneNodes []string
	workerNodes       []string
}

// Log writes the line to logger or to stdout if no logger was provided.
func (options *UpgradeOptions) Log(line string, args ...any) {
	if options.LogOutput != nil {
		options.LogOutput.Write([]byte(fmt.Sprintf(line, args...))) //nolint:errcheck

		return
	}

	fmt.Printf(line+"\n", args...)
}
