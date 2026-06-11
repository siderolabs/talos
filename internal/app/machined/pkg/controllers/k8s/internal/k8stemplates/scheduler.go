// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8stemplates

import (
	"strings"

	"github.com/siderolabs/go-kubernetes/kubernetes/compatibility"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// SchedulerPod builds a static pod for the kube-scheduler based on the config.
func SchedulerPod(configResource *k8s.SchedulerConfig, secretsVersion string) (runtime.Object, error) {
	cfg := configResource.TypedSpec()

	resources, err := Resources(cfg.Resources, "10m", "64Mi")
	if err != nil {
		return nil, err
	}

	env := EnvVars(cfg.EnvironmentVariables)
	if goGCEnv := GoGCEnvFromResources(resources); goGCEnv.Name != "" {
		env = append(env, goGCEnv)
	}

	kubeSchedulerVersion := compatibility.VersionFromImageRef(cfg.Image)

	livenessProbe := &v1.Probe{
		ProbeHandler: v1.ProbeHandler{
			HTTPGet: &v1.HTTPGetAction{
				Path:   kubeSchedulerVersion.KubeSchedulerHealthLivenessEndpoint(),
				Host:   "localhost",
				Port:   intstr.FromInt(10259),
				Scheme: v1.URISchemeHTTPS,
			},
		},
		TimeoutSeconds: 15,
	}

	readinessProbe := &v1.Probe{
		ProbeHandler: v1.ProbeHandler{
			HTTPGet: &v1.HTTPGetAction{
				Path:   kubeSchedulerVersion.KubeSchedulerHealthReadinessEndpoint(),
				Host:   "localhost",
				Port:   intstr.FromInt(10259),
				Scheme: v1.URISchemeHTTPS,
			},
		},
		TimeoutSeconds: 15,
	}

	startupProbe := &v1.Probe{
		ProbeHandler: v1.ProbeHandler{
			HTTPGet: &v1.HTTPGetAction{
				Path:   kubeSchedulerVersion.KubeSchedulerHealthStartupEndpoint(),
				Host:   "localhost",
				Port:   intstr.FromInt(10259),
				Scheme: v1.URISchemeHTTPS,
			},
		},
		// Give 60 seconds for the container to start up
		PeriodSeconds:    5,
		FailureThreshold: 12,
		TimeoutSeconds:   15,
	}

	return &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8s.SchedulerID,
			Namespace: "kube-system",
			Annotations: map[string]string{
				constants.AnnotationStaticPodSecretsVersion: secretsVersion,
				constants.AnnotationStaticPodConfigVersion:  configResource.Metadata().Version().String(),
			},
			Labels: map[string]string{
				"tier":                         "control-plane",
				"k8s-app":                      k8s.SchedulerID,
				"component":                    k8s.SchedulerID,
				"app.kubernetes.io/name":       k8s.SchedulerID,
				"app.kubernetes.io/version":    compatibility.VersionFromImageRef(cfg.Image).String(),
				"app.kubernetes.io/component":  "control-plane",
				"app.kubernetes.io/managed-by": strings.ReplaceAll(version.Name, " ", "-"),
			},
		},
		Spec: v1.PodSpec{
			Priority:          new(SystemCriticalPriority),
			PriorityClassName: "system-cluster-critical",
			Containers: []v1.Container{
				{
					Name:    k8s.SchedulerID,
					Image:   cfg.Image,
					Command: cfg.Args,
					Env: append(
						[]v1.EnvVar{
							{
								Name: "POD_IP",
								ValueFrom: &v1.EnvVarSource{
									FieldRef: &v1.ObjectFieldSelector{
										FieldPath: "status.podIP",
									},
								},
							},
						},
						env...,
					),
					VolumeMounts: append(append([]v1.VolumeMount{
						{
							Name:      "secrets",
							MountPath: constants.KubernetesSchedulerSecretsDir,
							ReadOnly:  true,
						},
						{
							Name:      "config",
							MountPath: constants.KubernetesSchedulerConfigDir,
							ReadOnly:  true,
						},
					}, EphemeralWritableMounts()...), VolumeMounts(cfg.ExtraVolumes)...),
					StartupProbe:   startupProbe,
					LivenessProbe:  livenessProbe,
					ReadinessProbe: readinessProbe,
					Resources:      resources,
					SecurityContext: &v1.SecurityContext{
						AllowPrivilegeEscalation: new(false),
						ReadOnlyRootFilesystem:   new(true),
						Capabilities: &v1.Capabilities{
							Drop: []v1.Capability{"ALL"},
						},
						SeccompProfile: &v1.SeccompProfile{
							Type: v1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
			},
			HostNetwork: true,
			SecurityContext: &v1.PodSecurityContext{
				RunAsNonRoot: new(true),
				RunAsUser:    new(int64(constants.KubernetesSchedulerRunUser)),
				RunAsGroup:   new(int64(constants.KubernetesSchedulerRunGroup)),
			},
			Volumes: append(append([]v1.Volume{
				{
					Name: "secrets",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: constants.KubernetesSchedulerSecretsDir,
						},
					},
				},
				{
					Name: "config",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: constants.KubernetesSchedulerConfigDir,
						},
					},
				},
			}, EphemeralWritableVolumes()...), Volumes(cfg.ExtraVolumes)...),
		},
	}, nil
}
