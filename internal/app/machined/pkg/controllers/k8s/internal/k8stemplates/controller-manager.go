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

	return &v1.Pod{
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
		Spec: v1.PodSpec{
			Priority:          new(SystemCriticalPriority),
			PriorityClassName: "system-cluster-critical",
			Containers: []v1.Container{
				{
					Name:    k8s.ControllerManagerID,
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
					VolumeMounts: append([]v1.VolumeMount{
						{
							Name:      "secrets",
							MountPath: constants.KubernetesControllerManagerSecretsDir,
							ReadOnly:  true,
						},
					}, VolumeMounts(cfg.ExtraVolumes)...),
					StartupProbe: &v1.Probe{
						ProbeHandler: v1.ProbeHandler{
							HTTPGet: &v1.HTTPGetAction{
								Path:   "/healthz",
								Host:   "localhost",
								Port:   intstr.FromInt(10257),
								Scheme: v1.URISchemeHTTPS,
							},
						},
						// Give 60 seconds for the container to start up
						PeriodSeconds:                 5,
						FailureThreshold:              12,
						TerminationGracePeriodSeconds: nil,
					},
					LivenessProbe: &v1.Probe{
						ProbeHandler: v1.ProbeHandler{
							HTTPGet: &v1.HTTPGetAction{
								Path:   "/healthz",
								Host:   "localhost",
								Port:   intstr.FromInt(10257),
								Scheme: v1.URISchemeHTTPS,
							},
						},
						TimeoutSeconds: 15,
					},
					Resources: resources,
					SecurityContext: &v1.SecurityContext{
						AllowPrivilegeEscalation: new(false),
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
				RunAsUser:    new(int64(constants.KubernetesControllerManagerRunUser)),
				RunAsGroup:   new(int64(constants.KubernetesControllerManagerRunGroup)),
			},
			Volumes: append([]v1.Volume{
				{
					Name: "secrets",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: constants.KubernetesControllerManagerSecretsDir,
						},
					},
				},
			}, Volumes(cfg.ExtraVolumes)...),
		},
	}, nil
}
