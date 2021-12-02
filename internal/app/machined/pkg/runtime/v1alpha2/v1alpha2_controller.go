// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha2

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	osruntime "github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/talos-systems/go-procfs/procfs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/cluster"
	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/config"
	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/files"
	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/kubespan"
	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network"
	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/perf"
	runtimecontrollers "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/secrets"
	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/siderolink"
	timecontrollers "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/time"
	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/v1alpha1"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	runtimelogging "github.com/talos-systems/talos/internal/app/machined/pkg/runtime/logging"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/pkg/logging"
	talosconfig "github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	configresource "github.com/talos-systems/talos/pkg/machinery/resources/config"
)

// Controller implements runtime.V1alpha2Controller.
type Controller struct {
	controllerRuntime *osruntime.Runtime

	loggingManager  runtime.LoggingManager
	consoleLogLevel zap.AtomicLevel
	logger          *zap.Logger

	v1alpha1Runtime runtime.Runtime
}

// NewController creates Controller.
func NewController(v1alpha1Runtime runtime.Runtime) (*Controller, error) {
	ctrl := &Controller{
		consoleLogLevel: zap.NewAtomicLevel(),
		loggingManager:  v1alpha1Runtime.Logging(),
		v1alpha1Runtime: v1alpha1Runtime,
	}

	logWriter, err := ctrl.loggingManager.ServiceLog("controller-runtime").Writer()
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
func (ctrl *Controller) Run(ctx context.Context, drainer *runtime.Drainer) error {
	// adjust the log level based on machine configuration
	go ctrl.watchMachineConfig(ctx)

	for _, c := range []controller.Controller{
		&v1alpha1.ServiceController{
			// V1Events
			V1Alpha1Events: ctrl.v1alpha1Runtime.Events(),
		},
		&timecontrollers.SyncController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&cluster.AffiliateMergeController{},
		&cluster.ConfigController{},
		&cluster.DiscoveryServiceController{},
		&cluster.EndpointController{},
		&cluster.LocalAffiliateController{},
		&cluster.MemberController{},
		&cluster.KubernetesPullController{},
		&cluster.KubernetesPushController{},
		&cluster.NodeIdentityController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&config.MachineTypeController{},
		&config.K8sAddressFilterController{},
		&config.K8sControlPlaneController{},
		&files.EtcFileController{
			EtcPath:    "/etc",
			ShadowPath: constants.SystemEtcPath,
		},
		&k8s.ControlPlaneStaticPodController{},
		&k8s.EndpointController{},
		&k8s.ExtraManifestController{},
		&k8s.KubeletConfigController{},
		&k8s.KubeletServiceController{
			V1Alpha1Services: system.Services(ctrl.v1alpha1Runtime),
		},
		&k8s.KubeletSpecController{},
		&k8s.KubeletStaticPodController{},
		&k8s.ManifestController{},
		&k8s.ManifestApplyController{},
		&k8s.NodeIPController{},
		&k8s.NodeIPConfigController{},
		&k8s.NodenameController{},
		&k8s.RenderSecretsStaticPodController{},
		&kubespan.ConfigController{},
		&kubespan.EndpointController{},
		&kubespan.IdentityController{},
		&kubespan.ManagerController{},
		&kubespan.PeerSpecController{},
		&network.AddressConfigController{
			Cmdline:      procfs.ProcCmdline(),
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&network.AddressEventController{
			V1Alpha1Events: ctrl.v1alpha1Runtime.Events(),
		},
		&network.AddressMergeController{},
		&network.AddressSpecController{},
		&network.AddressStatusController{},
		&network.EtcFileController{},
		&network.HardwareAddrController{},
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
		&network.TimeServerSpecController{},
		&perf.StatsController{},
		&runtimecontrollers.EventsSinkController{
			V1Alpha1Events: ctrl.v1alpha1Runtime.Events(),
			Cmdline:        procfs.ProcCmdline(),
			Drainer:        drainer,
		},
		&runtimecontrollers.KernelParamConfigController{},
		&runtimecontrollers.KernelParamDefaultsController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&runtimecontrollers.KernelParamSpecController{},
		&runtimecontrollers.KmsgLogDeliveryController{
			Cmdline: procfs.ProcCmdline(),
			Drainer: drainer,
		},
		&secrets.APIController{},
		&secrets.APICertSANsController{},
		&secrets.EtcdController{},
		&secrets.KubeletController{},
		&secrets.KubernetesController{},
		&secrets.KubernetesCertSANsController{},
		&secrets.RootController{},
		&siderolink.ManagerController{
			Cmdline: procfs.ProcCmdline(),
		},
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

func (ctrl *Controller) watchMachineConfig(ctx context.Context) {
	watchCh := make(chan state.Event)

	if err := ctrl.v1alpha1Runtime.State().V1Alpha2().Resources().Watch(
		ctx,
		resource.NewMetadata(configresource.NamespaceName, configresource.MachineConfigType, configresource.V1Alpha1ID, resource.VersionUndefined),
		watchCh,
	); err != nil {
		ctrl.logger.Warn("error watching machine configuration", zap.Error(err))

		return
	}

	var loggingEndpoints []*url.URL

	for {
		var cfg talosconfig.Provider
		select {
		case event := <-watchCh:
			if event.Type == state.Destroyed {
				continue
			}

			cfg = event.Resource.(*configresource.MachineConfig).Config()

		case <-ctx.Done():
			return
		}

		ctrl.updateConsoleLoggingConfig(cfg)
		ctrl.updateLoggingConfig(ctx, cfg, &loggingEndpoints)
	}
}

func (ctrl *Controller) updateConsoleLoggingConfig(cfg talosconfig.Provider) {
	newLogLevel := zapcore.InfoLevel
	if cfg.Debug() {
		newLogLevel = zapcore.DebugLevel
	}

	if newLogLevel != ctrl.consoleLogLevel.Level() {
		ctrl.logger.Info("setting console log level", zap.Stringer("level", newLogLevel))
		ctrl.consoleLogLevel.SetLevel(newLogLevel)
	}
}

func (ctrl *Controller) updateLoggingConfig(ctx context.Context, cfg talosconfig.Provider, prevLoggingEndpoints *[]*url.URL) {
	dests := cfg.Machine().Logging().Destinations()
	loggingEndpoints := make([]*url.URL, len(dests))

	for i, dest := range dests {
		switch f := dest.Format(); f {
		case constants.LoggingFormatJSONLines:
			loggingEndpoints[i] = dest.Endpoint()
		default:
			// should not be possible due to validation
			panic(fmt.Sprintf("unhandled log destination format %q", f))
		}
	}

	loggingChanged := len(*prevLoggingEndpoints) != len(loggingEndpoints)
	if !loggingChanged {
		for i, u := range *prevLoggingEndpoints {
			if u.String() != loggingEndpoints[i].String() {
				loggingChanged = true

				break
			}
		}
	}

	if !loggingChanged {
		return
	}

	*prevLoggingEndpoints = loggingEndpoints

	var prevSenders []runtime.LogSender

	if len(loggingEndpoints) > 0 {
		senders := make([]runtime.LogSender, len(loggingEndpoints))
		for i, u := range loggingEndpoints {
			senders[i] = runtimelogging.NewJSONLines(u)
		}

		ctrl.logger.Info("enabling JSON logging")
		prevSenders = ctrl.loggingManager.SetSenders(senders)
	} else {
		ctrl.logger.Info("disabling JSON logging")
		prevSenders = ctrl.loggingManager.SetSenders(nil)
	}

	closeCtx, closeCancel := context.WithTimeout(ctx, 3*time.Second)
	defer closeCancel()

	var wg sync.WaitGroup

	for _, sender := range prevSenders {
		sender := sender

		wg.Add(1)

		go func() {
			defer wg.Done()

			err := sender.Close(closeCtx)
			ctrl.logger.Info("log sender closed", zap.Error(err))
		}()
	}

	wg.Wait()
}
