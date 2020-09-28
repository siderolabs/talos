// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package images

import (
	"fmt"
	"runtime"

	"github.com/talos-systems/bootkube-plugin/pkg/asset"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/version"
)

// List returns a list of images used.
func List(config config.Provider) asset.ImageVersions {
	images := asset.DefaultImages

	// Override all kube-related images with default val or specified image locations
	images.Flannel = fmt.Sprintf("quay.io/coreos/flannel:v0.12.0-%s", runtime.GOARCH)
	images.FlannelCNI = fmt.Sprintf("ghcr.io/talos-systems/install-cni:%s", version.PkgsVersion)
	images.Kubelet = config.Machine().Kubelet().Image()
	images.KubeAPIServer = config.Cluster().APIServer().Image()
	images.KubeControllerManager = config.Cluster().ControllerManager().Image()
	images.KubeProxy = config.Cluster().Proxy().Image()
	images.KubeScheduler = config.Cluster().Scheduler().Image()
	images.Etcd = config.Cluster().Etcd().Image()

	// Allow for overriding by users via config data
	images.CoreDNS = config.Cluster().CoreDNS().Image()
	images.PodCheckpointer = config.Cluster().PodCheckpointer().Image()

	return images
}
