// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	k8sadapter "github.com/talos-systems/talos/internal/app/machined/pkg/adapters/k8s"
	"github.com/talos-systems/talos/pkg/argsbuilder"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
	"github.com/talos-systems/talos/pkg/machinery/resources/v1alpha1"
)

// ControlPlaneStaticPodController manages k8s.StaticPod based on control plane configuration.
type ControlPlaneStaticPodController struct{}

// Name implements controller.Controller interface.
func (ctrl *ControlPlaneStaticPodController) Name() string {
	return "k8s.ControlPlaneStaticPodController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ControlPlaneStaticPodController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.K8sControlPlaneType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.SecretsStatusType,
			ID:        pointer.ToString(k8s.StaticPodSecretsStaticPodID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        pointer.ToString("etcd"),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ControlPlaneStaticPodController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.StaticPodType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *ControlPlaneStaticPodController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// wait for etcd to be healthy as kube-apiserver is using local etcd instance
		etcdResource, err := r.Get(ctx, resource.NewMetadata(v1alpha1.NamespaceName, v1alpha1.ServiceType, "etcd", resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.teardownAll(ctx, r); err != nil {
					return fmt.Errorf("error tearing down: %w", err)
				}

				continue
			}

			return err
		}

		if !etcdResource.(*v1alpha1.Service).Healthy() {
			continue
		}

		secretsStatusResource, err := r.Get(ctx, resource.NewMetadata(k8s.ControlPlaneNamespaceName, k8s.SecretsStatusType, k8s.StaticPodSecretsStaticPodID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.teardownAll(ctx, r); err != nil {
					return fmt.Errorf("error tearing down: %w", err)
				}

				continue
			}

			return err
		}

		secretsVersion := secretsStatusResource.(*k8s.SecretsStatus).TypedSpec().Version

		touchedIDs := map[string]struct{}{}

		for _, pod := range []struct {
			f  func(context.Context, controller.Runtime, *zap.Logger, *config.K8sControlPlane, string) (string, error)
			id resource.ID
		}{
			{
				f:  ctrl.manageAPIServer,
				id: config.K8sControlPlaneAPIServerID,
			},
			{
				f:  ctrl.manageControllerManager,
				id: config.K8sControlPlaneControllerManagerID,
			},
			{
				f:  ctrl.manageScheduler,
				id: config.K8sControlPlaneSchedulerID,
			},
		} {
			res, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.K8sControlPlaneType, pod.id, resource.VersionUndefined))
			if err != nil {
				if state.IsNotFoundError(err) {
					continue
				}

				return fmt.Errorf("error getting control plane config: %w", err)
			}

			var podID string

			if podID, err = pod.f(ctx, r, logger, res.(*config.K8sControlPlane), secretsVersion); err != nil {
				return fmt.Errorf("error updating static pod for %q: %w", pod.id, err)
			}

			if podID != "" {
				touchedIDs[podID] = struct{}{}
			}
		}

		// clean up static pods which haven't been touched
		{
			list, err := r.List(ctx, resource.NewMetadata(k8s.ControlPlaneNamespaceName, k8s.StaticPodType, "", resource.VersionUndefined))
			if err != nil {
				return err
			}

			for _, res := range list.Items {
				if _, ok := touchedIDs[res.Metadata().ID()]; ok {
					continue
				}

				if err = r.Destroy(ctx, res.Metadata()); err != nil {
					return err
				}
			}
		}
	}
}

func (ctrl *ControlPlaneStaticPodController) teardownAll(ctx context.Context, r controller.Runtime) error {
	list, err := r.List(ctx, resource.NewMetadata(k8s.ControlPlaneNamespaceName, k8s.StaticPodType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	// TODO: change this to proper teardown sequence

	for _, res := range list.Items {
		if err = r.Destroy(ctx, res.Metadata()); err != nil {
			return err
		}
	}

	return nil
}

func volumeMounts(volumes []config.K8sExtraVolume) []v1.VolumeMount {
	result := make([]v1.VolumeMount, 0, len(volumes))

	for _, volume := range volumes {
		result = append(result, v1.VolumeMount{
			Name:      volume.Name,
			MountPath: volume.MountPath,
			ReadOnly:  volume.ReadOnly,
		})
	}

	return result
}

func volumes(volumes []config.K8sExtraVolume) []v1.Volume {
	result := make([]v1.Volume, 0, len(volumes))

	for _, volume := range volumes {
		result = append(result, v1.Volume{
			Name: volume.Name,
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: volume.HostPath,
				},
			},
		})
	}

	return result
}

