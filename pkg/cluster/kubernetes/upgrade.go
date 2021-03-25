// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"fmt"

	"github.com/coreos/go-semver/semver"
	appsv1 "k8s.io/api/apps/v1"
)

const (
	namespace                 = "kube-system"
	checkpointerAnnotation    = "checkpointer.alpha.coreos.com/checkpoint"
	checkpointedPodAnnotation = "checkpointer.alpha.coreos.com/checkpoint-of"

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

	extraUpdaters                []daemonsetUpdater
	podCheckpointerExtraUpdaters []daemonsetUpdater
	masterNodes                  []string
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

type daemonsetUpdater func(ds string, daemonset *appsv1.DaemonSet) error
