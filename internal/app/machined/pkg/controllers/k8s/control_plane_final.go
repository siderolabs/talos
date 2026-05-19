// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/transform"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	schedulerv1 "k8s.io/kube-scheduler/config/v1"

	"github.com/siderolabs/talos/pkg/argsbuilder"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// ControlPlaneSchedulerFinalController manages final k8s.SchedulerConfig.
type ControlPlaneSchedulerFinalController = transform.Controller[*k8s.SchedulerConfig, *k8s.SchedulerConfig]

// NewControlPlaneSchedulerFinalController instanciates the controller.
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

				outCfg := runtime.DeepCopyJSON(in.TypedSpec().Config)
				if outCfg == nil {
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
