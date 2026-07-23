// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8stemplates

import (
	"fmt"
	"strings"

	"github.com/siderolabs/go-kubernetes/kubernetes/compatibility"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	apiserverv1 "k8s.io/apiserver/pkg/apis/apiserver/v1"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// APIServerEncryptionConfig returns the encryption configuration for the API server.
func APIServerEncryptionConfig(rootK8sSecrets *secrets.KubernetesRootSpec) (runtime.Object, error) {
	if cfg := rootK8sSecrets.EtcdEncryptionConfig; cfg != nil {
		var obj apiserverv1.EncryptionConfiguration

		if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(cfg, &obj, true); err != nil {
			return nil, fmt.Errorf("error converting etcd encryption config: %w", err)
		}

		obj.TypeMeta = metav1.TypeMeta{
			Kind:       "EncryptionConfig",
			APIVersion: apiserverv1.SchemeGroupVersion.Version,
		}

		return &obj, nil
	}

	// legacy path, pre-multidoc Kubernetes config, generated fixed configuration based on the secrets available.
	obj := apiserverv1.EncryptionConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "EncryptionConfig",
			APIVersion: apiserverv1.SchemeGroupVersion.Version,
		},
		Resources: []apiserverv1.ResourceConfiguration{
			{
				Resources: []string{"secrets"},
				Providers: []apiserverv1.ProviderConfiguration{},
			},
		},
	}

	if rootK8sSecrets.SecretboxEncryptionSecret != "" {
		obj.Resources[0].Providers = append(obj.Resources[0].Providers, apiserverv1.ProviderConfiguration{
			Secretbox: &apiserverv1.SecretboxConfiguration{
				Keys: []apiserverv1.Key{
					{
						Name:   "key2",
						Secret: rootK8sSecrets.SecretboxEncryptionSecret,
					},
				},
			},
		})
	}

	if rootK8sSecrets.AESCBCEncryptionSecret != "" {
		obj.Resources[0].Providers = append(obj.Resources[0].Providers, apiserverv1.ProviderConfiguration{
			AESCBC: &apiserverv1.AESConfiguration{
				Keys: []apiserverv1.Key{
					{
						Name:   "key1",
						Secret: rootK8sSecrets.AESCBCEncryptionSecret,
					},
				},
			},
		})
	}

	obj.Resources[0].Providers = append(obj.Resources[0].Providers, apiserverv1.ProviderConfiguration{
		Identity: &apiserverv1.IdentityConfiguration{},
	})

	return &obj, nil
}

