// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8stemplates

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/siderolabs/go-pointer"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// FlannelClusterRoleTemplate returns the template of the ClusterRole
// for the flannel CNI plugin.
func FlannelClusterRoleTemplate() runtime.Object {
	return &rbacv1.ClusterRole{
		TypeMeta: v1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "ClusterRole",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "flannel",
			Labels: map[string]string{
				"k8s-app": "flannel",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"nodes"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"nodes/status"},
				Verbs:     []string{"patch"},
			},
		},
	}
}

// FlannelClusterRoleBindingTemplate returns the template of the
// ClusterRoleBinding for the flannel CNI plugin.
func FlannelClusterRoleBindingTemplate() runtime.Object {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: v1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "flannel",
			Labels: map[string]string{
				"k8s-app": "flannel",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "ClusterRole",
			Name:     "flannel",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "flannel",
				Namespace: "kube-system",
			},
		},
	}
}

// FlannelServiceAccountTemplate returns the template of the
// ServiceAccount for the flannel CNI plugin.
func FlannelServiceAccountTemplate() runtime.Object {
	return &corev1.ServiceAccount{
		TypeMeta: v1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ServiceAccount",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "flannel",
			Namespace: "kube-system",
			Labels: map[string]string{
				"k8s-app": "flannel",
			},
		},
	}
}

// FlannelConfigMapTemplate returns the template of the ConfigMap
// for the flannel CNI plugin.
func FlannelConfigMapTemplate(spec *k8s.BootstrapManifestsConfigSpec) runtime.Object {
	data := map[string]string{
		"cni-conf.json": `{
  "name": "cbr0",
  "cniVersion": "1.0.0",
  "plugins": [
    {
      "type": "flannel",
      "delegate": {
        "hairpinMode": true,
        "isDefaultGateway": true
      }
    },
    {
      "type": "portmap",
      "capabilities": {
        "portMappings": true
      }
    }
  ]
}`,
	}

	var netConf struct {
		Network     string `json:"Network,omitempty"`
		IPv6Network string `json:"IPv6Network,omitempty"`
		EnableIPv6  *bool  `json:"EnableIPv6,omitempty"`
		EnableIPv4  *bool  `json:"EnableIPv4,omitempty"`
		Backend     struct {
			Type string `json:"Type"`
			Port int    `json:"Port"`
		} `json:"Backend"`
	}

	netConf.Backend.Type = "vxlan"
	netConf.Backend.Port = 4789

	hasIPv4 := false

	for _, cidr := range spec.PodCIDRs {
		if strings.Contains(cidr, ".") {
			netConf.Network = cidr
			hasIPv4 = true
		} else {
			netConf.IPv6Network = cidr
			netConf.EnableIPv6 = pointer.To(true)
		}
	}

	if !hasIPv4 {
		netConf.EnableIPv4 = pointer.To(false)
	}

	netConfJSON, err := json.MarshalIndent(netConf, "", "  ")
	if err != nil {
		// should never happen
		panic(fmt.Sprintf("failed to marshal net-conf.json: %s", err))
	}

	data["net-conf.json"] = string(netConfJSON)

	return &corev1.ConfigMap{
		TypeMeta: v1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "kube-flannel-cfg",
			Namespace: "kube-system",
			Labels: map[string]string{
				"k8s-app": "flannel",
				"tier":    "node",
			},
		},
		Data: data,
	}
}

// FlannelDaemonSetTemplate returns the template of the DaemonSet
// for the flannel CNI plugin.
func FlannelDaemonSetTemplate(spec *k8s.BootstrapManifestsConfigSpec) runtime.Object {
	envVars := []corev1.EnvVar{
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "POD_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name:  "EVENT_QUEUE_DEPTH",
			Value: "5000",
		},
		{
			Name:  "CONT_WHEN_CACHE_NOT_READY",
			Value: "false",
		},
	}

	if spec.FlannelKubeServiceHost != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "KUBERNETES_SERVICE_HOST",
			Value: spec.FlannelKubeServiceHost,
		})
	}

	if spec.FlannelKubeServicePort != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "KUBERNETES_SERVICE_PORT",
			Value: spec.FlannelKubeServicePort,
		})
	}

	return &appsv1.DaemonSet{
		TypeMeta: v1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "DaemonSet",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "kube-flannel",
			Namespace: "kube-system",
			Labels: map[string]string{
				"k8s-app": "flannel",
				"tier":    "node",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"k8s-app": "flannel",
					"tier":    "node",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{
						"k8s-app": "flannel",
						"tier":    "node",
					},
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/os",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"linux"},
											},
										},
									},
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:    "kube-flannel",
							Image:   spec.FlannelImage,
							Command: []string{"/opt/bin/flanneld"},
							Args: slices.Concat(
								[]string{
									"--ip-masq",
									"--kube-subnet-mgr",
								},
								spec.FlannelExtraArgs,
							),
							Env: envVars,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{"NET_ADMIN", "NET_RAW"},
								},
								Privileged: pointer.To(false),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "run",
									MountPath: "/run/flannel",
								},
								{
									Name:      "flannel-cfg",
									MountPath: "/etc/kube-flannel/",
								},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:    "install-config",
							Image:   spec.FlannelImage,
							Command: []string{"cp"},
							Args:    []string{"-f", "/etc/kube-flannel/cni-conf.json", "/etc/cni/net.d/10-flannel.conflist"},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "cni", MountPath: "/etc/cni/net.d"},
								{Name: "flannel-cfg", MountPath: "/etc/kube-flannel/"},
							},
						},
					},
					HostNetwork:        true,
					PriorityClassName:  "system-node-critical",
					ServiceAccountName: "flannel",
					Tolerations: []corev1.Toleration{
						{Effect: corev1.TaintEffectNoSchedule, Operator: corev1.TolerationOpExists},
						{Effect: corev1.TaintEffectNoExecute, Operator: corev1.TolerationOpExists},
					},
					Volumes: []corev1.Volume{
						{Name: "run", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/run/flannel"}}},
						{Name: "cni-plugin", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/opt/cni/bin"}}},
						{Name: "cni", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/etc/cni/net.d"}}},
						{Name: "flannel-cfg", VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "kube-flannel-cfg"},
							},
						}},
					},
				},
			},
		},
	}
}
