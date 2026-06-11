// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package stdpatches contains standard patches applied to Talos machine configurations.
package stdpatches

import (
	"fmt"

	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/config"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/security"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// WithStaticHostname returns a patch that sets a static hostname in the machine configuration.
func WithStaticHostname(versionContract *config.VersionContract, hostname string) ([]byte, error) {
	if versionContract.MultidocNetworkConfigSupported() {
		hostnameConfig := network.NewHostnameConfigV1Alpha1()
		hostnameConfig.ConfigAuto = new(nethelpers.AutoHostnameKindOff)
		hostnameConfig.ConfigHostname = hostname

		return patchFromDocument(hostnameConfig)
	}

	return patchFromV1Alpha1(map[string]any{
		"machine": map[string]any{
			"network": map[string]any{
				"hostname": hostname,
			},
		},
	})
}

// WithTrustedRoots returns a patch that sets trusted roots in the machine configuration.
func WithTrustedRoots(versionContract *config.VersionContract, trustedRoots string) ([]byte, error) {
	if versionContract.MultidocNetworkConfigSupported() {
		trustedRootsConfig := security.NewTrustedRootsConfigV1Alpha1()
		trustedRootsConfig.Certificates = trustedRoots

		return patchFromDocument(trustedRootsConfig)
	}

	return nil, fmt.Errorf("trusted roots patch is not supported for version contract %s", versionContract.String())
}

// WithKubeletImage returns a patch that updates the kubelet image in the machine configuration.
func WithKubeletImage(versionContract *config.VersionContract, kubeletImage string) ([]byte, error) {
	return patchFromV1Alpha1(map[string]any{
		"machine": map[string]any{
			"kubelet": map[string]any{
				"image": kubeletImage,
			},
		},
	})
}

// WithKubeAPIServerImage returns a patch that updates the kube-apiserver image in the machine configuration.
func WithKubeAPIServerImage(versionContract *config.VersionContract, kubeAPIServerImage string) ([]byte, error) {
	return patchFromV1Alpha1(map[string]any{
		"cluster": map[string]any{
			"apiServer": map[string]any{
				"image": kubeAPIServerImage,
			},
		},
	})
}

// WithKubeControllerManagerImage returns a patch that updates the kube-controller-manager image in the machine configuration.
func WithKubeControllerManagerImage(versionContract *config.VersionContract, kubeControllerManagerImage string) ([]byte, error) {
	if versionContract.MultidocKubernetesConfigSupported() {
		controllerManagerConfig := k8s.NewKubeControllerManagerConfigV1Alpha1()
		controllerManagerConfig.PodImage = kubeControllerManagerImage

		return patchFromDocument(controllerManagerConfig)
	}

	return patchFromV1Alpha1(map[string]any{
		"cluster": map[string]any{
			"controllerManager": map[string]any{
				"image": kubeControllerManagerImage,
			},
		},
	})
}

// WithKubeSchedulerImage returns a patch that updates the kube-scheduler image in the machine configuration.
func WithKubeSchedulerImage(versionContract *config.VersionContract, kubeSchedulerImage string) ([]byte, error) {
	if versionContract.MultidocKubernetesConfigSupported() {
		schedulerConfig := k8s.NewKubeSchedulerConfigV1Alpha1()
		schedulerConfig.PodImage = kubeSchedulerImage

		return patchFromDocument(schedulerConfig)
	}

	return patchFromV1Alpha1(map[string]any{
		"cluster": map[string]any{
			"scheduler": map[string]any{
				"image": kubeSchedulerImage,
			},
		},
	})
}

// WithKubeProxyImage returns a patch that updates the kube-proxy image in the machine configuration.
func WithKubeProxyImage(versionContract *config.VersionContract, kubeProxyImage string) ([]byte, error) {
	if versionContract.MultidocKubernetesConfigSupported() {
		proxyConfig := k8s.NewKubeProxyConfigV1Alpha1()
		proxyConfig.ProxyImage = kubeProxyImage

		return patchFromDocument(proxyConfig)
	}

	return patchFromV1Alpha1(map[string]any{
		"cluster": map[string]any{
			"proxy": map[string]any{
				"image": kubeProxyImage,
			},
		},
	})
}

func patchFromDocument(doc configconfig.Document) ([]byte, error) {
	ctr, err := container.New(doc)
	if err != nil {
		return nil, err
	}

	return ctr.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
}

func patchFromV1Alpha1(doc any) ([]byte, error) {
	return yaml.Marshal(doc)
}
