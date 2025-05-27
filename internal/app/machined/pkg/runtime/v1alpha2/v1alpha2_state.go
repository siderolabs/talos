// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha2

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/registry"

	talosconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubeaccess"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/perf"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	"github.com/siderolabs/talos/pkg/machinery/resources/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/resources/time"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// State implements runtime.V1alpha2State interface.
type State struct {
	resources state.State

	namespaceRegistry *registry.NamespaceRegistry
	resourceRegistry  *registry.ResourceRegistry
}

// NewState creates State.
func NewState() (*State, error) {
	s := &State{}

	ctx := context.TODO()

	s.resources = state.WrapCore(namespaced.NewState(
		func(ns string) state.CoreState {
			return inmem.NewStateWithOptions(
				inmem.WithHistoryInitialCapacity(8),
				inmem.WithHistoryMaxCapacity(1024),
				inmem.WithHistoryGap(4),
			)(ns)
		},
	))
	s.namespaceRegistry = registry.NewNamespaceRegistry(s.resources)
	s.resourceRegistry = registry.NewResourceRegistry(s.resources)

	if err := s.namespaceRegistry.RegisterDefault(ctx); err != nil {
		return nil, err
	}

	if err := s.resourceRegistry.RegisterDefault(ctx); err != nil {
		return nil, err
	}

	// register Talos namespaces
	for _, ns := range []struct {
		name        string
		description string
	}{
		{v1alpha1.NamespaceName, "Talos v1alpha1 subsystems glue resources."},
		{cluster.NamespaceName, "Cluster configuration and discovery resources."},
		{cluster.RawNamespaceName, "Cluster unmerged raw resources."},
		{config.NamespaceName, "Talos node configuration."},
		{etcd.NamespaceName, "etcd resources."},
		{files.NamespaceName, "Files and file-like resources."},
		{hardware.NamespaceName, "Hardware resources."},
		{k8s.NamespaceName, "Kubernetes all node types resources."},
		{k8s.ControlPlaneNamespaceName, "Kubernetes control plane resources."},
		{kubespan.NamespaceName, "KubeSpan resources."},
		{network.NamespaceName, "Networking resources."},
		{network.ConfigNamespaceName, "Networking configuration resources."},
		{cri.NamespaceName, "CRI Seccomp resources."},
		{secrets.NamespaceName, "Resources with secret material."},
		{perf.NamespaceName, "Stats resources."},
	} {
		if err := s.namespaceRegistry.Register(ctx, ns.name, ns.description); err != nil {
			return nil, err
		}
	}

	// register Talos resources
	for _, r := range []meta.ResourceWithRD{
		&block.Device{},
		&block.DiscoveredVolume{},
		&block.DiscoveryRefreshRequest{},
		&block.DiscoveryRefreshStatus{},
		&block.Disk{},
		&block.MountRequest{},
		&block.MountStatus{},
		&block.SwapStatus{},
		&block.Symlink{},
		&block.SystemDisk{},
		&block.UserDiskConfigStatus{},
		&block.VolumeConfig{},
		&block.VolumeLifecycle{},
		&block.VolumeMountRequest{},
		&block.VolumeMountStatus{},
		&block.VolumeStatus{},
		&cluster.Affiliate{},
		&cluster.Config{},
		&cluster.Identity{},
		&cluster.Info{},
		&cluster.Member{},
		&config.MachineConfig{},
		&config.MachineType{},
		&cri.ImageCacheConfig{},
		&cri.SeccompProfile{},
		&etcd.Config{},
		&etcd.PKIStatus{},
		&etcd.Spec{},
		&etcd.Member{},
		&files.EtcFileSpec{},
		&files.EtcFileStatus{},
		&hardware.MemoryModule{},
		&hardware.PCIDevice{},
		&hardware.PCIDriverRebindConfig{},
		&hardware.PCIDriverRebindStatus{},
		&hardware.PCRStatus{},
		&hardware.Processor{},
		&hardware.SystemInformation{},
		&k8s.AdmissionControlConfig{},
		&k8s.AuditPolicyConfig{},
		&k8s.AuthorizationConfig{},
		&k8s.APIServerConfig{},
		&k8s.KubePrismEndpoints{},
		&k8s.ConfigStatus{},
		&k8s.ControllerManagerConfig{},
		&k8s.Endpoint{},
		&k8s.ExtraManifestsConfig{},
		&k8s.KubeletConfig{},
		&k8s.KubeletLifecycle{},
		&k8s.KubeletSpec{},
		&k8s.KubePrismConfig{},
		&k8s.KubePrismStatuses{},
		&k8s.Manifest{},
		&k8s.ManifestStatus{},
		&k8s.BootstrapManifestsConfig{},
		&k8s.NodeAnnotationSpec{},
		&k8s.NodeCordonedSpec{},
		&k8s.NodeIP{},
		&k8s.NodeIPConfig{},
		&k8s.NodeLabelSpec{},
		&k8s.Nodename{},
		&k8s.NodeStatus{},
		&k8s.NodeTaintSpec{},
		&k8s.SchedulerConfig{},
		&k8s.StaticPod{},
		&k8s.StaticPodServerStatus{},
		&k8s.StaticPodStatus{},
		&k8s.SecretsStatus{},
		&kubeaccess.Config{},
		&kubespan.Config{},
		&kubespan.Endpoint{},
		&kubespan.Identity{},
		&kubespan.PeerSpec{},
		&kubespan.PeerStatus{},
		&network.AddressStatus{},
		&network.AddressSpec{},
		&network.DeviceConfigSpec{},
		&network.DNSResolveCache{},
		&network.DNSUpstream{},
		&network.EthernetSpec{},
		&network.EthernetStatus{},
		&network.HardwareAddr{},
		&network.HostDNSConfig{},
		&network.HostnameStatus{},
		&network.HostnameSpec{},
		&network.LinkRefresh{},
		&network.LinkStatus{},
		&network.LinkSpec{},
		&network.NfTablesChain{},
		&network.NodeAddress{},
		&network.NodeAddressFilter{},
		&network.NodeAddressSortAlgorithm{},
		&network.OperatorSpec{},
		&network.ProbeSpec{},
		&network.ProbeStatus{},
		&network.ResolverStatus{},
		&network.ResolverSpec{},
		&network.RouteStatus{},
		&network.RouteSpec{},
		&network.Status{},
		&network.TimeServerStatus{},
		&network.TimeServerSpec{},
		&perf.CPU{},
		&perf.Memory{},
		&cri.RegistriesConfig{},
		&runtime.DevicesStatus{},
		&runtime.Diagnostic{},
		&runtime.EventSinkConfig{},
		&runtime.ExtensionServiceConfig{},
		&runtime.ExtensionServiceConfigStatus{},
		&runtime.ExtensionStatus{},
		&runtime.KernelModuleSpec{},
		&runtime.KernelParamSpec{},
		&runtime.KernelParamDefaultSpec{},
		&runtime.KernelParamStatus{},
		&runtime.KmsgLogConfig{},
		&runtime.MaintenanceServiceConfig{},
		&runtime.MaintenanceServiceRequest{},
		&runtime.MachineResetSignal{},
		&runtime.MachineStatus{},
		&runtime.MetaKey{},
		&runtime.MetaLoaded{},
		&runtime.MountStatus{},
		&runtime.PlatformMetadata{},
		&runtime.SecurityState{},
		&runtime.UniqueMachineToken{},
		&runtime.Version{},
		&runtime.WatchdogTimerConfig{},
		&runtime.WatchdogTimerStatus{},
		&secrets.API{},
		&secrets.CertSAN{},
		&secrets.Etcd{},
		&secrets.EtcdRoot{},
		&secrets.Kubelet{},
		&secrets.Kubernetes{},
		&secrets.KubernetesDynamicCerts{},
		&secrets.KubernetesRoot{},
		&secrets.MaintenanceServiceCerts{},
		&secrets.MaintenanceRoot{},
		&secrets.OSRoot{},
		&secrets.Trustd{},
		&siderolink.Config{},
		&siderolink.Status{},
		&siderolink.Tunnel{},
		&time.AdjtimeStatus{},
		&time.Status{},
		&v1alpha1.AcquireConfigSpec{},
		&v1alpha1.AcquireConfigStatus{},
		&v1alpha1.Service{},
	} {
		if err := s.resourceRegistry.Register(ctx, r); err != nil {
			return nil, err
		}
	}

	return s, nil
}

// Resources implements runtime.V1alpha2State interface.
func (s *State) Resources() state.State {
	return s.resources
}

// NamespaceRegistry implements runtime.V1alpha2State interface.
func (s *State) NamespaceRegistry() *registry.NamespaceRegistry {
	return s.namespaceRegistry
}

// ResourceRegistry implements runtime.V1alpha2State interface.
func (s *State) ResourceRegistry() *registry.ResourceRegistry {
	return s.resourceRegistry
}

// GetConfig implements runtime.V1alpha2State interface.
func (s *State) GetConfig(ctx context.Context) (talosconfig.Provider, error) {
	cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, s.resources, config.ActiveID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, err
	}

	return cfg.Provider(), nil
}

// SetConfig implements runtime.V1alpha2State interface.
func (s *State) SetConfig(ctx context.Context, id string, cfg talosconfig.Provider) error {
	cfgResource := config.NewMachineConfigWithID(cfg, id)

	oldCfg, err := s.resources.Get(ctx, cfgResource.Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return s.resources.Create(ctx, cfgResource)
		}

		return err
	}

	cfgResource.Metadata().SetVersion(oldCfg.Metadata().Version())

	return s.resources.Update(ctx, cfgResource)
}
