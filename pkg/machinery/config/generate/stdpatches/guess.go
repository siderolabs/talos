// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package stdpatches

import (
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
)

// GuessVersionContractKubelet attempts to guess the version contract for kubelet configuration based on the provided machine configuration.
func GuessVersionContractKubelet(cfg config.Container) *config.VersionContract {
	return config.TalosVersionCurrent
}

// GuessVersionContractKubeAPIServer attempts to guess the version contract for kube-apiserver configuration based on the provided machine configuration.
func GuessVersionContractKubeAPIServer(cfg config.Container) *config.VersionContract {
	if hasDocument(k8s.KubeAPIServerConfig, cfg) {
		return config.TalosVersionCurrent
	}

	// the last before multi-doc k8s config
	return config.TalosVersion1_13
}

// GuessVersionContractKubeControllerManager attempts to guess the version contract for kube-controller-manager configuration based on the provided machine configuration.
func GuessVersionContractKubeControllerManager(cfg config.Container) *config.VersionContract {
	if hasDocument(k8s.KubeControllerManagerConfig, cfg) {
		return config.TalosVersionCurrent
	}

	// the last before multi-doc k8s config
	return config.TalosVersion1_13
}

// GuessVersionContractKubeScheduler attempts to guess the version contract for kube-scheduler configuration based on the provided machine configuration.
func GuessVersionContractKubeScheduler(cfg config.Container) *config.VersionContract {
	if hasDocument(k8s.KubeSchedulerConfig, cfg) {
		return config.TalosVersionCurrent
	}

	// the last before multi-doc k8s config
	return config.TalosVersion1_13
}

// GuessVersionContractKubeProxy attempts to guess the version contract for kube-proxy configuration based on the provided machine configuration.
func GuessVersionContractKubeProxy(cfg config.Container) *config.VersionContract {
	if hasDocument(k8s.KubeProxyConfig, cfg) {
		return config.TalosVersionCurrent
	}

	// the last before multi-doc k8s config
	return config.TalosVersion1_13
}

func hasDocument(kind string, cfg config.Container) bool {
	for _, doc := range cfg.Documents() {
		if doc.Kind() == kind {
			return true
		}
	}

	return false
}
