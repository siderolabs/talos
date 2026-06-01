// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8stemplates

import (
	"slices"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// KubeProxyDaemonSetTemplate generates a DaemonSet for kube-proxy.
func KubeProxyDaemonSetTemplate(spec *k8s.BootstrapManifestsConfigSpec) runtime.Object {
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
						{
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
							SecurityContext: &corev1.SecurityContext{
								Privileged: new(true),
							},
							VolumeMounts: []corev1.VolumeMount{
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
							},
						},
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
					Volumes: []corev1.Volume{
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
					},
				},
			},
		},
	}
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
