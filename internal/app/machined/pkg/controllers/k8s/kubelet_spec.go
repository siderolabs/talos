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
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-kubernetes/kubernetes/compatibility"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"

	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/cgroup"
	"github.com/siderolabs/talos/pkg/argsbuilder"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/kubelet"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
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
			ID:        optional.Some(k8s.KubeletID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodenameType,
			ID:        optional.Some(k8s.NodenameID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodeIPType,
			ID:        optional.Some(k8s.KubeletID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineTypeType,
			ID:        optional.Some(config.MachineTypeID),
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
//nolint:gocyclo,cyclop
func (ctrl *KubeletSpecController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*k8s.KubeletConfig](ctx, r, k8s.KubeletID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		cfgSpec := cfg.TypedSpec()

		kubeletVersion := compatibility.VersionFromImageRef(cfgSpec.Image)

		machineType, err := safe.ReaderGetByID[*config.MachineType](ctx, r, config.MachineTypeID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting machine type: %w", err)
		}

		nodename, err := safe.ReaderGetByID[*k8s.Nodename](ctx, r, k8s.NodenameID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting nodename: %w", err)
		}

		expectedNodename := nodename.TypedSpec().Nodename

		args := argsbuilder.Args{
			"config": "/etc/kubernetes/kubelet.yaml",

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

		if !kubeletVersion.SupportsKubeletConfigContainerRuntimeEndpoint() {
			args["container-runtime-endpoint"] = constants.CRIContainerdAddress
		}

		extraArgs := argsbuilder.Args(cfgSpec.ExtraArgs)

		// if the user supplied a hostname override, we do not manage it anymore
		if extraArgs.Contains("hostname-override") {
			expectedNodename = ""
		}

		// if the user supplied node-ip via extra args, no need to pick automatically
		if !extraArgs.Contains("node-ip") {
			nodeIP, nodeErr := safe.ReaderGetByID[*k8s.NodeIP](ctx, r, k8s.KubeletID)
			if nodeErr != nil {
				if state.IsNotFoundError(nodeErr) {
					continue
				}

				return fmt.Errorf("error getting node IPs: %w", nodeErr)
			}

			nodeIPsString := xslices.Map(nodeIP.TypedSpec().Addresses, netip.Addr.String)
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

		// these flags are present from v1.24
		if cfgSpec.CredentialProviderConfig != nil {
			args["image-credential-provider-bin-dir"] = constants.KubeletCredentialProviderBinDir
			args["image-credential-provider-config"] = constants.KubeletCredentialProviderConfig
		}

		kubeletConfig, err := NewKubeletConfiguration(cfgSpec, kubeletVersion, machineType.MachineType())
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

		if err = safe.WriterModify(
			ctx,
			r,
			k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID),
			func(r *k8s.KubeletSpec) error {
				kubeletSpec := r.TypedSpec()

				kubeletSpec.Image = cfgSpec.Image
				kubeletSpec.ExtraMounts = cfgSpec.ExtraMounts
				kubeletSpec.Args = args.Args()
				kubeletSpec.Config = unstructuredConfig
				kubeletSpec.ExpectedNodename = expectedNodename
				kubeletSpec.CredentialProviderConfig = cfgSpec.CredentialProviderConfig

				return nil
			},
		); err != nil {
			return fmt.Errorf("error modifying KubeletSpec resource: %w", err)
		}

		r.ResetRestartBackoff()
	}
}

func prepareExtraConfig(extraConfig map[string]any) (*kubeletconfig.KubeletConfiguration, error) {
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
func NewKubeletConfiguration(cfgSpec *k8s.KubeletConfigSpec, kubeletVersion compatibility.Version, machineType machine.Type) (*kubeletconfig.KubeletConfiguration, error) {
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
	config.CgroupRoot = cgroup.Root()
	config.SystemCgroups = cgroup.Path(constants.CgroupSystem)
	config.KubeletCgroups = cgroup.Path(constants.CgroupKubelet)
	config.RotateCertificates = true
	config.ProtectKernelDefaults = true

	if kubeletVersion.SupportsKubeletConfigContainerRuntimeEndpoint() {
		config.ContainerRuntimeEndpoint = "unix://" + constants.CRIContainerdAddress
	}

	if cfgSpec.DefaultRuntimeSeccompEnabled {
		config.SeccompDefault = pointer.To(true)
	}

	if cfgSpec.EnableFSQuotaMonitoring {
		if _, overridden := config.FeatureGates["LocalStorageCapacityIsolationFSQuotaMonitoring"]; !overridden {
			if config.FeatureGates == nil {
				config.FeatureGates = map[string]bool{}
			}

			config.FeatureGates["LocalStorageCapacityIsolationFSQuotaMonitoring"] = true
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
			"pid":               constants.KubeletSystemReservedPid,
			"ephemeral-storage": constants.KubeletSystemReservedEphemeralStorage,
		}

		if machineType.IsControlPlane() {
			config.SystemReserved["memory"] = constants.KubeletSystemReservedMemoryControlPlane
		} else {
			config.SystemReserved["memory"] = constants.KubeletSystemReservedMemoryWorker
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

	config.ResolverConfig = pointer.To(constants.PodResolvConfPath)

	return config, nil
}
