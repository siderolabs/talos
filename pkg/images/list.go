// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package images

import (
	"fmt"

	criconfig "github.com/containerd/containerd/pkg/cri/config"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/version"
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
func List(config config.Config) Versions {
	var images Versions

	images.Etcd = config.Cluster().Etcd().Image()
	images.CoreDNS = config.Cluster().CoreDNS().Image()
	images.Flannel = fmt.Sprintf("ghcr.io/siderolabs/flannel:%s", constants.FlannelVersion) // mirrored from docker.io/flannelcni/flannel
	images.FlannelCNI = fmt.Sprintf("ghcr.io/siderolabs/install-cni:%s", version.ExtrasVersion)
	images.Kubelet = config.Machine().Kubelet().Image()
	images.KubeAPIServer = config.Cluster().APIServer().Image()
	images.KubeControllerManager = config.Cluster().ControllerManager().Image()
	images.KubeProxy = config.Cluster().Proxy().Image()
	images.KubeScheduler = config.Cluster().Scheduler().Image()

	images.Installer = DefaultInstallerImage

	images.Pause = criconfig.DefaultConfig().SandboxImage

	return images
}