func (ctrl *ControlPlaneStaticPodController) manageAPIServer(ctx context.Context, r controller.Runtime, logger *zap.Logger,
	configResource *config.K8sControlPlane, secretsVersion string) (string, error) {
	cfg := configResource.APIServer()

	enabledAdmissionPlugins := []string{"NodeRestriction"}

	if cfg.PodSecurityPolicyEnabled {
		enabledAdmissionPlugins = append(enabledAdmissionPlugins, "PodSecurityPolicy")
	}

	args := []string{
		"/usr/local/bin/kube-apiserver",
	}

	builder := argsbuilder.Args{
		"enable-admission-plugins":           strings.Join(enabledAdmissionPlugins, ","),
		"advertise-address":                  "$(POD_IP)",
		"allow-privileged":                   "true",
		"api-audiences":                      cfg.ControlPlaneEndpoint,
		"authorization-mode":                 "Node,RBAC",
		"bind-address":                       "0.0.0.0",
		"client-ca-file":                     filepath.Join(constants.KubernetesAPIServerSecretsDir, "ca.crt"),
		"requestheader-client-ca-file":       filepath.Join(constants.KubernetesAPIServerSecretsDir, "aggregator-ca.crt"),
		"requestheader-allowed-names":        "front-proxy-client",
		"requestheader-extra-headers-prefix": "X-Remote-Extra-",
		"requestheader-group-headers":        "X-Remote-Group",
		"requestheader-username-headers":     "X-Remote-User",
		"proxy-client-cert-file":             filepath.Join(constants.KubernetesAPIServerSecretsDir, "front-proxy-client.crt"),
		"proxy-client-key-file":              filepath.Join(constants.KubernetesAPIServerSecretsDir, "front-proxy-client.key"),
		"enable-bootstrap-token-auth":        "true",
		// NB: using TLS 1.2 instead of 1.3 here for interoperability, since this is an externally-facing service.
		"tls-min-version":                  "VersionTLS12",
		"tls-cipher-suites":                "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256", //nolint:lll
		"encryption-provider-config":       filepath.Join(constants.KubernetesAPIServerSecretsDir, "encryptionconfig.yaml"),
		"audit-policy-file":                filepath.Join(constants.KubernetesAPIServerSecretsDir, "auditpolicy.yaml"),
		"audit-log-path":                   "-",
		"audit-log-maxage":                 "30",
		"audit-log-maxbackup":              "3",
		"audit-log-maxsize":                "50",
		"profiling":                        "false",
		"etcd-cafile":                      filepath.Join(constants.KubernetesAPIServerSecretsDir, "etcd-client-ca.crt"),
		"etcd-certfile":                    filepath.Join(constants.KubernetesAPIServerSecretsDir, "etcd-client.crt"),
		"etcd-keyfile":                     filepath.Join(constants.KubernetesAPIServerSecretsDir, "etcd-client.key"),
		"etcd-servers":                     strings.Join(cfg.EtcdServers, ","),
		"kubelet-client-certificate":       filepath.Join(constants.KubernetesAPIServerSecretsDir, "apiserver-kubelet-client.crt"),
		"kubelet-client-key":               filepath.Join(constants.KubernetesAPIServerSecretsDir, "apiserver-kubelet-client.key"),
		"secure-port":                      strconv.FormatInt(int64(cfg.LocalPort), 10),
		"service-account-issuer":           cfg.ControlPlaneEndpoint,
		"service-account-key-file":         filepath.Join(constants.KubernetesAPIServerSecretsDir, "service-account.pub"),
		"service-account-signing-key-file": filepath.Join(constants.KubernetesAPIServerSecretsDir, "service-account.key"),
		"service-cluster-ip-range":         strings.Join(cfg.ServiceCIDRs, ","),
		"tls-cert-file":                    filepath.Join(constants.KubernetesAPIServerSecretsDir, "apiserver.crt"),
		"tls-private-key-file":             filepath.Join(constants.KubernetesAPIServerSecretsDir, "apiserver.key"),
		"kubelet-preferred-address-types":  "InternalIP,ExternalIP,Hostname",
	}

	if cfg.CloudProvider != "" {
		builder.Set("cloud-provider", cfg.CloudProvider)
	}

	mergePolicies := argsbuilder.MergePolicies{
		"enable-admission-plugins": argsbuilder.MergeAdditive,
		"authorization-mode":       argsbuilder.MergeAdditive,
		"tls-cipher-suites":        argsbuilder.MergeAdditive,

		"etcd-servers":                     argsbuilder.MergeDenied,
		"client-ca-file":                   argsbuilder.MergeDenied,
		"requestheader-client-ca-file":     argsbuilder.MergeDenied,
		"proxy-client-cert-file":           argsbuilder.MergeDenied,
		"proxy-client-key-file":            argsbuilder.MergeDenied,
		"encryption-provider-config":       argsbuilder.MergeDenied,
		"etcd-cafile":                      argsbuilder.MergeDenied,
		"etcd-certfile":                    argsbuilder.MergeDenied,
		"etcd-keyfile":                     argsbuilder.MergeDenied,
		"kubelet-client-certificate":       argsbuilder.MergeDenied,
		"kubelet-client-key":               argsbuilder.MergeDenied,
		"service-account-key-file":         argsbuilder.MergeDenied,
		"service-account-signing-key-file": argsbuilder.MergeDenied,
		"tls-cert-file":                    argsbuilder.MergeDenied,
		"tls-private-key-file":             argsbuilder.MergeDenied,
	}

	if err := builder.Merge(cfg.ExtraArgs, argsbuilder.WithMergePolicies(mergePolicies)); err != nil {
		return "", err
	}

	args = append(args, builder.Args()...)

	return config.K8sControlPlaneAPIServerID, r.Modify(ctx, k8s.NewStaticPod(k8s.ControlPlaneNamespaceName, config.K8sControlPlaneAPIServerID), func(r resource.Resource) error {
		return k8sadapter.StaticPod(r.(*k8s.StaticPod)).SetPod(&v1.Pod{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Pod",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver",
				Namespace: "kube-system",
				Annotations: map[string]string{
					constants.AnnotationStaticPodSecretsVersion: secretsVersion,
					constants.AnnotationStaticPodConfigVersion:  configResource.Metadata().Version().String(),
				},
				Labels: map[string]string{
					"tier":    "control-plane",
					"k8s-app": "kube-apiserver",
				},
			},
			Spec: v1.PodSpec{
				PriorityClassName: "system-cluster-critical",
				Containers: []v1.Container{
					{
						Name:    "kube-apiserver",
						Image:   cfg.Image,
						Command: args,
						Env: []v1.EnvVar{
							{
								Name: "POD_IP",
								ValueFrom: &v1.EnvVarSource{
									FieldRef: &v1.ObjectFieldSelector{
										FieldPath: "status.podIP",
									},
								},
							},
						},
						VolumeMounts: append([]v1.VolumeMount{
							{
								Name:      "secrets",
								MountPath: constants.KubernetesAPIServerSecretsDir,
								ReadOnly:  true,
							},
						}, volumeMounts(cfg.ExtraVolumes)...),
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceCPU:    apiresource.MustParse("200m"),
								v1.ResourceMemory: apiresource.MustParse("512Mi"),
							},
						},
					},
				},
				HostNetwork: true,
				SecurityContext: &v1.PodSecurityContext{
					RunAsNonRoot: pointer.ToBool(true),
					RunAsUser:    pointer.ToInt64(constants.KubernetesRunUser),
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
				}, volumes(cfg.ExtraVolumes)...),
			},
		})
	})
}

