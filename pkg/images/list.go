// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package images

import (
	"fmt"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Versions holds all the images (and their versions) that are used in Talos.
type Versions struct {
	Etcd    string
	Flannel string
	CoreDNS string

	Kubelet               string
	KubeAPIServer         string
	KubeControllerManager string
	KubeProxy             string
	KubeScheduler         string

	Installer string
	Talos     string

	Pause string
}

// DefaultSandboxImage is defined as a constant in cri package of containerd, and it's not exported.
//
// The integration test verifies that our constant is accurate.
const DefaultSandboxImage = "registry.k8s.io/pause:3.10"

// List returns default image versions.
func List(config config.Config) Versions {
	var images Versions

	images.Etcd = config.Cluster().Etcd().Image()
	images.CoreDNS = config.Cluster().CoreDNS().Image()
	images.Flannel = fmt.Sprintf("ghcr.io/siderolabs/flannel:%s", constants.FlannelVersion) // mirrored from docker.io/flannelcni/flannel
	images.Kubelet = config.Machine().Kubelet().Image()
	images.KubeAPIServer = config.Cluster().APIServer().Image()
	images.KubeControllerManager = config.Cluster().ControllerManager().Image()
	images.KubeProxy = config.Cluster().Proxy().Image()
	images.KubeScheduler = config.Cluster().Scheduler().Image()

	images.Installer = DefaultInstallerImage
	images.Talos = DefaultTalosImage

	images.Pause = DefaultSandboxImage

	return images
}
