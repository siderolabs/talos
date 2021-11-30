// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package images

import (
	"fmt"

	criconfig "github.com/containerd/cri/pkg/config"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/version"
)

// Versions holds all the images (and their versions) that are used in Talos.
type Versions struct {
	Etcd       string
	Flannel    string
	FlannelCNI string
	CoreDNS    string

	Kubelet               string
	KubeAPIServer         string
	KubeControllerManager string
	KubeProxy             string
	KubeScheduler         string

	Installer string

	Pause string
}

// List returns default image versions.
func List(config config.Provider) Versions {
	var images Versions

	images.Etcd = config.Cluster().Etcd().Image()
	images.CoreDNS = config.Cluster().CoreDNS().Image()
	images.Flannel = "quay.io/coreos/flannel:v0.15.1"
	images.FlannelCNI = fmt.Sprintf("ghcr.io/talos-systems/install-cni:%s", version.ExtrasVersion)
	images.Kubelet = config.Machine().Kubelet().Image()
	images.KubeAPIServer = config.Cluster().APIServer().Image()
	images.KubeControllerManager = config.Cluster().ControllerManager().Image()
	images.KubeProxy = config.Cluster().Proxy().Image()
	images.KubeScheduler = config.Cluster().Scheduler().Image()

	images.Installer = DefaultInstallerImage

	images.Pause = criconfig.DefaultConfig().SandboxImage

	return images
}
