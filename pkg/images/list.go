// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package images

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Versions holds all the images (and their versions) that are used in Talos.
type Versions struct {
	Etcd    name.Tag
	Flannel name.Tag
	CoreDNS name.Tag

	Kubelet               name.Tag
	KubeAPIServer         name.Tag
	KubeControllerManager name.Tag
	KubeNetworkPolicies   name.Tag
	KubeProxy             name.Tag
	KubeScheduler         name.Tag

	Pause name.Tag
}

// DefaultSandboxImage is defined as a constant in cri package of containerd, and it's not exported.
//
// The integration test verifies that our constant is accurate.
const DefaultSandboxImage = "registry.k8s.io/pause:3.10.1"

// List returns default image versions.
func List(config config.Config) Versions {
	var images Versions

	images.Etcd = mustParseTag(config.Cluster().Etcd().Image())
	images.CoreDNS = mustParseTag(config.Cluster().CoreDNS().Image())
	images.Flannel = mustParseTag(fmt.Sprintf("ghcr.io/siderolabs/flannel:%s", constants.FlannelVersion)) // mirrored from docker.io/flannelcni/flannel
	images.Kubelet = mustParseTag(config.Machine().Kubelet().Image())
	images.KubeAPIServer = mustParseTag(config.Cluster().APIServer().Image())
	images.KubeControllerManager = mustParseTag(config.Cluster().ControllerManager().Image())
	images.KubeNetworkPolicies = mustParseTag(fmt.Sprintf("registry.k8s.io/networking/kube-network-policies:%s", constants.KubeNetworkPoliciesVersion))
	images.KubeProxy = mustParseTag(config.Cluster().Proxy().Image())
	images.KubeScheduler = mustParseTag(config.Cluster().Scheduler().Image())

	images.Pause = mustParseTag(DefaultSandboxImage)

	return images
}

// VersionsListOptions allows overriding the default component versions
// displayed to the user.
//
// Any non-empty field value replaces the corresponding default version
// when presenting available or selected versions. Fields left empty
// will fall back to their built-in defaults.
type VersionsListOptions struct {
	// KubernetesVersion overrides the default Kubernetes version.
	KubernetesVersion string

	// CoreDNSVersion overrides the default CoreDNS version.
	CoreDNSVersion string

	// EtcdVersion overrides the default etcd version.
	EtcdVersion string

	// FlannelVersion overrides the default Flannel version.
	FlannelVersion string

	// PauseVersion overrides the default pause container image version.
	PauseVersion string

	// KubeNetworkPoliciesVersion overrides the default kube-network-policies version.
	KubeNetworkPoliciesVersion string
}

// ListWithOptions returns image versions with overrides.
func ListWithOptions(config config.Config, opts VersionsListOptions) Versions {
	images := List(config)

	if opts.CoreDNSVersion != "" {
		images.CoreDNS = images.CoreDNS.Tag(opts.CoreDNSVersion)
	}

	if opts.EtcdVersion != "" {
		images.Etcd = images.Etcd.Tag(opts.EtcdVersion)
	}

	if opts.FlannelVersion != "" {
		images.Flannel = images.Flannel.Tag(opts.FlannelVersion)
	}

	if opts.PauseVersion != "" {
		images.Pause = images.Pause.Tag(opts.PauseVersion)
	}

	if opts.KubernetesVersion != "" {
		images.Kubelet = images.Kubelet.Tag(opts.KubernetesVersion)
		images.KubeAPIServer = images.KubeAPIServer.Tag(opts.KubernetesVersion)
		images.KubeControllerManager = images.KubeControllerManager.Tag(opts.KubernetesVersion)
		images.KubeProxy = images.KubeProxy.Tag(opts.KubernetesVersion)
		images.KubeScheduler = images.KubeScheduler.Tag(opts.KubernetesVersion)
	}

	if opts.KubeNetworkPoliciesVersion != "" {
		images.KubeNetworkPolicies = images.KubeNetworkPolicies.Tag(opts.KubeNetworkPoliciesVersion)
	}

	return images
}

func mustParseTag(s string) name.Tag {
	r, err := name.ParseReference(s)
	if err != nil {
		panic(err)
	}

	t, ok := r.(name.Tag)
	if !ok {
		panic(fmt.Sprintf("%T is not name.Tag: %#+v", r, r))
	}

	return t
}

func mustParseReferenceWithTag(ref, tag string) name.Tag {
	r, err := name.ParseReference(ref)
	if err != nil {
		panic(err)
	}

	return r.Context().Tag(tag)
}

// TalosBundle holds the core images (and their versions) that are used to build Talos.
type TalosBundle struct {
	Installer     name.Tag
	InstallerBase name.Tag
	Imager        name.Tag
	Talos         name.Tag
	TalosctlAll   name.Tag

	Overlays   name.Tag
	Extensions name.Tag
}

// ListSourcesFor returns source bundle for specific version.
func ListSourcesFor(tag string) TalosBundle {
	var bundle TalosBundle

	bundle.Installer = mustParseReferenceWithTag(DefaultInstallerImageRepository, tag)
	bundle.InstallerBase = mustParseReferenceWithTag(DefaultInstallerBaseImageRepository, tag)
	bundle.Imager = mustParseReferenceWithTag(DefaultImagerImageRepository, tag)
	bundle.Talos = mustParseReferenceWithTag(DefaultTalosImageRepository, tag)
	bundle.TalosctlAll = mustParseReferenceWithTag(DefaultTalosctlAllImageRepository, tag)

	bundle.Overlays = mustParseReferenceWithTag(DefaultOverlaysManifestRepository, tag)
	bundle.Extensions = mustParseReferenceWithTag(DefaultExtensionsManifestRepository, tag)

	return bundle
}
