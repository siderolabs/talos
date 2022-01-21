// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"strings"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/component-base/config/v1alpha1"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"

	"github.com/talos-systems/talos/pkg/argsbuilder"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
)

// KubeletSpecController renders manifests based on templates and config/secrets.
type KubeletSpecController struct{}

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
			ID:        pointer.ToString(k8s.KubeletID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodenameType,
			ID:        pointer.ToString(k8s.NodenameID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodeIPType,
			ID:        pointer.ToString(k8s.KubeletID),
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

		args := argsbuilder.Args{
			"bootstrap-kubeconfig":       constants.KubeletBootstrapKubeconfig,
			"kubeconfig":                 constants.KubeletKubeconfig,
			"container-runtime":          "remote",
			"container-runtime-endpoint": "unix://" + constants.CRIContainerdAddress,
			"config":                     "/etc/kubernetes/kubelet.yaml",

			"cert-dir": constants.KubeletPKIDir,

			"hostname-override": nodenameSpec.Nodename,
		}

		if cfgSpec.CloudProviderExternal {
			args["cloud-provider"] = "external"
		}

		extraArgs := argsbuilder.Args(cfgSpec.ExtraArgs)

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

			nodeIPsString := make([]string, len(nodeIPSpec.Addresses))

			for i := range nodeIPSpec.Addresses {
				nodeIPsString[i] = nodeIPSpec.Addresses[i].String()
			}

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

		kubeletConfig := newKubeletConfiguration(cfgSpec.ClusterDNS, cfgSpec.ClusterDomain)

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

				return nil
			},
		); err != nil {
			return fmt.Errorf("error modifying KubeletSpec resource: %w", err)
		}
	}
}

func newKubeletConfiguration(clusterDNS []string, dnsDomain string) *kubeletconfig.KubeletConfiguration {
	return &kubeletconfig.KubeletConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kubelet.config.k8s.io/v1beta1",
			Kind:       "KubeletConfiguration",
		},
		StaticPodPath:      constants.ManifestsDirectory,
		Address:            "0.0.0.0",
		Port:               constants.KubeletPort,
		OOMScoreAdj:        pointer.ToInt32(constants.KubeletOOMScoreAdj),
		RotateCertificates: true,
		Authentication: kubeletconfig.KubeletAuthentication{
			X509: kubeletconfig.KubeletX509Authentication{
				ClientCAFile: constants.KubernetesCACert,
			},
			Webhook: kubeletconfig.KubeletWebhookAuthentication{
				Enabled: pointer.ToBool(true),
			},
			Anonymous: kubeletconfig.KubeletAnonymousAuthentication{
				Enabled: pointer.ToBool(false),
			},
		},
		Authorization: kubeletconfig.KubeletAuthorization{
			Mode: kubeletconfig.KubeletAuthorizationModeWebhook,
		},
		ClusterDomain:       dnsDomain,
		ClusterDNS:          clusterDNS,
		SerializeImagePulls: pointer.ToBool(false),
		FailSwapOn:          pointer.ToBool(false),
		CgroupRoot:          "/",
		SystemCgroups:       constants.CgroupSystem,
		SystemReserved: map[string]string{
			"cpu":               constants.KubeletSystemReservedCPU,
			"memory":            constants.KubeletSystemReservedMemory,
			"pid":               constants.KubeletSystemReservedPid,
			"ephemeral-storage": constants.KubeletSystemReservedEphemeralStorage,
		},
		KubeletCgroups: constants.CgroupKubelet,
		Logging: v1alpha1.LoggingConfiguration{
			Format: "json",
		},
		TLSMinVersion: "VersionTLS13",
	}
}
