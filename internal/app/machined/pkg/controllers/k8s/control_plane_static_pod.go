// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/slices"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	k8sadapter "github.com/talos-systems/talos/internal/app/machined/pkg/adapters/k8s"
	"github.com/talos-systems/talos/pkg/argsbuilder"
	"github.com/talos-systems/talos/pkg/machinery/constants"
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
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.APIServerConfigType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.ControllerManagerConfigType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.SchedulerConfigType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.SecretsStatusType,
			ID:        pointer.To(k8s.StaticPodSecretsStaticPodID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.ConfigStatusType,
			ID:        pointer.To(k8s.ConfigStatusStaticPodID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        pointer.To("etcd"),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ControlPlaneStaticPodController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.StaticPodType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
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

		if !etcdResource.(*v1alpha1.Service).TypedSpec().Healthy {
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

		configStatusResource, err := r.Get(ctx, resource.NewMetadata(k8s.ControlPlaneNamespaceName, k8s.ConfigStatusType, k8s.ConfigStatusStaticPodID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.teardownAll(ctx, r); err != nil {
					return fmt.Errorf("error tearing down: %w", err)
				}

				continue
			}

			return err
		}

		configVersion := configStatusResource.(*k8s.ConfigStatus).TypedSpec().Version

		touchedIDs := map[string]struct{}{}

		for _, pod := range []struct {
			f  func(context.Context, controller.Runtime, *zap.Logger, resource.Resource, string, string) (string, error)
			md *resource.Metadata
		}{
			{
				f:  ctrl.manageAPIServer,
				md: k8s.NewAPIServerConfig().Metadata(),
			},
			{
				f:  ctrl.manageControllerManager,
				md: k8s.NewControllerManagerConfig().Metadata(),
			},
			{
				f:  ctrl.manageScheduler,
				md: k8s.NewSchedulerConfig().Metadata(),
			},
		} {
			res, err := r.Get(ctx, pod.md)
			if err != nil {
				if state.IsNotFoundError(err) {
					continue
				}

				return fmt.Errorf("error getting control plane config: %w", err)
			}

			var podID string

			if podID, err = pod.f(ctx, r, logger, res, secretsVersion, configVersion); err != nil {
				return fmt.Errorf("error updating static pod for %q: %w", pod.md.Type(), err)
			}

			if podID != "" {
				touchedIDs[podID] = struct{}{}
			}
		}

		// clean up static pods which haven't been touched
		{
			list, err := r.List(ctx, resource.NewMetadata(k8s.NamespaceName, k8s.StaticPodType, "", resource.VersionUndefined))
			if err != nil {
				return err
			}

			for _, res := range list.Items {
				if _, ok := touchedIDs[res.Metadata().ID()]; ok {
					continue
				}

				if res.Metadata().Owner() != ctrl.Name() {
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
	list, err := r.List(ctx, resource.NewMetadata(k8s.NamespaceName, k8s.StaticPodType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	for _, res := range list.Items {
		if res.Metadata().Owner() != ctrl.Name() {
			continue
		}

		if err = r.Destroy(ctx, res.Metadata()); err != nil {
			return err
		}
	}

	return nil
}

func volumeMounts(volumes []k8s.ExtraVolume) []v1.VolumeMount {
	return slices.Map(volumes, func(vol k8s.ExtraVolume) v1.VolumeMount {
		return v1.VolumeMount{
			Name:      vol.Name,
			MountPath: vol.MountPath,
			ReadOnly:  vol.ReadOnly,
		}
	})
}

func volumes(volumes []k8s.ExtraVolume) []v1.Volume {
	return slices.Map(volumes, func(vol k8s.ExtraVolume) v1.Volume {
		return v1.Volume{
			Name: vol.Name,
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: vol.HostPath,
				},
			},
		}
	})
}

func envVars(environment map[string]string) []v1.EnvVar {
	if len(environment) == 0 {
		return nil
	}

	keys := maps.Keys(environment)
	sort.Strings(keys)

	return slices.Map(keys, func(key string) v1.EnvVar {
		// Kubernetes supports variable references in variable values, so escape '$' to prevent that.
		return v1.EnvVar{
			Name:  key,
			Value: strings.ReplaceAll(environment[key], "$", "$$"),
		}
	})
}

func (ctrl *ControlPlaneStaticPodController) manageAPIServer(ctx context.Context, r controller.Runtime, logger *zap.Logger,
	configResource resource.Resource, secretsVersion, configVersion string,
) (string, error) {
	cfg := configResource.(*k8s.APIServerConfig).TypedSpec()

	enabledAdmissionPlugins := []string{"NodeRestriction"}

	if cfg.PodSecurityPolicyEnabled {
		enabledAdmissionPlugins = append(enabledAdmissionPlugins, "PodSecurityPolicy")
	}

	args := []string{
		"/usr/local/bin/kube-apiserver",
	}

	builder := argsbuilder.Args{
		"admission-control-config-file": filepath.Join(constants.KubernetesAPIServerConfigDir, "admission-control-config.yaml"),
		"allow-privileged":              "true",
		// Do not accept anonymous requests by default. Otherwise the kube-apiserver will set the request's group to system:unauthenticated exposing endpoints like /version etc.
		"anonymous-auth":                     "false",
		"api-audiences":                      cfg.ControlPlaneEndpoint,
		"authorization-mode":                 "Node,RBAC",
		"bind-address":                       "0.0.0.0",
		"client-ca-file":                     filepath.Join(constants.KubernetesAPIServerSecretsDir, "ca.crt"),
		"enable-admission-plugins":           strings.Join(enabledAdmissionPlugins, ","),
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
		"audit-policy-file":                filepath.Join(constants.KubernetesAPIServerConfigDir, "auditpolicy.yaml"),
		"audit-log-path":                   filepath.Join(constants.KubernetesAuditLogDir, "kube-apiserver.log"),
		"audit-log-maxage":                 "30",
		"audit-log-maxbackup":              "10",
		"audit-log-maxsize":                "100",
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

	if cfg.AdvertisedAddress != "" {
		builder.Set("advertise-address", cfg.AdvertisedAddress)
	}

	if cfg.CloudProvider != "" {
		builder.Set("cloud-provider", cfg.CloudProvider)
	}

	mergePolicies := argsbuilder.MergePolicies{
		"enable-admission-plugins": argsbuilder.MergeAdditive,
		"feature-gates":            argsbuilder.MergeAdditive,
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

	return k8s.APIServerID, r.Modify(ctx, k8s.NewStaticPod(k8s.NamespaceName, k8s.APIServerID), func(r resource.Resource) error {
		return k8sadapter.StaticPod(r.(*k8s.StaticPod)).SetPod(&v1.Pod{
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
					"tier":    "control-plane",
					"k8s-app": k8s.APIServerID,
				},
			},
			Spec: v1.PodSpec{
				PriorityClassName: "system-cluster-critical",
				Containers: []v1.Container{
					{
						Name:    k8s.APIServerID,
						Image:   cfg.Image,
						Command: args,
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
							envVars(cfg.EnvironmentVariables)...),
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
						}, volumeMounts(cfg.ExtraVolumes)...),
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceCPU:    apiresource.MustParse("200m"),
								v1.ResourceMemory: apiresource.MustParse("512Mi"),
							},
						},
						SecurityContext: &v1.SecurityContext{
							AllowPrivilegeEscalation: pointer.To(false),
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
					RunAsNonRoot: pointer.To(true),
					RunAsUser:    pointer.To[int64](constants.KubernetesAPIServerRunUser),
					RunAsGroup:   pointer.To[int64](constants.KubernetesAPIServerRunGroup),
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
				}, volumes(cfg.ExtraVolumes)...),
			},
		})
	})
}

func (ctrl *ControlPlaneStaticPodController) manageControllerManager(ctx context.Context, r controller.Runtime,
	logger *zap.Logger, configResource resource.Resource, secretsVersion, configVersion string,
) (string, error) {
	cfg := configResource.(*k8s.ControllerManagerConfig).TypedSpec()

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
	return k8s.ControllerManagerID, r.Modify(ctx, k8s.NewStaticPod(k8s.NamespaceName, k8s.ControllerManagerID), func(r resource.Resource) error {
		return k8sadapter.StaticPod(r.(*k8s.StaticPod)).SetPod(&v1.Pod{
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
					"tier":    "control-plane",
					"k8s-app": k8s.ControllerManagerID,
				},
			},
			Spec: v1.PodSpec{
				PriorityClassName: "system-cluster-critical",
				Containers: []v1.Container{
					{
						Name:    k8s.ControllerManagerID,
						Image:   cfg.Image,
						Command: args,
						Env:     envVars(cfg.EnvironmentVariables),
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
						SecurityContext: &v1.SecurityContext{
							AllowPrivilegeEscalation: pointer.To(false),
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
					RunAsNonRoot: pointer.To(true),
					RunAsUser:    pointer.To[int64](constants.KubernetesControllerManagerRunUser),
					RunAsGroup:   pointer.To[int64](constants.KubernetesControllerManagerRunGroup),
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
	logger *zap.Logger, configResource resource.Resource, secretsVersion, configVersion string,
) (string, error) {
	cfg := configResource.(*k8s.SchedulerConfig).TypedSpec()

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
	return k8s.SchedulerID, r.Modify(ctx, k8s.NewStaticPod(k8s.NamespaceName, k8s.SchedulerID), func(r resource.Resource) error {
		return k8sadapter.StaticPod(r.(*k8s.StaticPod)).SetPod(&v1.Pod{
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
					"tier":    "control-plane",
					"k8s-app": k8s.SchedulerID,
				},
			},
			Spec: v1.PodSpec{
				PriorityClassName: "system-cluster-critical",
				Containers: []v1.Container{
					{
						Name:    k8s.SchedulerID,
						Image:   cfg.Image,
						Command: args,
						Env:     envVars(cfg.EnvironmentVariables),
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
						SecurityContext: &v1.SecurityContext{
							AllowPrivilegeEscalation: pointer.To(false),
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
					RunAsNonRoot: pointer.To(true),
					RunAsUser:    pointer.To[int64](constants.KubernetesSchedulerRunUser),
					RunAsGroup:   pointer.To[int64](constants.KubernetesSchedulerRunGroup),
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
