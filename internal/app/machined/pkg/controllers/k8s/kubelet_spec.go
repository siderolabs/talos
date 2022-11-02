// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/slices"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"

	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/argsbuilder"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/kubelet"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// KubeletSpecController renders manifests based on templates and config/secrets.
type KubeletSpecController struct {
	V1Alpha1Mode v1alpha1runtime.Mode
}

// Name implements controller.Controller interface.
func (ctrl *KubeletSpecController) Name() string {
	return "k8s.KubeletSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KubeletSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.KubeletConfigType,
			ID:        pointer.To(k8s.KubeletID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodenameType,
			ID:        pointer.To(k8s.NodenameID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodeIPType,
			ID:        pointer.To(k8s.KubeletID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *KubeletSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.KubeletSpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *KubeletSpecController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := r.Get(ctx, resource.NewMetadata(k8s.NamespaceName, k8s.KubeletConfigType, k8s.KubeletID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		cfgSpec := cfg.(*k8s.KubeletConfig).TypedSpec()

		nodename, err := r.Get(ctx, resource.NewMetadata(k8s.NamespaceName, k8s.NodenameType, k8s.NodenameID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting nodename: %w", err)
		}

		nodenameSpec := nodename.(*k8s.Nodename).TypedSpec()

		expectedNodename := nodenameSpec.Nodename

		args := argsbuilder.Args{
			"container-runtime":          "remote",
			"container-runtime-endpoint": "unix://" + constants.CRIContainerdAddress,
			"config":                     "/etc/kubernetes/kubelet.yaml",

			"cert-dir": constants.KubeletPKIDir,

			"hostname-override": expectedNodename,
		}

		if !cfgSpec.SkipNodeRegistration {
			args["bootstrap-kubeconfig"] = constants.KubeletBootstrapKubeconfig
			args["kubeconfig"] = constants.KubeletKubeconfig
		}

		if cfgSpec.CloudProviderExternal {
			args["cloud-provider"] = "external"
		}

		extraArgs := argsbuilder.Args(cfgSpec.ExtraArgs)

		// if the user supplied a hostname override, we do not manage it anymore
		if extraArgs.Contains("hostname-override") {
			expectedNodename = ""
		}

		// if the user supplied node-ip via extra args, no need to pick automatically
		if !extraArgs.Contains("node-ip") {
			var nodeIP resource.Resource

			nodeIP, err = r.Get(ctx, resource.NewMetadata(k8s.NamespaceName, k8s.NodeIPType, k8s.KubeletID, resource.VersionUndefined))
			if err != nil {
				if state.IsNotFoundError(err) {
					continue
				}

				return fmt.Errorf("error getting node IPs: %w", err)
			}

			nodeIPSpec := nodeIP.(*k8s.NodeIP).TypedSpec()

			nodeIPsString := slices.Map(nodeIPSpec.Addresses, netip.Addr.String)
			args["node-ip"] = strings.Join(nodeIPsString, ",")
		}

		if err = args.Merge(extraArgs, argsbuilder.WithMergePolicies(
			argsbuilder.MergePolicies{
				"bootstrap-kubeconfig":       argsbuilder.MergeDenied,
				"kubeconfig":                 argsbuilder.MergeDenied,
				"container-runtime":          argsbuilder.MergeDenied,
				"container-runtime-endpoint": argsbuilder.MergeDenied,
				"config":                     argsbuilder.MergeDenied,
				"cert-dir":                   argsbuilder.MergeDenied,
			},
		)); err != nil {
			return fmt.Errorf("error merging arguments: %w", err)
		}

		kubeletConfig, err := NewKubeletConfiguration(cfgSpec)
		if err != nil {
			return fmt.Errorf("error creating kubelet configuration: %w", err)
		}

		// If our platform is container, we cannot rely on the ability to change kernel parameters.
		// Therefore, we need to NOT attempt to enforce the kernel parameter checking done by the kubelet
		// when the `ProtectKernelDefaults` setting is enabled.
		if ctrl.V1Alpha1Mode == v1alpha1runtime.ModeContainer {
			kubeletConfig.ProtectKernelDefaults = false
		}

		unstructuredConfig, err := runtime.DefaultUnstructuredConverter.ToUnstructured(kubeletConfig)
		if err != nil {
			return fmt.Errorf("error converting to unstructured: %w", err)
		}

		if err = r.Modify(
			ctx,
			k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID),
			func(r resource.Resource) error {
				kubeletSpec := r.(*k8s.KubeletSpec).TypedSpec()

				kubeletSpec.Image = cfgSpec.Image
				kubeletSpec.ExtraMounts = cfgSpec.ExtraMounts
				kubeletSpec.Args = args.Args()
				kubeletSpec.Config = unstructuredConfig
				kubeletSpec.ExpectedNodename = expectedNodename

				return nil
			},
		); err != nil {
			return fmt.Errorf("error modifying KubeletSpec resource: %w", err)
		}
	}
}

func prepareExtraConfig(extraConfig map[string]interface{}) (*kubeletconfig.KubeletConfiguration, error) {
	// check for fields that can't be overridden via extraConfig
	var multiErr *multierror.Error

	for _, field := range kubelet.ProtectedConfigurationFields {
		if _, exists := extraConfig[field]; exists {
			multiErr = multierror.Append(multiErr, fmt.Errorf("field %q can't be overridden", field))
		}
	}

	if err := multiErr.ErrorOrNil(); err != nil {
		return nil, err
	}

	var config kubeletconfig.KubeletConfiguration

	// unmarshal extra config into the config structure
	// as unmarshalling zeroes the missing fields, we can't do that after setting the defaults
	if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(extraConfig, &config, true); err != nil {
		return nil, fmt.Errorf("error unmarshalling extra kubelet configuration: %w", err)
	}

	return &config, nil
}

// NewKubeletConfiguration builds kubelet configuration with defaults and overrides from extraConfig.
//
//nolint:gocyclo,cyclop
func NewKubeletConfiguration(cfgSpec *k8s.KubeletConfigSpec) (*kubeletconfig.KubeletConfiguration, error) {
	config, err := prepareExtraConfig(cfgSpec.ExtraConfig)
	if err != nil {
		return nil, err
	}

	// required fields (always set)
	config.TypeMeta = metav1.TypeMeta{
		APIVersion: kubeletconfig.SchemeGroupVersion.String(),
		Kind:       "KubeletConfiguration",
	}

	if cfgSpec.DisableManifestsDirectory {
		config.StaticPodPath = ""
	} else {
		config.StaticPodPath = constants.ManifestsDirectory
	}

	config.StaticPodURL = cfgSpec.StaticPodListURL
	config.Port = constants.KubeletPort
	config.Authentication = kubeletconfig.KubeletAuthentication{
		X509: kubeletconfig.KubeletX509Authentication{
			ClientCAFile: constants.KubernetesCACert,
		},
		Webhook: kubeletconfig.KubeletWebhookAuthentication{
			Enabled: pointer.To(true),
		},
		Anonymous: kubeletconfig.KubeletAnonymousAuthentication{
			Enabled: pointer.To(false),
		},
	}
	config.Authorization = kubeletconfig.KubeletAuthorization{
		Mode: kubeletconfig.KubeletAuthorizationModeWebhook,
	}
	config.CgroupRoot = "/"
	config.SystemCgroups = constants.CgroupSystem
	config.KubeletCgroups = constants.CgroupKubelet
	config.RotateCertificates = true
	config.ProtectKernelDefaults = true

	if cfgSpec.DefaultRuntimeSeccompEnabled {
		config.SeccompDefault = pointer.To(true)
		if config.FeatureGates != nil {
			if defaultRuntimeSeccompProfileEnabled, overridden := config.FeatureGates["SeccompDefault"]; overridden && !defaultRuntimeSeccompProfileEnabled {
				config.FeatureGates["SeccompDefault"] = true
			}
		} else {
			config.FeatureGates = map[string]bool{
				"SeccompDefault": true,
			}
		}
	}

	if cfgSpec.SkipNodeRegistration {
		config.Authentication.Webhook.Enabled = pointer.To(false)
		config.Authorization.Mode = kubeletconfig.KubeletAuthorizationModeAlwaysAllow
	}

	// fields which can be overridden
	if config.Address == "" {
		config.Address = "0.0.0.0"
	}

	if config.OOMScoreAdj == nil {
		config.OOMScoreAdj = pointer.To[int32](constants.KubeletOOMScoreAdj)
	}

	if config.ClusterDomain == "" {
		config.ClusterDomain = cfgSpec.ClusterDomain
	}

	if len(config.ClusterDNS) == 0 {
		config.ClusterDNS = cfgSpec.ClusterDNS
	}

	if config.SerializeImagePulls == nil {
		config.SerializeImagePulls = pointer.To(false)
	}

	if config.FailSwapOn == nil {
		config.FailSwapOn = pointer.To(false)
	}

	if len(config.SystemReserved) == 0 {
		config.SystemReserved = map[string]string{
			"cpu":               constants.KubeletSystemReservedCPU,
			"memory":            constants.KubeletSystemReservedMemory,
			"pid":               constants.KubeletSystemReservedPid,
			"ephemeral-storage": constants.KubeletSystemReservedEphemeralStorage,
		}
	}

	if config.Logging.Format == "" {
		config.Logging.Format = "json"
	}

	extraConfig := cfgSpec.ExtraConfig

	if _, overridden := extraConfig["shutdownGracePeriod"]; !overridden && config.ShutdownGracePeriod.Duration == 0 {
		config.ShutdownGracePeriod = metav1.Duration{Duration: constants.KubeletShutdownGracePeriod}
	}

	if _, overridden := extraConfig["shutdownGracePeriodCriticalPods"]; !overridden && config.ShutdownGracePeriodCriticalPods.Duration == 0 {
		config.ShutdownGracePeriodCriticalPods = metav1.Duration{Duration: constants.KubeletShutdownGracePeriodCriticalPods}
	}

	if config.StreamingConnectionIdleTimeout.Duration == 0 {
		config.StreamingConnectionIdleTimeout = metav1.Duration{Duration: 5 * time.Minute}
	}

	if config.TLSMinVersion == "" {
		config.TLSMinVersion = "VersionTLS13"
	}

	return config, nil
}
