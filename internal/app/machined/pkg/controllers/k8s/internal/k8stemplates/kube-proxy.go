// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8stemplates

import (
	"fmt"
	"slices"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/yaml"

	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

const (
	// kubeProxyConfigMapNamePrefix is the name prefix of the ConfigMap holding the kube-proxy configuration file.
	//
	// The actual ConfigMap name carries a content-based checksum suffix (see kubeProxyConfigMapName) so that a
	// configuration change produces a new ConfigMap rather than mutating the existing one. This changes the
	// kube-proxy DaemonSet pod template (the referenced ConfigMap volume name), so the DaemonSet is rolled and
	// every pod template revision always mounts the configuration it was rendered with, regardless of apply order.
	// Stale ConfigMaps are pruned by the manifest sync (server-side apply).
	kubeProxyConfigMapNamePrefix = "kube-proxy-config"
	// kubeProxyConfigChecksumLen is the number of hex characters of the configuration checksum used as the
	// ConfigMap name suffix; it keeps the name short while remaining collision-resistant for this use.
	kubeProxyConfigChecksumLen = 16
	// kubeProxyConfigFileName is the key/file name of the kube-proxy configuration inside the ConfigMap.
	kubeProxyConfigFileName = "config.conf"
	// kubeProxyConfigMountDir is the directory the kube-proxy config ConfigMap is mounted at.
	//
	// It must match the `--config` flag value set in the control plane controller.
	kubeProxyConfigMountDir = "/var/lib/kube-proxy"
)

// kubeProxyConfigMapName returns the ConfigMap name for the kube-proxy configuration with the given checksum,
// appending a content-based suffix so a configuration change rolls over to a freshly named ConfigMap.
func kubeProxyConfigMapName(checksum string) string {
	return kubeProxyConfigMapNamePrefix + "-" + checksum[:min(len(checksum), kubeProxyConfigChecksumLen)]
}

// KubeProxyConfigMapTemplate renders the kube-proxy configuration (spec.ProxyConfig) as a ConfigMap
// which is mounted into the kube-proxy DaemonSet.
func KubeProxyConfigMapTemplate(spec *k8s.BootstrapManifestsConfigSpec) (runtime.Object, error) {
	configYAML, err := yaml.Marshal(spec.ProxyConfig)
	if err != nil {
		return nil, fmt.Errorf("error marshaling kube-proxy configuration: %w", err)
	}

	return &corev1.ConfigMap{
		TypeMeta: v1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      kubeProxyConfigMapName(spec.ProxyConfigChecksum),
			Namespace: "kube-system",
			Labels: map[string]string{
				"tier":    "node",
				"k8s-app": "kube-proxy",
			},
		},
		Data: map[string]string{
			kubeProxyConfigFileName: string(configYAML),
		},
	}, nil
}

// KubeProxyDaemonSetTemplate generates a DaemonSet for kube-proxy.
func KubeProxyDaemonSetTemplate(spec *k8s.BootstrapManifestsConfigSpec) (runtime.Object, error) {
	resources, err := Resources(spec.ProxyResources, "100m", "50Mi")
	if err != nil {
		return nil, fmt.Errorf("invalid kube-proxy resource requirements: %w", err)
	}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "lib-modules",
			MountPath: "/lib/modules",
			ReadOnly:  true,
		},
		{
			Name:      "ssl-certs-host",
			MountPath: "/etc/ssl/certs",
			ReadOnly:  true,
		},
		{
			Name:      "kubeconfig",
			MountPath: "/etc/kubernetes",
			ReadOnly:  true,
		},
	}

	volumes := []corev1.Volume{
		{
			Name: "lib-modules",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/usr/lib/modules",
				},
			},
		},
		{
			Name: "ssl-certs-host",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/ssl/certs",
				},
			},
		},
		{
			Name: "kubeconfig",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "kubeconfig-in-cluster",
					},
				},
			},
		},
	}

	if spec.ProxyConfig != nil {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "config",
			MountPath: kubeProxyConfigMountDir,
			ReadOnly:  true,
		})

		volumes = append(volumes, corev1.Volume{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					// The name carries a content-based checksum suffix, so a config change references a
					// new ConfigMap and rolls the DaemonSet, guaranteeing each revision mounts its own config.
					LocalObjectReference: corev1.LocalObjectReference{
						Name: kubeProxyConfigMapName(spec.ProxyConfigChecksum),
					},
				},
			},
		})
	}

	proxyContainer := corev1.Container{
		Name:  "kube-proxy",
		Image: spec.ProxyImage,
		Command: slices.Concat(
			[]string{"/usr/local/bin/kube-proxy"},
			spec.ProxyArgs,
		),
		Env: []corev1.EnvVar{
			{
				Name: "NODE_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "spec.nodeName",
					},
				},
			},
			{
				Name: "POD_IP",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "status.podIP",
					},
				},
			},
		},
		Resources: resources,
		SecurityContext: &corev1.SecurityContext{
			Privileged: new(true),
		},
		VolumeMounts: volumeMounts,
	}

	if gcEnv := GoGCEnvFromResources(resources); gcEnv.Name != "" {
		proxyContainer.Env = append(proxyContainer.Env, gcEnv)
	}

	return &appsv1.DaemonSet{
		TypeMeta: v1.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: appsv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "kube-proxy",
			Namespace: "kube-system",
			Labels: map[string]string{
				"tier":    "node",
				"k8s-app": "kube-proxy",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"tier":    "node",
					"k8s-app": "kube-proxy",
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: new(intstr.FromInt(1)),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{
						"tier":    "node",
						"k8s-app": "kube-proxy",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						proxyContainer,
					},
					HostNetwork:        true,
					PriorityClassName:  "system-cluster-critical",
					ServiceAccountName: "kube-proxy",
					Tolerations: []corev1.Toleration{
						{
							Effect:   corev1.TaintEffectNoSchedule,
							Operator: corev1.TolerationOpExists,
						},
						{
							Effect:   corev1.TaintEffectNoExecute,
							Operator: corev1.TolerationOpExists,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}, nil
}

// KubeProxyServiceAccount returns the ServiceAccount for kube-proxy.
func KubeProxyServiceAccount() runtime.Object {
	return &corev1.ServiceAccount{
		TypeMeta: v1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "kube-proxy",
			Namespace: "kube-system",
		},
	}
}

// KubeProxyClusterRoleBinding returns the ClusterRoleBinding for kube-proxy.
func KubeProxyClusterRoleBinding() runtime.Object {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: v1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "kube-proxy",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "kube-proxy",
				Namespace: "kube-system",
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "system:node-proxier",
			APIGroup: rbacv1.GroupName,
		},
	}
}
