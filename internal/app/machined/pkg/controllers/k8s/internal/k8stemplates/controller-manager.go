// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8stemplates

import (
	"strings"

	"github.com/siderolabs/go-kubernetes/kubernetes/compatibility"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// ControllerManagerPod builds a static pod for the kube-controller-manager based on the config.
func ControllerManagerPod(configResource *k8s.ControllerManagerConfig, secretsVersion string) (runtime.Object, error) {
	cfg := configResource.TypedSpec()

	resources, err := Resources(cfg.Resources, "50m", "256Mi")
	if err != nil {
		return nil, err
	}

	env := EnvVars(cfg.EnvironmentVariables)
	if goGCEnv := GoGCEnvFromResources(resources); goGCEnv.Name != "" {
		env = append(env, goGCEnv)
	}

	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8s.ControllerManagerID,
			Namespace: "kube-system",
			Annotations: map[string]string{
				constants.AnnotationStaticPodSecretsVersion: secretsVersion,
				constants.AnnotationStaticPodConfigVersion:  configResource.Metadata().Version().String(),
			},
			Labels: map[string]string{
				"tier":                         "control-plane",
				"k8s-app":                      k8s.ControllerManagerID,
				"component":                    k8s.ControllerManagerID,
				"app.kubernetes.io/name":       k8s.ControllerManagerID,
				"app.kubernetes.io/version":    compatibility.VersionFromImageRef(cfg.Image).String(),
				"app.kubernetes.io/component":  "control-plane",
				"app.kubernetes.io/managed-by": strings.ReplaceAll(version.Name, " ", "-"),
			},
		},
		Spec: corev1.PodSpec{
			Priority:          new(SystemCriticalPriority),
			PriorityClassName: SystemClusterCriticalPriorityClassName,
			Containers: []corev1.Container{
				{
					Name:    k8s.ControllerManagerID,
					Image:   cfg.Image,
					Command: cfg.Args,
					Env: append(
						[]corev1.EnvVar{
							{
								Name: "POD_IP",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "status.podIP",
									},
								},
							},
						},
						env...,
					),
					VolumeMounts: append(append([]corev1.VolumeMount{
						{
							Name:      "secrets",
							MountPath: constants.KubernetesControllerManagerSecretsDir,
							ReadOnly:  true,
						},
					}, EphemeralWritableMounts()...), VolumeMounts(cfg.ExtraVolumes)...),
					StartupProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path:   "/healthz",
								Host:   "localhost",
								Port:   intstr.FromInt(10257),
								Scheme: corev1.URISchemeHTTPS,
							},
						},
						// Give 60 seconds for the container to start up
						PeriodSeconds:                 5,
						FailureThreshold:              12,
						TimeoutSeconds:                15,
						TerminationGracePeriodSeconds: nil,
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path:   "/healthz",
								Host:   "localhost",
								Port:   intstr.FromInt(10257),
								Scheme: corev1.URISchemeHTTPS,
							},
						},
						TimeoutSeconds: 15,
					},
					Resources: resources,
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: new(false),
						ReadOnlyRootFilesystem:   new(true),
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
						},
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
			},
			HostNetwork: true,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: new(true),
				RunAsUser:    new(int64(constants.KubernetesControllerManagerRunUser)),
				RunAsGroup:   new(int64(constants.KubernetesControllerManagerRunGroup)),
			},
			Volumes: append(append([]corev1.Volume{
				{
					Name: "secrets",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: constants.KubernetesControllerManagerSecretsDir,
						},
					},
				},
			}, EphemeralWritableVolumes()...), Volumes(cfg.ExtraVolumes)...),
		},
	}, nil
}
