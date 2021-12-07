// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"fmt"
	"io"

	"github.com/coreos/go-semver/semver"
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
	FromVersion string
	ToVersion   string

	ControlPlaneEndpoint string
	LogOutput            io.Writer
	UpgradeKubelet       bool
	DryRun               bool

	extraUpdaters []daemonsetUpdater
	masterNodes   []string
	workerNodes   []string
}

// Path returns upgrade path in a form "FromMajor.FromMinor->ToMajor.ToMinor" (e.g. "1.20->1.21"),
// or empty string, if one or both versions can't be parsed.
func (options *UpgradeOptions) Path() string {
	from, fromErr := semver.NewVersion(options.FromVersion)
	to, toErr := semver.NewVersion(options.ToVersion)

	if fromErr != nil || toErr != nil {
		return ""
	}

	return fmt.Sprintf("%d.%d->%d.%d", from.Major, from.Minor, to.Major, to.Minor)
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
