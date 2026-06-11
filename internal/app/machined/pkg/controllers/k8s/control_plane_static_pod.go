// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-kubernetes/kubernetes/compatibility"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	k8sadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/k8s"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s/internal/k8stemplates"
	"github.com/siderolabs/talos/pkg/argsbuilder"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/version"
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
			ID:        optional.Some(k8s.FinalControllerManagerConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.SchedulerConfigType,
			ID:        optional.Some(k8s.FinalSchedulerConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.SecretsStatusType,
			ID:        optional.Some(k8s.StaticPodSecretsStaticPodID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.ConfigStatusType,
			ID:        optional.Some(k8s.ConfigStatusStaticPodID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some("etcd"),
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
		etcdResource, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, r, "etcd")
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get etcd service status: %w", err)
		}

		secretsStatusResource, err := safe.ReaderGetByID[*k8s.SecretsStatus](ctx, r, k8s.StaticPodSecretsStaticPodID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get secrets status resource: %w", err)
		}

		configStatusResource, err := safe.ReaderGetByID[*k8s.ConfigStatus](ctx, r, k8s.ConfigStatusStaticPodID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get config status resource: %w", err)
		}

		r.StartTrackingOutputs()

		// pre-condition to produce static pods
		if etcdResource != nil && etcdResource.TypedSpec().Healthy && configStatusResource != nil && secretsStatusResource != nil {
			configVersion := configStatusResource.TypedSpec().Version
			secretsVersion := secretsStatusResource.TypedSpec().Version

			for _, manageFunc := range []func(context.Context, controller.Runtime, *zap.Logger, string, string) error{
				ctrl.manageAPIServer,
				ctrl.manageControllerManager,
				ctrl.manageScheduler,
			} {
				if err = manageFunc(ctx, r, logger, secretsVersion, configVersion); err != nil {
					return err
				}
			}
		}

		// clean up static pods which haven't been touched
		if err := safe.CleanupOutputs[*k8s.StaticPod](ctx, r); err != nil {
			return fmt.Errorf("failed to cleanup outputs: %w", err)
		}
	}
}

