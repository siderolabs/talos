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
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-procfs/procfs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/cluster"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/config"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/cri"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/etcd"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/files"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hardware"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/kubeaccess"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/kubespan"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/perf"
	runtimecontrollers "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/secrets"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/siderolink"
	timecontrollers "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/time"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/v1alpha1"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	runtimelogging "github.com/siderolabs/talos/internal/app/machined/pkg/runtime/logging"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/pkg/logging"
	talosconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	configresource "github.com/siderolabs/talos/pkg/machinery/resources/config"
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

	var err error

	ctrl.logger, err = ctrl.MakeLogger("controller-runtime")
	if err != nil {
		return nil, err
	}

	ctrl.controllerRuntime, err = osruntime.NewRuntime(v1alpha1Runtime.State().V1Alpha2().Resources(), ctrl.logger)

	return ctrl, err
}

// Run the controller runtime.
func (ctrl *Controller) Run(ctx context.Context, drainer *runtime.Drainer) error {
	// adjust the log level based on machine configuration
	go ctrl.watchMachineConfig(ctx)

	dnsCacheLogger, err := ctrl.MakeLogger("dns-resolve-cache")
	if err != nil {
		return err
	}

	for _, c := range []controller.Controller{
		&block.DevicesController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&block.DiscoveryController{},
		&block.DisksController{},
		&block.LVMActivationController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&block.MountController{},
		&block.MountRequestController{},
		&block.MountStatusController{},
		&block.SwapStatusController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&block.SymlinksController{},
		&block.SystemDiskController{},
		&block.UserDiskConfigController{},
		&block.UserVolumeConfigController{},
		&block.VolumeConfigController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&block.VolumeManagerController{},
		&cluster.AffiliateMergeController{},
		cluster.NewConfigController(),
		&cluster.DiscoveryServiceController{},
		&cluster.EndpointController{},
		cluster.NewInfoController(),
		&cluster.KubernetesPullController{},
		&cluster.KubernetesPushController{},
		&cluster.LocalAffiliateController{},
		&cluster.MemberController{},
		&cluster.NodeIdentityController{},
		&config.AcquireController{
			PlatformConfiguration: &platformConfigurator{
				platform: ctrl.v1alpha1Runtime.State().Platform(),
				state:    ctrl.v1alpha1Runtime.State().V1Alpha2().Resources(),
			},
			PlatformEvent: &platformEventer{
				platform: ctrl.v1alpha1Runtime.State().Platform(),
			},
			Mode:           ctrl.v1alpha1Runtime.State().Platform().Mode(),
			CmdlineGetter:  procfs.ProcCmdline,
			ConfigSetter:   ctrl.v1alpha1Runtime,
			EventPublisher: ctrl.v1alpha1Runtime.Events(),
			ValidationMode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
			ResourceState:  ctrl.v1alpha1Runtime.State().V1Alpha2().Resources(),
		},
		&config.MachineTypeController{},
		&config.PersistenceController{},
		&cri.ImageCacheConfigController{
			V1Alpha1ServiceManager: system.Services(ctrl.v1alpha1Runtime),
		},
		&cri.RegistriesConfigController{},
		&cri.SeccompProfileController{},
		&cri.SeccompProfileFileController{
			V1Alpha1Mode:             ctrl.v1alpha1Runtime.State().Platform().Mode(),
			SeccompProfilesDirectory: constants.SeccompProfilesDirectory,
		},
		&etcd.AdvertisedPeerController{},
		etcd.NewConfigController(),
		&etcd.PKIController{},
		&etcd.SpecController{},
		&etcd.MemberController{},
		&files.CRIBaseRuntimeSpecController{},
		&files.CRIConfigPartsController{},
		&files.CRIRegistryConfigController{},
		&files.EtcFileController{
			EtcPath:    "/etc",
			ShadowPath: constants.SystemEtcPath,
		},
		&files.IQNController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&files.NQNController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&hardware.PCIDevicesController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&hardware.PCIDriverRebindConfigController{},
		&hardware.PCIDriverRebindController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&hardware.PCRStatusController{},
		&hardware.SystemInfoController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&k8s.AddressFilterController{},
		k8s.NewControlPlaneAPIServerController(),
		k8s.NewControlPlaneAdmissionControlController(),
		k8s.NewControlPlaneAuditPolicyController(),
		k8s.NewControlPlaneAuthorizationController(),
		k8s.NewControlPlaneBootstrapManifestsController(),
		k8s.NewControlPlaneControllerManagerController(),
		k8s.NewControlPlaneExtraManifestsController(),
		k8s.NewControlPlaneSchedulerController(),
		&k8s.ControlPlaneStaticPodController{},
		&k8s.EndpointController{},
		&k8s.ExtraManifestController{},
		k8s.NewKubeletConfigController(),
		&k8s.KubeletServiceController{
			V1Alpha1Services: system.Services(ctrl.v1alpha1Runtime),
			V1Alpha1Mode:     ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&k8s.KubeletSpecController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&k8s.KubeletStaticPodController{},
		k8s.NewKubePrismEndpointsController(),
		k8s.NewKubePrismConfigController(),
		&k8s.KubePrismController{},
		&k8s.ManifestApplyController{},
		&k8s.ManifestController{},
		k8s.NewNodeIPConfigController(),
		&k8s.NodeIPController{},
		&k8s.NodeAnnotationSpecController{},
		&k8s.NodeApplyController{},
		&k8s.NodeCordonedSpecController{},
		&k8s.NodeLabelSpecController{},
		&k8s.NodeStatusController{},
		&k8s.NodeTaintSpecController{},
		&k8s.NodenameController{},
		&k8s.RenderConfigsStaticPodController{},
		&k8s.RenderSecretsStaticPodController{},
		&k8s.StaticEndpointController{},
		&k8s.StaticPodConfigController{},
		&k8s.StaticPodServerController{},
		kubeaccess.NewConfigController(),
		&kubeaccess.CRDController{},
		&kubeaccess.EndpointController{},
		kubespan.NewConfigController(),
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
		network.NewAddressMergeController(),
		&network.AddressSpecController{},
		&network.AddressStatusController{},
		&network.DeviceConfigController{},
		&network.DNSResolveCacheController{
			State:  ctrl.v1alpha1Runtime.State().V1Alpha2().Resources(),
			Logger: dnsCacheLogger,
		},
		&network.DNSUpstreamController{},
		&network.EtcFileController{
			PodResolvConfPath: constants.PodResolvConfPath,
			V1Alpha1Mode:      ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&network.EthernetConfigController{},
		&network.EthernetSpecController{},
		&network.EthernetStatusController{},
		&network.HardwareAddrController{},
		&network.HostDNSConfigController{},
		&network.HostnameConfigController{
			Cmdline: procfs.ProcCmdline(),
		},
		network.NewHostnameMergeController(),
		&network.HostnameSpecController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&network.LinkConfigController{
			Cmdline: procfs.ProcCmdline(),
		},
		network.NewLinkMergeController(),
		&network.LinkSpecController{},
		&network.LinkStatusController{},
		&network.NfTablesChainConfigController{},
		&network.NfTablesChainController{},
		&network.NodeAddressController{},
		&network.NodeAddressSortAlgorithmController{},
		&network.OperatorConfigController{
			Cmdline: procfs.ProcCmdline(),
		},
		network.NewOperatorMergeController(),
		&network.OperatorSpecController{
			V1alpha1Platform: ctrl.v1alpha1Runtime.State().Platform(),
			State:            ctrl.v1alpha1Runtime.State().V1Alpha2().Resources(),
		},
		&network.OperatorVIPConfigController{
			Cmdline: procfs.ProcCmdline(),
		},
		&network.PlatformConfigController{
			V1alpha1Platform: ctrl.v1alpha1Runtime.State().Platform(),
			PlatformState:    ctrl.v1alpha1Runtime.State().V1Alpha2().Resources(),
		},
		&network.ProbeController{},
		&network.ResolverConfigController{
			Cmdline: procfs.ProcCmdline(),
		},
		network.NewResolverMergeController(),
		&network.ResolverSpecController{},
		&network.RouteConfigController{
			Cmdline: procfs.ProcCmdline(),
		},
		network.NewRouteMergeController(),
		&network.RouteSpecController{},
		&network.RouteStatusController{},
		&network.StatusController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&network.TimeServerConfigController{
			Cmdline: procfs.ProcCmdline(),
		},
		network.NewTimeServerMergeController(),
		&network.TimeServerSpecController{},
		&perf.StatsController{},
		&runtimecontrollers.CRIImageGCController{},
		&runtimecontrollers.DevicesStatusController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&runtimecontrollers.DiagnosticsController{},
		&runtimecontrollers.DiagnosticsLoggerController{},
		&runtimecontrollers.DropUpgradeFallbackController{
			MetaProvider: ctrl.v1alpha1Runtime.State().Machine(),
		},
		&runtimecontrollers.ExtensionServiceConfigController{},
		&runtimecontrollers.ExtensionServiceConfigFilesController{
			V1Alpha1Mode:            ctrl.v1alpha1Runtime.State().Platform().Mode(),
			ExtensionsConfigBaseDir: constants.ExtensionServiceUserConfigPath,
		},
		&runtimecontrollers.EventsSinkConfigController{
			Cmdline:      procfs.ProcCmdline(),
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&runtimecontrollers.EventsSinkController{
			V1Alpha1Events: ctrl.v1alpha1Runtime.Events(),
			Drainer:        drainer,
		},
		&runtimecontrollers.ExtensionServiceController{
			V1Alpha1Services: system.Services(ctrl.v1alpha1Runtime),
			ConfigPath:       constants.ExtensionServiceConfigPath,
		},
		&runtimecontrollers.ExtensionStatusController{},
		&runtimecontrollers.KernelModuleConfigController{},
		&runtimecontrollers.KernelModuleSpecController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&runtimecontrollers.KernelParamConfigController{},
		&runtimecontrollers.KernelParamDefaultsController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&runtimecontrollers.KernelParamSpecController{},
		&runtimecontrollers.KmsgLogConfigController{
			Cmdline: procfs.ProcCmdline(),
		},
		&runtimecontrollers.KmsgLogDeliveryController{
			Drainer: drainer,
		},
		&runtimecontrollers.MaintenanceConfigController{},
		&runtimecontrollers.MaintenanceServiceController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&runtimecontrollers.MachineStatusController{
			V1Alpha1Events: ctrl.v1alpha1Runtime.Events(),
		},
		&runtimecontrollers.MachineStatusPublisherController{
			V1Alpha1Events: ctrl.v1alpha1Runtime.Events(),
		},
		&runtimecontrollers.MountStatusController{},
		&runtimecontrollers.SecurityStateController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&runtimecontrollers.UniqueMachineTokenController{},
		&runtimecontrollers.VersionController{},
		&runtimecontrollers.WatchdogTimerConfigController{},
		&runtimecontrollers.WatchdogTimerController{},
		&secrets.APICertSANsController{},
		&secrets.APIController{},
		&secrets.EtcdController{},
		secrets.NewKubeletController(),
		&secrets.KubernetesCertSANsController{},
		&secrets.KubernetesDynamicCertsController{},
		&secrets.KubernetesController{},
		&secrets.MaintenanceController{},
		&secrets.MaintenanceCertSANsController{},
		&secrets.MaintenanceRootController{},
		secrets.NewRootEtcdController(),
		secrets.NewRootKubernetesController(),
		secrets.NewRootOSController(),
		&secrets.TrustedRootsController{},
		&secrets.TrustdController{},
		&siderolink.ConfigController{
			Cmdline:      procfs.ProcCmdline(),
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&siderolink.ManagerController{},
		&siderolink.StatusController{},
		&siderolink.UserspaceWireguardController{
			RelayRetryTimeout: 10 * time.Second,
		},
		&timecontrollers.AdjtimeStatusController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&timecontrollers.SyncController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&v1alpha1.ServiceController{
			V1Alpha1Events: ctrl.v1alpha1Runtime.Events(),
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

type loggingDestination struct {
	Format    string
	Endpoint  *url.URL
	ExtraTags map[string]string
}

func (a *loggingDestination) Equal(b *loggingDestination) bool {
	if a.Format != b.Format {
		return false
	}

	if a.Endpoint.String() != b.Endpoint.String() {
		return false
	}

	if len(a.ExtraTags) != len(b.ExtraTags) {
		return false
	}

	for k, v := range a.ExtraTags {
		if vv, ok := b.ExtraTags[k]; !ok || vv != v {
			return false
		}
	}

	return true
}

func (ctrl *Controller) watchMachineConfig(ctx context.Context) {
	watchCh := make(chan state.Event)

	if err := ctrl.v1alpha1Runtime.State().V1Alpha2().Resources().Watch(
		ctx,
		resource.NewMetadata(configresource.NamespaceName, configresource.MachineConfigType, configresource.ActiveID, resource.VersionUndefined),
		watchCh,
	); err != nil {
		ctrl.logger.Warn("error watching machine configuration", zap.Error(err))

		return
	}

	var loggingDestinations []loggingDestination

	for {
		var cfg talosconfig.Config
		select {
		case event := <-watchCh:
			if event.Type != state.Created && event.Type != state.Updated {
				continue
			}

			cfg = event.Resource.(*configresource.MachineConfig).Config()

		case <-ctx.Done():
			return
		}

		ctrl.updateConsoleLoggingConfig(cfg.Debug())

		if cfg.Machine() == nil {
			ctrl.updateLoggingConfig(ctx, nil, &loggingDestinations)
		} else {
			ctrl.updateLoggingConfig(ctx, cfg.Machine().Logging().Destinations(), &loggingDestinations)
		}
	}
}

func (ctrl *Controller) updateConsoleLoggingConfig(debug bool) {
	newLogLevel := zapcore.InfoLevel
	if debug {
		newLogLevel = zapcore.DebugLevel
	}

	if newLogLevel != ctrl.consoleLogLevel.Level() {
		ctrl.logger.Info("setting console log level", zap.Stringer("level", newLogLevel))
		ctrl.consoleLogLevel.SetLevel(newLogLevel)
	}
}

func (ctrl *Controller) updateLoggingConfig(ctx context.Context, dests []talosconfig.LoggingDestination, prevLoggingDestinations *[]loggingDestination) {
	loggingDestinations := make([]loggingDestination, len(dests))

	for i, dest := range dests {
		switch f := dest.Format(); f {
		case constants.LoggingFormatJSONLines:
			loggingDestinations[i] = loggingDestination{
				Format:    f,
				Endpoint:  dest.Endpoint(),
				ExtraTags: dest.ExtraTags(),
			}
		default:
			// should not be possible due to validation
			panic(fmt.Sprintf("unhandled log destination format %q", f))
		}
	}

	loggingChanged := len(*prevLoggingDestinations) != len(loggingDestinations)
	if !loggingChanged {
		for i, u := range *prevLoggingDestinations {
			if !u.Equal(&loggingDestinations[i]) {
				loggingChanged = true

				break
			}
		}
	}

	if !loggingChanged {
		return
	}

	*prevLoggingDestinations = loggingDestinations

	var prevSenders []runtime.LogSender

	if len(loggingDestinations) > 0 {
		senders := xslices.Map(dests, runtimelogging.NewJSONLines)

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
		wg.Add(1)

		go func() {
			defer wg.Done()

			err := sender.Close(closeCtx)
			ctrl.logger.Info("log sender closed", zap.Error(err))
		}()
	}

	wg.Wait()
}

// MakeLogger creates a logger for a service.
func (ctrl *Controller) MakeLogger(serviceName string) (*zap.Logger, error) {
	logWriter, err := ctrl.loggingManager.ServiceLog(serviceName).Writer()
	if err != nil {
		return nil, err
	}

	return logging.ZapLogger(
		logging.NewLogDestination(logWriter, zapcore.DebugLevel,
			logging.WithColoredLevels(),
		),
		logging.NewLogDestination(logging.StdWriter, ctrl.consoleLogLevel,
			logging.WithoutTimestamp(),
			logging.WithoutLogLevels(),
			logging.WithControllerErrorSuppressor(constants.ConsoleLogErrorSuppressThreshold),
		),
	).With(logging.Component(serviceName)), nil
}
