// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pelletier/go-toml/v2"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-kubernetes/kubernetes/compatibility"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"

	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/cgroup"
	"github.com/siderolabs/talos/pkg/argsbuilder"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/kubelet"
	"github.com/siderolabs/talos/pkg/machinery/labels"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

var kubeletSELinuxMounts = []specs.Mount{
	{Type: "bind", Destination: "/sys/fs/selinux", Source: "/sys/fs/selinux", Options: []string{"bind", "rw"}},
	{Type: "bind", Destination: "/usr/share/containers/selinux", Source: "/usr/share/containers/selinux", Options: []string{"bind", "ro"}},
}

// KubeletSpecController renders manifests based on templates and config/secrets.
type KubeletSpecController struct {
	V1Alpha1Mode v1alpha1runtime.Mode

	// MemoryCapacity is read at render time while a kubepods memory limit is active; defaults to ProcMemoryCapacity.
	MemoryCapacity MemoryCapacityReader
}

// Name implements controller.Controller interface.
func (ctrl *KubeletSpecController) Name() string {
	return "k8s.KubeletSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KubeletSpecController) Inputs() []controller.Input {
	return slices.Concat([]controller.Input{
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
	}, []controller.Input{
		{Namespace: files.NamespaceName, Type: files.EtcFileSpecType, ID: optional.Some(constants.CRIConfig), Kind: controller.InputWeak},
		{Namespace: runtimeres.NamespaceName, Type: runtimeres.SecurityStateType, ID: optional.Some(runtimeres.SecurityStateID), Kind: controller.InputWeak},
	})
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
func (ctrl *KubeletSpecController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
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
			"config":            argsbuilder.Value{"/etc/kubernetes/kubelet.yaml"},
			"cert-dir":          argsbuilder.Value{constants.KubeletPKIDir},
			"hostname-override": argsbuilder.Value{expectedNodename},
		}

		if !cfgSpec.SkipNodeRegistration {
			args["bootstrap-kubeconfig"] = argsbuilder.Value{constants.KubeletBootstrapKubeconfig}
			args["kubeconfig"] = argsbuilder.Value{constants.KubeletKubeconfig}
		}

		if cfgSpec.CloudProviderExternal {
			// we still need to specify `--cloud-provider=external` for the kubelet
			// to get the node properly tainted so that it gets picked up by the external CCM
			args["cloud-provider"] = argsbuilder.Value{CloudProviderExternal}
		}

		if !kubeletVersion.SupportsKubeletConfigContainerRuntimeEndpoint() {
			args["container-runtime-endpoint"] = argsbuilder.Value{constants.CRIContainerdAddress}
		}

		extraArgs := make(argsbuilder.Args, len(cfgSpec.ExtraArgs))
		for k, v := range cfgSpec.ExtraArgs {
			extraArgs[k] = v.Values
		}

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
			args["node-ip"] = argsbuilder.Value{strings.Join(nodeIPsString, ",")} // NOTE: flag has string type, cannot be multiple
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
			args["image-credential-provider-bin-dir"] = argsbuilder.Value{constants.KubeletCredentialProviderBinDir}
			args["image-credential-provider-config"] = argsbuilder.Value{constants.KubeletCredentialProviderConfig}
		}

		kubeletConfig, err := NewKubeletConfiguration(cfgSpec, kubeletVersion, machineType.MachineType(), ctrl.kubeletConfigurationOptions(cfgSpec)...)
		if err != nil {
			return fmt.Errorf("error creating kubelet configuration: %w", err)
		}

		// If our platform is container, we cannot rely on the ability to change kernel parameters.
		// Therefore, we need to NOT attempt to enforce the kernel parameter checking done by the kubelet
		// when the `ProtectKernelDefaults` setting is enabled.
		if ctrl.V1Alpha1Mode == v1alpha1runtime.ModeContainer {
			kubeletConfig.ProtectKernelDefaults = false
		}

		extraMounts := cfgSpec.ExtraMounts

		if selinuxMounts, err := ctrl.criLabelsContainers(ctx, r); err != nil {
			return err
		} else if selinuxMounts {
			extraMounts = append(slices.Clone(extraMounts), kubeletSELinuxMounts...)
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
				kubeletSpec.ExtraMounts = extraMounts
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

// kubeletConfigurationOptions wires the host memory reader in when a kubepods memory limit is active outside container mode.
//
// In container mode the kubelet does not own the host cgroup hierarchy, so the limit leaves its configuration unchanged.
func (ctrl *KubeletSpecController) kubeletConfigurationOptions(cfgSpec *k8s.KubeletConfigSpec) []KubeletConfigurationOption {
	if cfgSpec.KubepodsMemoryLimit == 0 {
		return nil
	}

	if ctrl.V1Alpha1Mode == v1alpha1runtime.ModeContainer {
		return []KubeletConfigurationOption{IgnoringKubepodsMemoryLimit()}
	}

	readCapacity := ctrl.MemoryCapacity
	if readCapacity == nil {
		readCapacity = ProcMemoryCapacity
	}

	return []KubeletConfigurationOption{WithHostMemory(readCapacity, os.Getpagesize())}
}

// criLabelsContainers reports whether the CRI labels containers, in which case the kubelet needs selinuxfs and the contexts file.
func (ctrl *KubeletSpecController) criLabelsContainers(ctx context.Context, r controller.Reader) (bool, error) {
	securityState, err := safe.ReaderGetByID[*runtimeres.SecurityState](ctx, r, runtimeres.SecurityStateID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("error getting security state: %w", err)
	}

	if securityState.TypedSpec().SELinuxState == runtimeres.SELinuxStateDisabled {
		return false, nil
	}

	criConfig, err := safe.ReaderGetByID[*files.EtcFileSpec](ctx, r, constants.CRIConfig)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("error getting CRI config: %w", err)
	}

	var cfg struct {
		Plugins map[string]struct {
			EnableSELinux bool `toml:"enable_selinux"`
		} `toml:"plugins"`
	}

	if err = toml.Unmarshal(criConfig.TypedSpec().Contents, &cfg); err != nil {
		return false, fmt.Errorf("error parsing CRI config: %w", err)
	}

	for _, plugin := range cfg.Plugins {
		if plugin.EnableSELinux {
			return true, nil
		}
	}

	return false, nil
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
// A kubepods memory limit requires WithHostMemory unless IgnoringKubepodsMemoryLimit is given;
// the conflicting raw settings are rejected before the Talos defaults are applied.
//
//nolint:gocyclo,cyclop
func NewKubeletConfiguration(
	cfgSpec *k8s.KubeletConfigSpec, kubeletVersion compatibility.Version, machineType machine.Type, opts ...KubeletConfigurationOption,
) (*kubeletconfig.KubeletConfiguration, error) {
	var options kubeletConfigurationOptions

	for _, opt := range opts {
		opt(&options)
	}

	kubepodsMemoryLimit := cfgSpec.KubepodsMemoryLimit
	if options.ignoreKubepodsMemoryLimit {
		kubepodsMemoryLimit = 0
	}

	if kubepodsMemoryLimit > 0 {
		if options.hostMemory == nil {
			return nil, errors.New("kubepods memory limit requires the host memory capacity")
		}

		extraArgs := maps.Map(cfgSpec.ExtraArgs, func(flag string, v k8s.ArgValues) (string, []string) { return flag, v.Values })

		if err := kubelet.ValidateMemoryLimitConfiguration(cfgSpec.ExtraConfig, extraArgs); err != nil {
			return nil, err
		}
	}

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
			Enabled: new(true),
		},
		Anonymous: kubeletconfig.KubeletAnonymousAuthentication{
			Enabled: new(false),
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
		config.SeccompDefault = new(true)
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
		config.Authentication.Webhook.Enabled = new(false)
		config.Authorization.Mode = kubeletconfig.KubeletAuthorizationModeAlwaysAllow
	} else if cfgSpec.ExtraArgs["register-with-taints"].Values == nil { // / don't clash with taints provided via extraArgs, it is deprecated on kubelet side
		// register with taint to prevent scheduling on control plane nodes race with NodeApplyController applying the initial taint
		// NodeApplyController will take ownership of the taint after the first successful apply
		for key, taint := range cfgSpec.RegisterWithTaints {
			value, effect := labels.ParseTaint(taint)

			if slices.IndexFunc(config.RegisterWithTaints, func(t corev1.Taint) bool {
				return t.Key == key
			}) == -1 { // don't add the taint if it's already in the config
				config.RegisterWithTaints = append(
					config.RegisterWithTaints,
					corev1.Taint{
						Key:    key,
						Effect: corev1.TaintEffect(effect),
						Value:  value,
					},
				)
			}
		}

		// sort the taints to establish stable order
		slices.SortFunc(config.RegisterWithTaints, func(a, b corev1.Taint) int {
			return cmp.Compare(a.Key, b.Key)
		})
	}

	// fields which can be overridden
	if config.Address == "" {
		config.Address = "0.0.0.0"
	}

	if config.OOMScoreAdj == nil {
		config.OOMScoreAdj = new(int32(constants.KubeletOOMScoreAdj))
	}

	if config.ClusterDomain == "" {
		config.ClusterDomain = cfgSpec.ClusterDomain
	}

	if len(config.ClusterDNS) == 0 {
		config.ClusterDNS = cfgSpec.ClusterDNS
	}

	if config.SerializeImagePulls == nil {
		config.SerializeImagePulls = new(false)
	}

	if config.FailSwapOn == nil {
		config.FailSwapOn = new(false)
	}

	// kubelet requires low < high, so only default both if neither is overridden
	if config.ImageGCHighThresholdPercent == nil && config.ImageGCLowThresholdPercent == nil {
		config.ImageGCHighThresholdPercent = new(int32(constants.KubeletImageGCHighThresholdPercent))
		config.ImageGCLowThresholdPercent = new(int32(constants.KubeletImageGCLowThresholdPercent))
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

	if kubepodsMemoryLimit > 0 {
		reservation, err := kubepodsMemoryReservation(config, kubepodsMemoryLimit, *options.hostMemory)
		if err != nil {
			return nil, err
		}

		config.SystemReserved["memory"] = strconv.FormatUint(reservation, 10)
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

	config.ResolverConfig = new(constants.PodResolvConfPath)

	return config, nil
}
