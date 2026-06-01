// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/transform"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-kubernetes/kubernetes/compatibility"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	schedulerv1 "k8s.io/kube-scheduler/config/v1"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s/internal/k8sjson"
	"github.com/siderolabs/talos/pkg/argsbuilder"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// ControlPlaneControllerManagerFinalController manages final k8s.ControllerManagerConfig.
type ControlPlaneControllerManagerFinalController = transform.Controller[*k8s.ControllerManagerConfig, *k8s.ControllerManagerConfig]

// NewControlPlaneControllerManagerFinalController instantiates the controller.
func NewControlPlaneControllerManagerFinalController() *ControlPlaneControllerManagerFinalController {
	return transform.NewController(
		transform.Settings[*k8s.ControllerManagerConfig, *k8s.ControllerManagerConfig]{
			Name: "k8s.ControlPlaneControllerManagerFinalController",
			MapMetadataOptionalFunc: func(in *k8s.ControllerManagerConfig) optional.Optional[*k8s.ControllerManagerConfig] {
				if in.Metadata().ID() != k8s.ControllerManagerConfigID {
					return optional.None[*k8s.ControllerManagerConfig]()
				}

				return optional.Some(k8s.NewControllerManagerConfig(k8s.FinalControllerManagerConfigID))
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, in *k8s.ControllerManagerConfig, out *k8s.ControllerManagerConfig) error {
				// clear the spec
				*out.TypedSpec() = k8s.ControllerManagerConfigSpec{}
				out.TypedSpec().Enabled = in.TypedSpec().Enabled

				if !in.TypedSpec().Enabled {
					return nil
				}

				out.TypedSpec().Image = in.TypedSpec().Image
				out.TypedSpec().ExtraVolumes = in.TypedSpec().ExtraVolumes
				out.TypedSpec().EnvironmentVariables = in.TypedSpec().EnvironmentVariables
				out.TypedSpec().Resources = in.TypedSpec().Resources

				args := []string{ //nolint:prealloc // very dynamic length
					"/usr/local/bin/kube-controller-manager",
					"--use-service-account-credentials",
				}

				builder := argsbuilder.Args{
					"allocate-node-cidrs":              {"true"},
					"bind-address":                     {"127.0.0.1"},
					"cluster-cidr":                     {strings.Join(in.TypedSpec().PodCIDRs, ",")},
					"service-cluster-ip-range":         {strings.Join(in.TypedSpec().ServiceCIDRs, ",")},
					"cluster-signing-cert-file":        {filepath.Join(constants.KubernetesControllerManagerSecretsDir, "ca.crt")},
					"cluster-signing-key-file":         {filepath.Join(constants.KubernetesControllerManagerSecretsDir, "ca.key")},
					"controllers":                      {"*"},
					"configure-cloud-routes":           {"false"},
					"kubeconfig":                       {filepath.Join(constants.KubernetesControllerManagerSecretsDir, "kubeconfig")},
					"authentication-kubeconfig":        {filepath.Join(constants.KubernetesControllerManagerSecretsDir, "kubeconfig")},
					"authorization-kubeconfig":         {filepath.Join(constants.KubernetesControllerManagerSecretsDir, "kubeconfig")},
					"leader-elect":                     {"true"},
					"root-ca-file":                     {filepath.Join(constants.KubernetesControllerManagerSecretsDir, "ca.crt")},
					"service-account-private-key-file": {filepath.Join(constants.KubernetesControllerManagerSecretsDir, "service-account.key")},
					"profiling":                        {"false"},
					"terminated-pod-gc-threshold":      {"100"},
					"tls-min-version":                  {"VersionTLS13"},
					"use-service-account-credentials":  {"true"},
				}

				k8sVersion := compatibility.VersionFromImageRef(in.TypedSpec().Image)

				if in.TypedSpec().CloudProvider != "" && !k8sVersion.CloudProviderFlagRemoved() {
					builder.Set("cloud-provider", argsbuilder.Value{in.TypedSpec().CloudProvider})
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

				extraArgs := make(argsbuilder.Args, len(in.TypedSpec().ExtraArgs))
				for k, v := range in.TypedSpec().ExtraArgs {
					extraArgs[k] = v.Values
				}

				if err := builder.Merge(extraArgs, argsbuilder.WithMergePolicies(mergePolicies)); err != nil {
					return fmt.Errorf("failed to build final args: %w", err)
				}

				out.TypedSpec().Args = slices.Concat(args, builder.Args())

				return nil
			},
		},
		transform.WithOutputKind(controller.OutputShared),
	)
}

// ControlPlaneSchedulerFinalController manages final k8s.SchedulerConfig.
type ControlPlaneSchedulerFinalController = transform.Controller[*k8s.SchedulerConfig, *k8s.SchedulerConfig]

// NewControlPlaneSchedulerFinalController instantiates the controller.
func NewControlPlaneSchedulerFinalController() *ControlPlaneSchedulerFinalController {
	return transform.NewController(
		transform.Settings[*k8s.SchedulerConfig, *k8s.SchedulerConfig]{
			Name: "k8s.ControlPlaneSchedulerFinalController",
			MapMetadataOptionalFunc: func(in *k8s.SchedulerConfig) optional.Optional[*k8s.SchedulerConfig] {
				if in.Metadata().ID() != k8s.SchedulerConfigID {
					return optional.None[*k8s.SchedulerConfig]()
				}

				return optional.Some(k8s.NewSchedulerConfig(k8s.FinalSchedulerConfigID))
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, in *k8s.SchedulerConfig, out *k8s.SchedulerConfig) error {
				// clear the spec
				*out.TypedSpec() = k8s.SchedulerConfigSpec{}
				out.TypedSpec().Enabled = in.TypedSpec().Enabled

				if !in.TypedSpec().Enabled {
					return nil
				}

				out.TypedSpec().Image = in.TypedSpec().Image
				out.TypedSpec().ExtraVolumes = in.TypedSpec().ExtraVolumes
				out.TypedSpec().EnvironmentVariables = in.TypedSpec().EnvironmentVariables
				out.TypedSpec().Resources = in.TypedSpec().Resources

				args := []string{ //nolint:prealloc // very dynamic length
					"/usr/local/bin/kube-scheduler",
				}

				builder := argsbuilder.Args{
					"config":                                 {filepath.Join(constants.KubernetesSchedulerConfigDir, "scheduler-config.yaml")},
					"authentication-tolerate-lookup-failure": {"false"},
					"authentication-kubeconfig":              {filepath.Join(constants.KubernetesSchedulerSecretsDir, "kubeconfig")},
					"authorization-kubeconfig":               {filepath.Join(constants.KubernetesSchedulerSecretsDir, "kubeconfig")},
					"bind-address":                           {"127.0.0.1"},
					"leader-elect":                           {"true"},
					"profiling":                              {"false"},
					"tls-min-version":                        {"VersionTLS13"},
				}

				mergePolicies := argsbuilder.MergePolicies{
					"kubeconfig":                argsbuilder.MergeDenied,
					"authentication-kubeconfig": argsbuilder.MergeDenied,
					"authorization-kubeconfig":  argsbuilder.MergeDenied,
					"config":                    argsbuilder.MergeDenied,
				}

				extraArgs := make(argsbuilder.Args, len(in.TypedSpec().ExtraArgs))
				for k, v := range in.TypedSpec().ExtraArgs {
					extraArgs[k] = v.Values
				}

				if err := builder.Merge(extraArgs, argsbuilder.WithMergePolicies(mergePolicies)); err != nil {
					return fmt.Errorf("failed to produce final kube-scheduler args: %w", err)
				}

				out.TypedSpec().Args = slices.Concat(args, builder.Args())

				// Validate against the typed schema, but emit the user-provided map so
				// fields the user didn't set don't leak into the YAML as zero values —
				// older Kubernetes releases reject keys they don't know about.
				var cfg schedulerv1.KubeSchedulerConfiguration

				if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(in.TypedSpec().Config, &cfg, false); err != nil {
					return fmt.Errorf("error unmarshaling scheduler configuration: %w", err)
				}

				outCfg, ok := k8sjson.DeepCopyToJSON(in.TypedSpec().Config).(map[string]any)
				if !ok || outCfg == nil {
					outCfg = map[string]any{}
				}

				outCfg["apiVersion"] = "kubescheduler.config.k8s.io/v1"
				outCfg["kind"] = "KubeSchedulerConfiguration"

				clientConn, _ := outCfg["clientConnection"].(map[string]any)
				if clientConn == nil {
					clientConn = map[string]any{}
					outCfg["clientConnection"] = clientConn
				}

				clientConn["kubeconfig"] = filepath.Join(constants.KubernetesSchedulerSecretsDir, "kubeconfig")

				out.TypedSpec().Config = outCfg

				return nil
			},
		},
		transform.WithOutputKind(controller.OutputShared),
	)
}
