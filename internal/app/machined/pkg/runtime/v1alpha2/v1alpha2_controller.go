// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha2

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	osruntime "github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/talos-systems/go-procfs/procfs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/config"
	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/files"
	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network"
	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/perf"
	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/secrets"
	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/time"
	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/v1alpha1"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	configresource "github.com/talos-systems/talos/pkg/resources/config"
)

// Controller implements runtime.V1alpha2Controller.
type Controller struct {
	controllerRuntime *osruntime.Runtime
	consoleLogLevel   zap.AtomicLevel
	logger            *zap.Logger

	v1alpha1Runtime runtime.Runtime
}

// NewController creates Controller.
func NewController(v1alpha1Runtime runtime.Runtime, loggingManager runtime.LoggingManager) (*Controller, error) {
	ctrl := &Controller{
		v1alpha1Runtime: v1alpha1Runtime,
		consoleLogLevel: zap.NewAtomicLevel(),
	}

	logWriter, err := loggingManager.ServiceLog("controller-runtime").Writer()
	if err != nil {
		return nil, err
	}

	ctrl.logger = logging.ZapLogger(
		logging.NewLogDestination(logWriter, zapcore.DebugLevel, logging.WithColoredLevels()),
		logging.NewLogDestination(logging.StdWriter, ctrl.consoleLogLevel, logging.WithoutTimestamp(), logging.WithoutLogLevels()),
	).With(logging.Component("controller-runtime"))

	ctrl.controllerRuntime, err = osruntime.NewRuntime(v1alpha1Runtime.State().V1Alpha2().Resources(), ctrl.logger)

	return ctrl, err
}

// Run the controller runtime.
func (ctrl *Controller) Run(ctx context.Context) error {
	// adjust the log level based on machine configuration
	go ctrl.watchMachineConfig(ctx, ctrl.logger)

	for _, c := range []controller.Controller{
		&v1alpha1.BootstrapStatusController{},
		&v1alpha1.ServiceController{
			// V1Events
			V1Alpha1Events: ctrl.v1alpha1Runtime.Events(),
		},
		&time.SyncController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&config.MachineTypeController{},
		&config.K8sControlPlaneController{},
		&files.EtcFileController{
			EtcPath:    "/etc",
			ShadowPath: constants.SystemEtcPath,
		},
		&k8s.ControlPlaneStaticPodController{},
		&k8s.EndpointController{},
		&k8s.ExtraManifestController{},
		&k8s.KubeletStaticPodController{},
		&k8s.ManifestController{},
		&k8s.ManifestApplyController{},
		&k8s.NodenameController{},
		&k8s.RenderSecretsStaticPodController{},
		&network.AddressConfigController{
			Cmdline:      procfs.ProcCmdline(),
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&network.AddressMergeController{},
		&network.AddressSpecController{},
		&network.AddressStatusController{},
		&network.EtcFileController{},
		&network.HostnameConfigController{
			Cmdline: procfs.ProcCmdline(),
		},
		&network.HostnameMergeController{},
		&network.HostnameSpecController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&network.LinkConfigController{
			Cmdline: procfs.ProcCmdline(),
		},
		&network.LinkMergeController{},
		&network.LinkStatusController{},
		&network.LinkSpecController{},
		&network.NodeAddressController{},
		&network.OperatorConfigController{
			Cmdline: procfs.ProcCmdline(),
		},
		&network.OperatorSpecController{
			V1alpha1Platform: ctrl.v1alpha1Runtime.State().Platform(),
			State:            ctrl.v1alpha1Runtime.State().V1Alpha2().Resources(),
		},
		&network.PlatformConfigController{
			V1alpha1Platform: ctrl.v1alpha1Runtime.State().Platform(),
		},
		&network.ResolverConfigController{
			Cmdline: procfs.ProcCmdline(),
		},
		&network.ResolverMergeController{},
		&network.ResolverSpecController{},
		&network.RouteConfigController{
			Cmdline: procfs.ProcCmdline(),
		},
		&network.RouteMergeController{},
		&network.RouteStatusController{},
		&network.RouteSpecController{},
		&network.StatusController{},
		&network.TimeServerConfigController{
			Cmdline: procfs.ProcCmdline(),
		},
		&network.TimeServerMergeController{},
		&perf.StatsController{},
		&network.TimeServerSpecController{},
		&secrets.APIController{},
		&secrets.EtcdController{},
		&secrets.KubernetesController{},
		&secrets.RootController{},
	} {
		if err := ctrl.controllerRuntime.RegisterController(c); err != nil {
			return err
		}
	}

	return ctrl.controllerRuntime.Run(ctx)
}

// DependencyGraph returns controller-resources dependencies.
func (ctrl *Controller) DependencyGraph() (*controller.DependencyGraph, error) {
	return ctrl.controllerRuntime.GetDependencyGraph()
}

func (ctrl *Controller) watchMachineConfig(ctx context.Context, logger *zap.Logger) {
	watchCh := make(chan state.Event)

	if err := ctrl.v1alpha1Runtime.State().V1Alpha2().Resources().Watch(
		ctx,
		resource.NewMetadata(configresource.NamespaceName, configresource.MachineConfigType, configresource.V1Alpha1ID, resource.VersionUndefined),
		watchCh,
	); err != nil {
		logger.Warn("error watching machine configuration", zap.Error(err))

		return
	}

	for {
		logLevel := zapcore.InfoLevel

		select {
		case event := <-watchCh:
			if event.Type != state.Destroyed {
				if event.Resource.(*configresource.MachineConfig).Config().Debug() {
					logLevel = zapcore.DebugLevel
				}
			}
		case <-ctx.Done():
			return
		}

		if ctrl.consoleLogLevel.Level() != logLevel {
			ctrl.consoleLogLevel.SetLevel(logLevel)

			ctrl.logger.Info("setting console log level", zap.Stringer("level", logLevel))
		}
	}
}