//nolint:gocyclo
func (ctrl *ControlPlaneStaticPodController) manageAPIServer(ctx context.Context, r controller.Runtime, _ *zap.Logger,
	secretsVersion, configVersion string,
) error {
	configResource, err := safe.ReaderGetByID[*k8s.APIServerConfig](ctx, r, k8s.APIServerConfigID)
	if err != nil {
		if state.IsNotFoundError(err) {
			// no config => no pod
			return nil
		}

		return fmt.Errorf("failed to get apiserver config: %w", err)
	}

	cfg := configResource.TypedSpec()

	enabledAdmissionPlugins := []string{"NodeRestriction"}

	args := []string{ //nolint:prealloc // very dynamic length
		"/usr/local/bin/kube-apiserver",
	}

	builder := argsbuilder.Args{
		"admission-control-config-file":      {filepath.Join(constants.KubernetesAPIServerConfigDir, "admission-control-config.yaml")},
		"allow-privileged":                   {"true"},
		"api-audiences":                      {cfg.ControlPlaneEndpoint},
		"bind-address":                       {"0.0.0.0"},
		"client-ca-file":                     {filepath.Join(constants.KubernetesAPIServerSecretsDir, "ca.crt")},
		"enable-admission-plugins":           {strings.Join(enabledAdmissionPlugins, ",")},
		"requestheader-client-ca-file":       {filepath.Join(constants.KubernetesAPIServerSecretsDir, "aggregator-ca.crt")},
		"requestheader-allowed-names":        {"front-proxy-client"},
		"requestheader-extra-headers-prefix": {"X-Remote-Extra-"},
		"requestheader-group-headers":        {"X-Remote-Group"},
		"requestheader-username-headers":     {"X-Remote-User"},
		"proxy-client-cert-file":             {filepath.Join(constants.KubernetesAPIServerSecretsDir, "front-proxy-client.crt")},
		"proxy-client-key-file":              {filepath.Join(constants.KubernetesAPIServerSecretsDir, "front-proxy-client.key")},
		"enable-bootstrap-token-auth":        {"true"},
		"tls-min-version":                    {"VersionTLS13"},
		"encryption-provider-config":         {filepath.Join(constants.KubernetesAPIServerSecretsDir, "encryptionconfig.yaml")},
		"audit-policy-file":                  {filepath.Join(constants.KubernetesAPIServerConfigDir, "auditpolicy.yaml")},
		"audit-log-path":                     {filepath.Join(constants.KubernetesAuditLogDir, "kube-apiserver.log")},
		"audit-log-maxage":                   {"30"},
		"audit-log-maxbackup":                {"10"},
		"audit-log-maxsize":                  {"100"},
		"profiling":                          {"false"},
		"etcd-cafile":                        {filepath.Join(constants.KubernetesAPIServerSecretsDir, "etcd-client-ca.crt")},
		"etcd-certfile":                      {filepath.Join(constants.KubernetesAPIServerSecretsDir, "etcd-client.crt")},
		"etcd-keyfile":                       {filepath.Join(constants.KubernetesAPIServerSecretsDir, "etcd-client.key")},
		"etcd-servers":                       {strings.Join(cfg.EtcdServers, ",")},
		"kubelet-client-certificate":         {filepath.Join(constants.KubernetesAPIServerSecretsDir, "apiserver-kubelet-client.crt")},
		"kubelet-client-key":                 {filepath.Join(constants.KubernetesAPIServerSecretsDir, "apiserver-kubelet-client.key")},
		"secure-port":                        {strconv.FormatInt(int64(cfg.LocalPort), 10)},
		"service-account-issuer":             {cfg.ControlPlaneEndpoint},
		"service-account-key-file":           {filepath.Join(constants.KubernetesAPIServerSecretsDir, "service-account.pub")},
		"service-account-signing-key-file":   {filepath.Join(constants.KubernetesAPIServerSecretsDir, "service-account.key")},
		"service-cluster-ip-range":           {strings.Join(cfg.ServiceCIDRs, ",")},
		"tls-cert-file":                      {filepath.Join(constants.KubernetesAPIServerSecretsDir, "apiserver.crt")},
		"tls-private-key-file":               {filepath.Join(constants.KubernetesAPIServerSecretsDir, "apiserver.key")},
		"kubelet-preferred-address-types":    {"InternalIP,ExternalIP,Hostname"},
	}

	if cfg.AdvertisedAddress != "" {
		builder.Set("advertise-address", argsbuilder.Value{cfg.AdvertisedAddress})
	}

	k8sVersion := compatibility.VersionFromImageRef(cfg.Image)

	if cfg.CloudProvider != "" && !k8sVersion.CloudProviderFlagRemoved() {
		builder.Set("cloud-provider", argsbuilder.Value{cfg.CloudProvider})
	}

	extraArgs := make(argsbuilder.Args, len(cfg.ExtraArgs))
	for k, v := range cfg.ExtraArgs {
		extraArgs[k] = v.Values
	}

	handleKubeAPIServerAuthorizationFlags(k8sVersion, builder, extraArgs)

	// Anonymous requests are not accepted by default, otherwise the kube-apiserver would set the request's
	// group to system:unauthenticated, exposing endpoints like /version etc. Anonymous access is instead
	// restricted to the health endpoints only (via the authentication config file), so the kubelet HTTP
	// liveness probe can reach /livez without credentials while every other endpoint keeps rejecting anonymous
	// requests.
	useAuthenticationConfig := HandleKubeAPIServerAnonymousAuthFlags(builder, extraArgs)

	mergePolicies := argsbuilder.MergePolicies{
		"enable-admission-plugins": argsbuilder.MergeAdditive,
		"feature-gates":            argsbuilder.MergeAdditive,
		"authorization-mode":       argsbuilder.MergeAdditive,

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
		"tls-min-version":                  argsbuilder.MergeDenied,
		"tls-private-key-file":             argsbuilder.MergeDenied,
		"authorization-config":             argsbuilder.MergeDenied,
		"authentication-config":            argsbuilder.MergeDenied,
	}

	if err := builder.Merge(extraArgs, argsbuilder.WithMergePolicies(mergePolicies)); err != nil {
		return err
	}

	args = append(args, builder.Args()...)

	resources, err := k8stemplates.Resources(cfg.Resources, "200m", "512Mi")
	if err != nil {
		return err
	}

	env := k8stemplates.EnvVars(cfg.EnvironmentVariables)
	if goGCEnv := k8stemplates.GoGCEnvFromResources(resources); goGCEnv.Name != "" {
		env = append(env, goGCEnv)
	}

	// The probes are unauthenticated requests, so they can only be used when anonymous access to the health
	// endpoints is allowed via the authentication config file, otherwise they would be rejected with a 401.
	var (
		startupProbe   *v1.Probe
		livenessProbe  *v1.Probe
		readinessProbe *v1.Probe
	)

	if useAuthenticationConfig {
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

	return safe.WriterModify(ctx, r, k8s.NewStaticPod(k8s.NamespaceName, k8s.APIServerID), func(r *k8s.StaticPod) error {
		return k8sadapter.StaticPod(r).SetPod(&v1.Pod{
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
					"app.kubernetes.io/version":    k8sVersion.String(),
					"app.kubernetes.io/component":  "control-plane",
					"app.kubernetes.io/managed-by": strings.ReplaceAll(version.Name, " ", "-"),
				},
			},
			Spec: v1.PodSpec{
				Priority:          new(k8stemplates.SystemCriticalPriority),
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
						}, k8stemplates.VolumeMounts(cfg.ExtraVolumes)...),
						StartupProbe:   startupProbe,
						LivenessProbe:  livenessProbe,
						ReadinessProbe: readinessProbe,
						Resources:      resources,
						SecurityContext: &v1.SecurityContext{
							AllowPrivilegeEscalation: new(false),
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
				}, k8stemplates.Volumes(cfg.ExtraVolumes)...),
			},
		})
	})
}

