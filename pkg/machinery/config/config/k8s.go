// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

// K8sSchedulerConfig defines configuration options for the kube-scheduler static pod.
type K8sSchedulerConfig interface {
	K8sSchedulerConfigSignal()
	Enabled() bool
	Image() string
	ExtraArgs() map[string][]string
	ExtraVolumes() []VolumeMount
	Env() Env
	Resources() Resources
	Config() map[string]any
}

// K8sEtcdEncryptionConfig defines the interface to access Kubernetes API server encryption of secret data at rest configuration.
type K8sEtcdEncryptionConfig interface {
	// EtcdEncryptionConfig returns the exact contents of the configuration file, excluding the apiVersion and kind fields.
	EtcdEncryptionConfig() map[string]any
}
