// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
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
}

type daemonsetUpdater func(ds string, daemonset *appsv1.DaemonSet) error