func (ctrl *ControlPlaneStaticPodController) manageControllerManager(ctx context.Context, r controller.Runtime,
	_ *zap.Logger, secretsVersion, _ string,
) error {
	configResource, err := safe.ReaderGetByID[*k8s.ControllerManagerConfig](ctx, r, k8s.FinalControllerManagerConfigID)
	if err != nil {
		if state.IsNotFoundError(err) {
			// no config => no pod
			return nil
		}

		return fmt.Errorf("failed to get controller-manager config: %w", err)
	}

	if !configResource.TypedSpec().Enabled {
		return nil
	}

	pod, err := k8stemplates.ControllerManagerPod(configResource, secretsVersion)
	if err != nil {
		return fmt.Errorf("error building controller-manager pod: %w", err)
	}

	return safe.WriterModify(ctx, r, k8s.NewStaticPod(k8s.NamespaceName, k8s.ControllerManagerID), func(r *k8s.StaticPod) error {
		return k8sadapter.StaticPod(r).SetPod(pod)
	})
}

func (ctrl *ControlPlaneStaticPodController) manageScheduler(ctx context.Context, r controller.Runtime,
	_ *zap.Logger, secretsVersion, _ string,
) error {
	configResource, err := safe.ReaderGetByID[*k8s.SchedulerConfig](ctx, r, k8s.FinalSchedulerConfigID)
	if err != nil {
		if state.IsNotFoundError(err) {
			// no config => no pod
			return nil
		}

		return fmt.Errorf("failed to get scheduler config: %w", err)
	}

	cfg := configResource.TypedSpec()

	if !cfg.Enabled {
		return nil
	}

	obj, err := k8stemplates.SchedulerPod(configResource, secretsVersion)
	if err != nil {
		return fmt.Errorf("error building kube-scheduler pod: %w", err)
	}

	return safe.WriterModify(ctx, r, k8s.NewStaticPod(k8s.NamespaceName, k8s.SchedulerID), func(r *k8s.StaticPod) error {
		return k8sadapter.StaticPod(r).SetPod(obj)
	})
}