func (ctrl *ControlPlaneStaticPodController) manageControllerManager(ctx context.Context, r controller.Runtime,
	logger *zap.Logger, configResource *config.K8sControlPlane, secretsVersion string) (string, error) {
	cfg := configResource.ControllerManager()

	if !cfg.Enabled {
		return "", nil
	}

	args := []string{
		"/usr/local/bin/kube-controller-manager",
		"--use-service-account-credentials",
	}

	builder := argsbuilder.Args{
		"allocate-node-cidrs":              "true",
		"bind-address":                     "127.0.0.1",
		"port":                             "0",
		"cluster-cidr":                     strings.Join(cfg.PodCIDRs, ","),
		"service-cluster-ip-range":         strings.Join(cfg.ServiceCIDRs, ","),
		"cluster-signing-cert-file":        filepath.Join(constants.KubernetesControllerManagerSecretsDir, "ca.crt"),
		"cluster-signing-key-file":         filepath.Join(constants.KubernetesControllerManagerSecretsDir, "ca.key"),
		"controllers":                      "*,tokencleaner",
		"configure-cloud-routes":           "false",
		"kubeconfig":                       filepath.Join(constants.KubernetesControllerManagerSecretsDir, "kubeconfig"),
		"authentication-kubeconfig":        filepath.Join(constants.KubernetesControllerManagerSecretsDir, "kubeconfig"),
		"authorization-kubeconfig":         filepath.Join(constants.KubernetesControllerManagerSecretsDir, "kubeconfig"),
		"leader-elect":                     "true",
		"root-ca-file":                     filepath.Join(constants.KubernetesControllerManagerSecretsDir, "ca.crt"),
		"service-account-private-key-file": filepath.Join(constants.KubernetesControllerManagerSecretsDir, "service-account.key"),
		"profiling":                        "false",
		"tls-min-version":                  "VersionTLS13",
	}

	if cfg.CloudProvider != "" {
		builder.Set("cloud-provider", cfg.CloudProvider)
	}

	mergePolicies := argsbuilder.MergePolicies{
		"service-cluster-ip-range": argsbuilder.MergeAdditive,
		"controllers":              argsbuilder.MergeAdditive,

		"cluster-signing-cert-file":        argsbuilder.MergeDenied,
		"cluster-signing-key-file":         argsbuilder.MergeDenied,
		"authentication-kubeconfig":        argsbuilder.MergeDenied,
		"authorization-kubeconfig":         argsbuilder.MergeDenied,
		"root-ca-file":                     argsbuilder.MergeDenied,
		"service-account-private-key-file": argsbuilder.MergeDenied,
	}

	if err := builder.Merge(cfg.ExtraArgs, argsbuilder.WithMergePolicies(mergePolicies)); err != nil {
		return "", err
	}

	args = append(args, builder.Args()...)

	//nolint:dupl
	return config.K8sControlPlaneControllerManagerID, r.Modify(ctx, k8s.NewStaticPod(k8s.ControlPlaneNamespaceName, config.K8sControlPlaneControllerManagerID), func(r resource.Resource) error {
		return k8sadapter.StaticPod(r.(*k8s.StaticPod)).SetPod(&v1.Pod{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Pod",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-controller-manager",
				Namespace: "kube-system",
				Annotations: map[string]string{
					constants.AnnotationStaticPodSecretsVersion: secretsVersion,
					constants.AnnotationStaticPodConfigVersion:  configResource.Metadata().Version().String(),
				},
				Labels: map[string]string{
					"tier":    "control-plane",
					"k8s-app": "kube-controller-manager",
				},
			},
			Spec: v1.PodSpec{
				PriorityClassName: "system-cluster-critical",
				Containers: []v1.Container{
					{
						Name:    "kube-controller-manager",
						Image:   cfg.Image,
						Command: args,
						VolumeMounts: append([]v1.VolumeMount{
							{
								Name:      "secrets",
								MountPath: constants.KubernetesControllerManagerSecretsDir,
								ReadOnly:  true,
							},
						}, volumeMounts(cfg.ExtraVolumes)...),
						LivenessProbe: &v1.Probe{
							ProbeHandler: v1.ProbeHandler{
								HTTPGet: &v1.HTTPGetAction{
									Path:   "/healthz",
									Host:   "localhost",
									Port:   intstr.FromInt(10257),
									Scheme: v1.URISchemeHTTPS,
								},
							},
							InitialDelaySeconds: 15,
							TimeoutSeconds:      15,
						},
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceCPU:    apiresource.MustParse("50m"),
								v1.ResourceMemory: apiresource.MustParse("256Mi"),
							},
						},
					},
				},
				HostNetwork: true,
				SecurityContext: &v1.PodSecurityContext{
					RunAsNonRoot: pointer.ToBool(true),
					RunAsUser:    pointer.ToInt64(constants.KubernetesRunUser),
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
				}, volumes(cfg.ExtraVolumes)...),
			},
		})
	})
}