// APIServerPod builds a static pod for the kube-apiserver based on the config.
func APIServerPod(configResource *k8s.APIServerConfig, secretsVersion, configVersion string) (runtime.Object, error) {
	cfg := configResource.TypedSpec()

	resources, err := Resources(cfg.Resources, "200m", "512Mi")
	if err != nil {
		return nil, err
	}

	env := EnvVars(cfg.EnvironmentVariables)
	if goGCEnv := GoGCEnvFromResources(resources); goGCEnv.Name != "" {
		env = append(env, goGCEnv)
	}

	// The probes are unauthenticated requests, so they can only be used when anonymous access to the health
	// endpoints is allowed via the authentication config file, otherwise they would be rejected with a 401.
	var (
		startupProbe   *v1.Probe
		livenessProbe  *v1.Probe
		readinessProbe *v1.Probe
	)

	if cfg.StartupProbesEnabled {
		// Probe configuration follows kubeadm defaults.
		startupProbe = &v1.Probe{
			ProbeHandler: v1.ProbeHandler{
				HTTPGet: &v1.HTTPGetAction{
					Path:   "/livez",
					Host:   "localhost",
					Port:   intstr.FromInt(cfg.LocalPort),
					Scheme: v1.URISchemeHTTPS,
				},
			},
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
			TimeoutSeconds:      15,
			FailureThreshold:    24,
		}

		readinessProbe = &v1.Probe{
			ProbeHandler: v1.ProbeHandler{
				HTTPGet: &v1.HTTPGetAction{
					Path:   "/readyz",
					Host:   "localhost",
					Port:   intstr.FromInt(cfg.LocalPort),
					Scheme: v1.URISchemeHTTPS,
				},
			},
			InitialDelaySeconds: 0,
			PeriodSeconds:       1,
			TimeoutSeconds:      15,
			FailureThreshold:    3,
		}

		livenessProbe = &v1.Probe{
			ProbeHandler: v1.ProbeHandler{
				HTTPGet: &v1.HTTPGetAction{
					Path:   "/livez",
					Host:   "localhost",
					Port:   intstr.FromInt(cfg.LocalPort),
					Scheme: v1.URISchemeHTTPS,
				},
			},
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
			TimeoutSeconds:      15,
			FailureThreshold:    8,
		}
	}

	return &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8s.APIServerID,
			Namespace: "kube-system",
			Annotations: map[string]string{
				constants.AnnotationStaticPodSecretsVersion:    secretsVersion,
				constants.AnnotationStaticPodConfigFileVersion: configVersion,
				constants.AnnotationStaticPodConfigVersion:     configResource.Metadata().Version().String(),
			},
			Labels: map[string]string{
				"tier":                         "control-plane",
				"k8s-app":                      k8s.APIServerID,
				"component":                    k8s.APIServerID,
				"app.kubernetes.io/name":       k8s.APIServerID,
				"app.kubernetes.io/version":    compatibility.VersionFromImageRef(cfg.Image).String(),
				"app.kubernetes.io/component":  "control-plane",
				"app.kubernetes.io/managed-by": strings.ReplaceAll(version.Name, " ", "-"),
			},
		},
		Spec: v1.PodSpec{
			Priority:          new(SystemCriticalPriority),
			PriorityClassName: SystemClusterCriticalPriorityClassName,
			Containers: []v1.Container{
				{
					Name:    k8s.APIServerID,
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
							MountPath: constants.KubernetesAPIServerSecretsDir,
							ReadOnly:  true,
						},
						{
							Name:      "config",
							MountPath: constants.KubernetesAPIServerConfigDir,
							ReadOnly:  true,
						},
						{
							Name:      "audit",
							MountPath: constants.KubernetesAuditLogDir,
							ReadOnly:  false,
						},
					}, append(EphemeralWritableMounts(), VolumeMounts(cfg.ExtraVolumes)...)...),
					StartupProbe:   startupProbe,
					LivenessProbe:  livenessProbe,
					ReadinessProbe: readinessProbe,
					Resources:      resources,
					SecurityContext: &v1.SecurityContext{
						AllowPrivilegeEscalation: new(false),
						ReadOnlyRootFilesystem:   new(true),
						Capabilities: &v1.Capabilities{
							Drop: []v1.Capability{"ALL"},
							// kube-apiserver binary has cap_net_bind_service=+ep set.
							// It does not matter if ports < 1024 are configured, the setcap flag causes a capability dependency.
							// https://github.com/kubernetes/kubernetes/blob/5b92e46b2238b4d84358451013e634361084ff7d/build/server-image/kube-apiserver/Dockerfile#L26
							Add: []v1.Capability{"NET_BIND_SERVICE"},
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
				RunAsUser:    new(int64(constants.KubernetesAPIServerRunUser)),
				RunAsGroup:   new(int64(constants.KubernetesAPIServerRunGroup)),
			},
			Volumes: append([]v1.Volume{
				{
					Name: "secrets",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: constants.KubernetesAPIServerSecretsDir,
						},
					},
				},
				{
					Name: "config",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: constants.KubernetesAPIServerConfigDir,
						},
					},
				},
				{
					Name: "audit",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: constants.KubernetesAuditLogDir,
						},
					},
				},
			}, append(EphemeralWritableVolumes(), Volumes(cfg.ExtraVolumes)...)...),
		},
	}, nil
}