func kubeAPIServerExtraArgsHasAuthorizationWebhookFlags(extraArgs map[string][]string) bool {
	return slices.ContainsFunc(maps.Keys(extraArgs), func(arg string) bool {
		return strings.HasPrefix(arg, "authorization-webhook-")
	})
}

func kubeAPIServerExtraArgsHasAuthorizationModeFlag(extraArgs map[string][]string) bool {
	_, ok := extraArgs["authorization-mode"]

	return ok
}

func handleKubeAPIServerAuthorizationFlags(kubeVersion compatibility.Version, argBuilder argsbuilder.Args, extraArgs map[string][]string) {
	// this handle multiple cases:
	// 1. user already has set `authorization-mode` flag, we'll just merge our default `authorization-mode` flag
	if kubeAPIServerExtraArgsHasAuthorizationModeFlag(extraArgs) {
		argBuilder.Set("authorization-mode", argsbuilder.Value{"Node,RBAC"})

		return
	}

	// 2. user has set `authorization-webhook-*` flags, we'll just merge our default `authorization-mode` flag
	if kubeAPIServerExtraArgsHasAuthorizationWebhookFlags(extraArgs) {
		argBuilder.Set("authorization-mode", argsbuilder.Value{"Node,RBAC"})

		return
	}

	// 3. user has not set `authorization-mode` flag and the kube-apiserver version doesn't support `authorization-config` flag
	// machine config validation should handle the case where either of `authorization-mode` or `authorization-webhook-*` flags are set
	// along with `authorizationConfig`
	if !kubeVersion.KubeAPIServerSupportsAuthorizationConfigFile() {
		argBuilder.Set("authorization-mode", argsbuilder.Value{"Node,RBAC"})

		return
	}

	if !kubeVersion.FeatureFlagStructuredAuthorizationConfigurationEnabledByDefault() {
		// feature-gates flag can be set multiple times, since it has merge addictive policy
		argBuilder.Set("feature-gates", argsbuilder.Value{"StructuredAuthorizationConfiguration=true"})
	}

	argBuilder.Set("authorization-config", argsbuilder.Value{filepath.Join(constants.KubernetesAPIServerConfigDir, "authorization-config.yaml")})
}

// KubeAPIServerExtraArgsConflictWithAuthenticationConfig returns true if user-provided extra args conflict with
// the structured authentication config file, which is mutually exclusive with the --anonymous-auth and --oidc-* flags.
func KubeAPIServerExtraArgsConflictWithAuthenticationConfig(extraArgs map[string][]string) bool {
	return slices.ContainsFunc(maps.Keys(extraArgs), func(arg string) bool {
		return arg == "anonymous-auth" || arg == "authentication-config" || strings.HasPrefix(arg, "oidc-")
	})
}

// HandleKubeAPIServerAnonymousAuthFlags configures anonymous authentication for the kube-apiserver.
//
// It returns true if anonymous auth is managed via the structured authentication config file
// (--authentication-config), which restricts anonymous access to the health endpoints only. Otherwise anonymous
// auth is fully disabled via --anonymous-auth=false and the caller must not rely on unauthenticated probes.
func HandleKubeAPIServerAnonymousAuthFlags(argBuilder argsbuilder.Args, extraArgs map[string][]string) bool {
	// --authentication-config is mutually exclusive with the --anonymous-auth and --oidc-* flags, so when the
	// user provides any of those via extra args, fall back to fully disabling anonymous auth.
	if KubeAPIServerExtraArgsConflictWithAuthenticationConfig(extraArgs) {
		argBuilder.Set("anonymous-auth", argsbuilder.Value{"false"})

		return false
	}

	argBuilder.Set("authentication-config", argsbuilder.Value{filepath.Join(constants.KubernetesAPIServerConfigDir, "authentication-config.yaml")})

	return true
}