func (ctrl *ControlPlaneStaticPodController) manageScheduler(ctx context.Context, r controller.Runtime,
	logger *zap.Logger, configResource *config.K8sControlPlane, secretsVersion string) (string, error) {
	cfg := configResource.Scheduler()

	if !cfg.Enabled {
		return "", nil
	}

	args := []string{
		"/usr/local/bin/kube-scheduler",
	}

	builder := argsbuilder.Args{
		"kubeconfig":                             filepath.Join(constants.KubernetesSchedulerSecretsDir, "kubeconfig"),
		"authentication-tolerate-lookup-failure": "false",
		"authentication-kubeconfig":              filepath.Join(constants.KubernetesSchedulerSecretsDir, "kubeconfig"),
		"authorization-kubeconfig":               filepath.Join(constants.KubernetesSchedulerSecretsDir, "kubeconfig"),
		"bind-address":                           "127.0.0.1",
		"port":                                   "0",
		"leader-elect":                           "true",
		"profiling":                              "false",
		"tls-min-version":                        "VersionTLS13",
	}

	mergePolicies := argsbuilder.MergePolicies{
		"kubeconfig":                argsbuilder.MergeDenied,
		"authentication-kubeconfig": argsbuilder.MergeDenied,
		"authorization-kubeconfig":  argsbuilder.MergeDenied,
	}

	if err := builder.Merge(cfg.ExtraArgs, argsbuilder.WithMergePolicies(mergePolicies)); err != nil {
		return "", err
	}

	args = append(args, builder.Args()...)

	//nolint:dupl
	return config.K8sControlPlaneSchedulerID, r.Modify(ctx, k8s.NewStaticPod(k8s.ControlPlaneNamespaceName, config.K8sControlPlaneSchedulerID), func(r resource.Resource) error {
		return k8sadapter.StaticPod(r.(*k8s.StaticPod)).SetPod(&v1.Pod{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Pod",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-scheduler",
				Namespace: "kube-system",
				Annotations: map[string]string{
					constants.AnnotationStaticPodSecretsVersion: secretsVersion,
					constants.AnnotationStaticPodConfigVersion:  configResource.Metadata().Version().String(),
				},
				Labels: map[string]string{
					"tier":    "control-plane",
					"k8s-app": "kube-scheduler",
				},
			},
			Spec: v1.PodSpec{
				PriorityClassName: "system-cluster-critical",
				Containers: []v1.Container{
					{
						Name:    "kube-scheduler",
						Image:   cfg.Image,
						Command: args,
						VolumeMounts: append([]v1.VolumeMount{
							{
								Name:      "secrets",
								MountPath: constants.KubernetesSchedulerSecretsDir,
								ReadOnly:  true,
							},
						}, volumeMounts(cfg.ExtraVolumes)...),
						LivenessProbe: &v1.Probe{
							ProbeHandler: v1.ProbeHandler{
								HTTPGet: &v1.HTTPGetAction{
									Path:   "/healthz",
									Host:   "localhost",
									Port:   intstr.FromInt(10259),
									Scheme: v1.URISchemeHTTPS,
								},
							},
							InitialDelaySeconds: 15,
							TimeoutSeconds:      15,
						},
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceCPU:    apiresource.MustParse("10m"),
								v1.ResourceMemory: apiresource.MustParse("64Mi"),
							},
						},
					},
				},
				HostNetwork: true,
				SecurityContext: &v1.PodSecurityContext{
					RunAsNonRoot: pointer.ToBool(true),
					RunAsUser:    pointer.ToInt64(constants.KubernetesRunUser),
				},
				Volumes: append([]v1.Volume{
					{
						Name: "secrets",
						VolumeSource: v1.VolumeSource{
							HostPath: &v1.HostPathVolumeSource{
								Path: constants.KubernetesSchedulerSecretsDir,
							},
						},
					},
				}, volumes(cfg.ExtraVolumes)...),
			},
		})
	})
}
