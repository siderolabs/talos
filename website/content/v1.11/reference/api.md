---
title: API
description: Talos gRPC API reference.
---

## Table of Contents

- [common/common.proto](#common/common.proto)
    - [Data](#common.Data)
    - [DataResponse](#common.DataResponse)
    - [Empty](#common.Empty)
    - [EmptyResponse](#common.EmptyResponse)
    - [Error](#common.Error)
    - [Metadata](#common.Metadata)
    - [NetIP](#common.NetIP)
    - [NetIPPort](#common.NetIPPort)
    - [NetIPPrefix](#common.NetIPPrefix)
    - [PEMEncodedCertificate](#common.PEMEncodedCertificate)
    - [PEMEncodedCertificateAndKey](#common.PEMEncodedCertificateAndKey)
    - [PEMEncodedKey](#common.PEMEncodedKey)
    - [URL](#common.URL)
  
    - [Code](#common.Code)
    - [ContainerDriver](#common.ContainerDriver)
    - [ContainerdNamespace](#common.ContainerdNamespace)
  
    - [File-level Extensions](#common/common.proto-extensions)
  
- [resource/definitions/block/block.proto](#resource/definitions/block/block.proto)
    - [DeviceSpec](#talos.resource.definitions.block.DeviceSpec)
    - [DiscoveredVolumeSpec](#talos.resource.definitions.block.DiscoveredVolumeSpec)
    - [DiscoveryRefreshRequestSpec](#talos.resource.definitions.block.DiscoveryRefreshRequestSpec)
    - [DiscoveryRefreshStatusSpec](#talos.resource.definitions.block.DiscoveryRefreshStatusSpec)
    - [DiskSelector](#talos.resource.definitions.block.DiskSelector)
    - [DiskSpec](#talos.resource.definitions.block.DiskSpec)
    - [EncryptionKey](#talos.resource.definitions.block.EncryptionKey)
    - [EncryptionSpec](#talos.resource.definitions.block.EncryptionSpec)
    - [FilesystemSpec](#talos.resource.definitions.block.FilesystemSpec)
    - [LocatorSpec](#talos.resource.definitions.block.LocatorSpec)
    - [MountRequestSpec](#talos.resource.definitions.block.MountRequestSpec)
    - [MountSpec](#talos.resource.definitions.block.MountSpec)
    - [MountStatusSpec](#talos.resource.definitions.block.MountStatusSpec)
    - [PartitionSpec](#talos.resource.definitions.block.PartitionSpec)
    - [ProvisioningSpec](#talos.resource.definitions.block.ProvisioningSpec)
    - [SwapStatusSpec](#talos.resource.definitions.block.SwapStatusSpec)
    - [SymlinkProvisioningSpec](#talos.resource.definitions.block.SymlinkProvisioningSpec)
    - [SymlinkSpec](#talos.resource.definitions.block.SymlinkSpec)
    - [SystemDiskSpec](#talos.resource.definitions.block.SystemDiskSpec)
    - [UserDiskConfigStatusSpec](#talos.resource.definitions.block.UserDiskConfigStatusSpec)
    - [VolumeConfigSpec](#talos.resource.definitions.block.VolumeConfigSpec)
    - [VolumeMountRequestSpec](#talos.resource.definitions.block.VolumeMountRequestSpec)
    - [VolumeMountStatusSpec](#talos.resource.definitions.block.VolumeMountStatusSpec)
    - [VolumeStatusSpec](#talos.resource.definitions.block.VolumeStatusSpec)
  
- [resource/definitions/cluster/cluster.proto](#resource/definitions/cluster/cluster.proto)
    - [AffiliateSpec](#talos.resource.definitions.cluster.AffiliateSpec)
    - [ConfigSpec](#talos.resource.definitions.cluster.ConfigSpec)
    - [ControlPlane](#talos.resource.definitions.cluster.ControlPlane)
    - [IdentitySpec](#talos.resource.definitions.cluster.IdentitySpec)
    - [InfoSpec](#talos.resource.definitions.cluster.InfoSpec)
    - [KubeSpanAffiliateSpec](#talos.resource.definitions.cluster.KubeSpanAffiliateSpec)
    - [MemberSpec](#talos.resource.definitions.cluster.MemberSpec)
  
- [resource/definitions/cri/cri.proto](#resource/definitions/cri/cri.proto)
    - [ImageCacheConfigSpec](#talos.resource.definitions.cri.ImageCacheConfigSpec)
    - [RegistriesConfigSpec](#talos.resource.definitions.cri.RegistriesConfigSpec)
    - [RegistriesConfigSpec.RegistryConfigEntry](#talos.resource.definitions.cri.RegistriesConfigSpec.RegistryConfigEntry)
    - [RegistriesConfigSpec.RegistryMirrorsEntry](#talos.resource.definitions.cri.RegistriesConfigSpec.RegistryMirrorsEntry)
    - [RegistryAuthConfig](#talos.resource.definitions.cri.RegistryAuthConfig)
    - [RegistryConfig](#talos.resource.definitions.cri.RegistryConfig)
    - [RegistryEndpointConfig](#talos.resource.definitions.cri.RegistryEndpointConfig)
    - [RegistryMirrorConfig](#talos.resource.definitions.cri.RegistryMirrorConfig)
    - [RegistryTLSConfig](#talos.resource.definitions.cri.RegistryTLSConfig)
    - [SeccompProfileSpec](#talos.resource.definitions.cri.SeccompProfileSpec)
  
- [resource/definitions/enums/enums.proto](#resource/definitions/enums/enums.proto)
    - [BlockEncryptionKeyType](#talos.resource.definitions.enums.BlockEncryptionKeyType)
    - [BlockEncryptionProviderType](#talos.resource.definitions.enums.BlockEncryptionProviderType)
    - [BlockFilesystemType](#talos.resource.definitions.enums.BlockFilesystemType)
    - [BlockVolumePhase](#talos.resource.definitions.enums.BlockVolumePhase)
    - [BlockVolumeType](#talos.resource.definitions.enums.BlockVolumeType)
    - [CriImageCacheCopyStatus](#talos.resource.definitions.enums.CriImageCacheCopyStatus)
    - [CriImageCacheStatus](#talos.resource.definitions.enums.CriImageCacheStatus)
    - [KubespanPeerState](#talos.resource.definitions.enums.KubespanPeerState)
    - [MachineType](#talos.resource.definitions.enums.MachineType)
    - [NethelpersADSelect](#talos.resource.definitions.enums.NethelpersADSelect)
    - [NethelpersARPAllTargets](#talos.resource.definitions.enums.NethelpersARPAllTargets)
    - [NethelpersARPValidate](#talos.resource.definitions.enums.NethelpersARPValidate)
    - [NethelpersAddressFlag](#talos.resource.definitions.enums.NethelpersAddressFlag)
    - [NethelpersAddressSortAlgorithm](#talos.resource.definitions.enums.NethelpersAddressSortAlgorithm)
    - [NethelpersBondMode](#talos.resource.definitions.enums.NethelpersBondMode)
    - [NethelpersBondXmitHashPolicy](#talos.resource.definitions.enums.NethelpersBondXmitHashPolicy)
    - [NethelpersConntrackState](#talos.resource.definitions.enums.NethelpersConntrackState)
    - [NethelpersDuplex](#talos.resource.definitions.enums.NethelpersDuplex)
    - [NethelpersFailOverMAC](#talos.resource.definitions.enums.NethelpersFailOverMAC)
    - [NethelpersFamily](#talos.resource.definitions.enums.NethelpersFamily)
    - [NethelpersLACPRate](#talos.resource.definitions.enums.NethelpersLACPRate)
    - [NethelpersLinkType](#talos.resource.definitions.enums.NethelpersLinkType)
    - [NethelpersMatchOperator](#talos.resource.definitions.enums.NethelpersMatchOperator)
    - [NethelpersNfTablesChainHook](#talos.resource.definitions.enums.NethelpersNfTablesChainHook)
    - [NethelpersNfTablesChainPriority](#talos.resource.definitions.enums.NethelpersNfTablesChainPriority)
    - [NethelpersNfTablesVerdict](#talos.resource.definitions.enums.NethelpersNfTablesVerdict)
    - [NethelpersOperationalState](#talos.resource.definitions.enums.NethelpersOperationalState)
    - [NethelpersPort](#talos.resource.definitions.enums.NethelpersPort)
    - [NethelpersPrimaryReselect](#talos.resource.definitions.enums.NethelpersPrimaryReselect)
    - [NethelpersProtocol](#talos.resource.definitions.enums.NethelpersProtocol)
    - [NethelpersRouteFlag](#talos.resource.definitions.enums.NethelpersRouteFlag)
    - [NethelpersRouteProtocol](#talos.resource.definitions.enums.NethelpersRouteProtocol)
    - [NethelpersRouteType](#talos.resource.definitions.enums.NethelpersRouteType)
    - [NethelpersRoutingTable](#talos.resource.definitions.enums.NethelpersRoutingTable)
    - [NethelpersScope](#talos.resource.definitions.enums.NethelpersScope)
    - [NethelpersVLANProtocol](#talos.resource.definitions.enums.NethelpersVLANProtocol)
    - [NetworkConfigLayer](#talos.resource.definitions.enums.NetworkConfigLayer)
    - [NetworkOperator](#talos.resource.definitions.enums.NetworkOperator)
    - [RuntimeMachineStage](#talos.resource.definitions.enums.RuntimeMachineStage)
    - [RuntimeSELinuxState](#talos.resource.definitions.enums.RuntimeSELinuxState)
  
- [resource/definitions/etcd/etcd.proto](#resource/definitions/etcd/etcd.proto)
    - [ConfigSpec](#talos.resource.definitions.etcd.ConfigSpec)
    - [ConfigSpec.ExtraArgsEntry](#talos.resource.definitions.etcd.ConfigSpec.ExtraArgsEntry)
    - [MemberSpec](#talos.resource.definitions.etcd.MemberSpec)
    - [PKIStatusSpec](#talos.resource.definitions.etcd.PKIStatusSpec)
    - [SpecSpec](#talos.resource.definitions.etcd.SpecSpec)
    - [SpecSpec.ExtraArgsEntry](#talos.resource.definitions.etcd.SpecSpec.ExtraArgsEntry)
  
- [resource/definitions/extensions/extensions.proto](#resource/definitions/extensions/extensions.proto)
    - [Compatibility](#talos.resource.definitions.extensions.Compatibility)
    - [Constraint](#talos.resource.definitions.extensions.Constraint)
    - [Layer](#talos.resource.definitions.extensions.Layer)
    - [Metadata](#talos.resource.definitions.extensions.Metadata)
  
- [resource/definitions/files/files.proto](#resource/definitions/files/files.proto)
    - [EtcFileSpecSpec](#talos.resource.definitions.files.EtcFileSpecSpec)
    - [EtcFileStatusSpec](#talos.resource.definitions.files.EtcFileStatusSpec)
  
- [resource/definitions/hardware/hardware.proto](#resource/definitions/hardware/hardware.proto)
    - [MemoryModuleSpec](#talos.resource.definitions.hardware.MemoryModuleSpec)
    - [PCIDeviceSpec](#talos.resource.definitions.hardware.PCIDeviceSpec)
    - [PCIDriverRebindConfigSpec](#talos.resource.definitions.hardware.PCIDriverRebindConfigSpec)
    - [PCIDriverRebindStatusSpec](#talos.resource.definitions.hardware.PCIDriverRebindStatusSpec)
    - [ProcessorSpec](#talos.resource.definitions.hardware.ProcessorSpec)
    - [SystemInformationSpec](#talos.resource.definitions.hardware.SystemInformationSpec)
  
- [resource/definitions/k8s/k8s.proto](#resource/definitions/k8s/k8s.proto)
    - [APIServerConfigSpec](#talos.resource.definitions.k8s.APIServerConfigSpec)
    - [APIServerConfigSpec.EnvironmentVariablesEntry](#talos.resource.definitions.k8s.APIServerConfigSpec.EnvironmentVariablesEntry)
    - [APIServerConfigSpec.ExtraArgsEntry](#talos.resource.definitions.k8s.APIServerConfigSpec.ExtraArgsEntry)
    - [AdmissionControlConfigSpec](#talos.resource.definitions.k8s.AdmissionControlConfigSpec)
    - [AdmissionPluginSpec](#talos.resource.definitions.k8s.AdmissionPluginSpec)
    - [AuditPolicyConfigSpec](#talos.resource.definitions.k8s.AuditPolicyConfigSpec)
    - [AuthorizationAuthorizersSpec](#talos.resource.definitions.k8s.AuthorizationAuthorizersSpec)
    - [AuthorizationConfigSpec](#talos.resource.definitions.k8s.AuthorizationConfigSpec)
    - [BootstrapManifestsConfigSpec](#talos.resource.definitions.k8s.BootstrapManifestsConfigSpec)
    - [ConfigStatusSpec](#talos.resource.definitions.k8s.ConfigStatusSpec)
    - [ControllerManagerConfigSpec](#talos.resource.definitions.k8s.ControllerManagerConfigSpec)
    - [ControllerManagerConfigSpec.EnvironmentVariablesEntry](#talos.resource.definitions.k8s.ControllerManagerConfigSpec.EnvironmentVariablesEntry)
    - [ControllerManagerConfigSpec.ExtraArgsEntry](#talos.resource.definitions.k8s.ControllerManagerConfigSpec.ExtraArgsEntry)
    - [EndpointSpec](#talos.resource.definitions.k8s.EndpointSpec)
    - [ExtraManifest](#talos.resource.definitions.k8s.ExtraManifest)
    - [ExtraManifest.ExtraHeadersEntry](#talos.resource.definitions.k8s.ExtraManifest.ExtraHeadersEntry)
    - [ExtraManifestsConfigSpec](#talos.resource.definitions.k8s.ExtraManifestsConfigSpec)
    - [ExtraVolume](#talos.resource.definitions.k8s.ExtraVolume)
    - [KubePrismConfigSpec](#talos.resource.definitions.k8s.KubePrismConfigSpec)
    - [KubePrismEndpoint](#talos.resource.definitions.k8s.KubePrismEndpoint)
    - [KubePrismEndpointsSpec](#talos.resource.definitions.k8s.KubePrismEndpointsSpec)
    - [KubePrismStatusesSpec](#talos.resource.definitions.k8s.KubePrismStatusesSpec)
    - [KubeletConfigSpec](#talos.resource.definitions.k8s.KubeletConfigSpec)
    - [KubeletConfigSpec.ExtraArgsEntry](#talos.resource.definitions.k8s.KubeletConfigSpec.ExtraArgsEntry)
    - [KubeletSpecSpec](#talos.resource.definitions.k8s.KubeletSpecSpec)
    - [ManifestSpec](#talos.resource.definitions.k8s.ManifestSpec)
    - [ManifestStatusSpec](#talos.resource.definitions.k8s.ManifestStatusSpec)
    - [NodeAnnotationSpecSpec](#talos.resource.definitions.k8s.NodeAnnotationSpecSpec)
    - [NodeIPConfigSpec](#talos.resource.definitions.k8s.NodeIPConfigSpec)
    - [NodeIPSpec](#talos.resource.definitions.k8s.NodeIPSpec)
    - [NodeLabelSpecSpec](#talos.resource.definitions.k8s.NodeLabelSpecSpec)
    - [NodeStatusSpec](#talos.resource.definitions.k8s.NodeStatusSpec)
    - [NodeStatusSpec.AnnotationsEntry](#talos.resource.definitions.k8s.NodeStatusSpec.AnnotationsEntry)
    - [NodeStatusSpec.LabelsEntry](#talos.resource.definitions.k8s.NodeStatusSpec.LabelsEntry)
    - [NodeTaintSpecSpec](#talos.resource.definitions.k8s.NodeTaintSpecSpec)
    - [NodenameSpec](#talos.resource.definitions.k8s.NodenameSpec)
    - [Resources](#talos.resource.definitions.k8s.Resources)
    - [Resources.LimitsEntry](#talos.resource.definitions.k8s.Resources.LimitsEntry)
    - [Resources.RequestsEntry](#talos.resource.definitions.k8s.Resources.RequestsEntry)
    - [SchedulerConfigSpec](#talos.resource.definitions.k8s.SchedulerConfigSpec)
    - [SchedulerConfigSpec.EnvironmentVariablesEntry](#talos.resource.definitions.k8s.SchedulerConfigSpec.EnvironmentVariablesEntry)
    - [SchedulerConfigSpec.ExtraArgsEntry](#talos.resource.definitions.k8s.SchedulerConfigSpec.ExtraArgsEntry)
    - [SecretsStatusSpec](#talos.resource.definitions.k8s.SecretsStatusSpec)
    - [SingleManifest](#talos.resource.definitions.k8s.SingleManifest)
    - [StaticPodServerStatusSpec](#talos.resource.definitions.k8s.StaticPodServerStatusSpec)
    - [StaticPodSpec](#talos.resource.definitions.k8s.StaticPodSpec)
    - [StaticPodStatusSpec](#talos.resource.definitions.k8s.StaticPodStatusSpec)
  
- [resource/definitions/kubeaccess/kubeaccess.proto](#resource/definitions/kubeaccess/kubeaccess.proto)
    - [ConfigSpec](#talos.resource.definitions.kubeaccess.ConfigSpec)
  
- [resource/definitions/kubespan/kubespan.proto](#resource/definitions/kubespan/kubespan.proto)
    - [ConfigSpec](#talos.resource.definitions.kubespan.ConfigSpec)
    - [EndpointSpec](#talos.resource.definitions.kubespan.EndpointSpec)
    - [IdentitySpec](#talos.resource.definitions.kubespan.IdentitySpec)
    - [PeerSpecSpec](#talos.resource.definitions.kubespan.PeerSpecSpec)
    - [PeerStatusSpec](#talos.resource.definitions.kubespan.PeerStatusSpec)
  
- [resource/definitions/network/network.proto](#resource/definitions/network/network.proto)
    - [AddressSpecSpec](#talos.resource.definitions.network.AddressSpecSpec)
    - [AddressStatusSpec](#talos.resource.definitions.network.AddressStatusSpec)
    - [BondMasterSpec](#talos.resource.definitions.network.BondMasterSpec)
    - [BondSlave](#talos.resource.definitions.network.BondSlave)
    - [BridgeMasterSpec](#talos.resource.definitions.network.BridgeMasterSpec)
    - [BridgeSlave](#talos.resource.definitions.network.BridgeSlave)
    - [BridgeVLANSpec](#talos.resource.definitions.network.BridgeVLANSpec)
    - [DHCP4OperatorSpec](#talos.resource.definitions.network.DHCP4OperatorSpec)
    - [DHCP6OperatorSpec](#talos.resource.definitions.network.DHCP6OperatorSpec)
    - [DNSResolveCacheSpec](#talos.resource.definitions.network.DNSResolveCacheSpec)
    - [EthernetChannelsSpec](#talos.resource.definitions.network.EthernetChannelsSpec)
    - [EthernetChannelsStatus](#talos.resource.definitions.network.EthernetChannelsStatus)
    - [EthernetFeatureStatus](#talos.resource.definitions.network.EthernetFeatureStatus)
    - [EthernetRingsSpec](#talos.resource.definitions.network.EthernetRingsSpec)
    - [EthernetRingsStatus](#talos.resource.definitions.network.EthernetRingsStatus)
    - [EthernetSpecSpec](#talos.resource.definitions.network.EthernetSpecSpec)
    - [EthernetSpecSpec.FeaturesEntry](#talos.resource.definitions.network.EthernetSpecSpec.FeaturesEntry)
    - [EthernetStatusSpec](#talos.resource.definitions.network.EthernetStatusSpec)
    - [HardwareAddrSpec](#talos.resource.definitions.network.HardwareAddrSpec)
    - [HostDNSConfigSpec](#talos.resource.definitions.network.HostDNSConfigSpec)
    - [HostnameSpecSpec](#talos.resource.definitions.network.HostnameSpecSpec)
    - [HostnameStatusSpec](#talos.resource.definitions.network.HostnameStatusSpec)
    - [LinkRefreshSpec](#talos.resource.definitions.network.LinkRefreshSpec)
    - [LinkSpecSpec](#talos.resource.definitions.network.LinkSpecSpec)
    - [LinkStatusSpec](#talos.resource.definitions.network.LinkStatusSpec)
    - [NfTablesAddressMatch](#talos.resource.definitions.network.NfTablesAddressMatch)
    - [NfTablesChainSpec](#talos.resource.definitions.network.NfTablesChainSpec)
    - [NfTablesClampMSS](#talos.resource.definitions.network.NfTablesClampMSS)
    - [NfTablesConntrackStateMatch](#talos.resource.definitions.network.NfTablesConntrackStateMatch)
    - [NfTablesIfNameMatch](#talos.resource.definitions.network.NfTablesIfNameMatch)
    - [NfTablesLayer4Match](#talos.resource.definitions.network.NfTablesLayer4Match)
    - [NfTablesLimitMatch](#talos.resource.definitions.network.NfTablesLimitMatch)
    - [NfTablesMark](#talos.resource.definitions.network.NfTablesMark)
    - [NfTablesPortMatch](#talos.resource.definitions.network.NfTablesPortMatch)
    - [NfTablesRule](#talos.resource.definitions.network.NfTablesRule)
    - [NodeAddressFilterSpec](#talos.resource.definitions.network.NodeAddressFilterSpec)
    - [NodeAddressSortAlgorithmSpec](#talos.resource.definitions.network.NodeAddressSortAlgorithmSpec)
    - [NodeAddressSpec](#talos.resource.definitions.network.NodeAddressSpec)
    - [OperatorSpecSpec](#talos.resource.definitions.network.OperatorSpecSpec)
    - [PortRange](#talos.resource.definitions.network.PortRange)
    - [ProbeSpecSpec](#talos.resource.definitions.network.ProbeSpecSpec)
    - [ProbeStatusSpec](#talos.resource.definitions.network.ProbeStatusSpec)
    - [ResolverSpecSpec](#talos.resource.definitions.network.ResolverSpecSpec)
    - [ResolverStatusSpec](#talos.resource.definitions.network.ResolverStatusSpec)
    - [RouteSpecSpec](#talos.resource.definitions.network.RouteSpecSpec)
    - [RouteStatusSpec](#talos.resource.definitions.network.RouteStatusSpec)
    - [STPSpec](#talos.resource.definitions.network.STPSpec)
    - [StatusSpec](#talos.resource.definitions.network.StatusSpec)
    - [TCPProbeSpec](#talos.resource.definitions.network.TCPProbeSpec)
    - [TimeServerSpecSpec](#talos.resource.definitions.network.TimeServerSpecSpec)
    - [TimeServerStatusSpec](#talos.resource.definitions.network.TimeServerStatusSpec)
    - [VIPEquinixMetalSpec](#talos.resource.definitions.network.VIPEquinixMetalSpec)
    - [VIPHCloudSpec](#talos.resource.definitions.network.VIPHCloudSpec)
    - [VIPOperatorSpec](#talos.resource.definitions.network.VIPOperatorSpec)
    - [VLANSpec](#talos.resource.definitions.network.VLANSpec)
    - [WireguardPeer](#talos.resource.definitions.network.WireguardPeer)
    - [WireguardSpec](#talos.resource.definitions.network.WireguardSpec)
  
- [resource/definitions/perf/perf.proto](#resource/definitions/perf/perf.proto)
    - [CPUSpec](#talos.resource.definitions.perf.CPUSpec)
    - [CPUStat](#talos.resource.definitions.perf.CPUStat)
    - [MemorySpec](#talos.resource.definitions.perf.MemorySpec)
  
- [resource/definitions/proto/proto.proto](#resource/definitions/proto/proto.proto)
    - [LinuxIDMapping](#talos.resource.definitions.proto.LinuxIDMapping)
    - [Mount](#talos.resource.definitions.proto.Mount)
  
- [resource/definitions/runtime/runtime.proto](#resource/definitions/runtime/runtime.proto)
    - [DevicesStatusSpec](#talos.resource.definitions.runtime.DevicesStatusSpec)
    - [DiagnosticSpec](#talos.resource.definitions.runtime.DiagnosticSpec)
    - [EventSinkConfigSpec](#talos.resource.definitions.runtime.EventSinkConfigSpec)
    - [ExtensionServiceConfigFile](#talos.resource.definitions.runtime.ExtensionServiceConfigFile)
    - [ExtensionServiceConfigSpec](#talos.resource.definitions.runtime.ExtensionServiceConfigSpec)
    - [ExtensionServiceConfigStatusSpec](#talos.resource.definitions.runtime.ExtensionServiceConfigStatusSpec)
    - [KernelModuleSpecSpec](#talos.resource.definitions.runtime.KernelModuleSpecSpec)
    - [KernelParamSpecSpec](#talos.resource.definitions.runtime.KernelParamSpecSpec)
    - [KernelParamStatusSpec](#talos.resource.definitions.runtime.KernelParamStatusSpec)
    - [KmsgLogConfigSpec](#talos.resource.definitions.runtime.KmsgLogConfigSpec)
    - [MachineStatusSpec](#talos.resource.definitions.runtime.MachineStatusSpec)
    - [MachineStatusStatus](#talos.resource.definitions.runtime.MachineStatusStatus)
    - [MaintenanceServiceConfigSpec](#talos.resource.definitions.runtime.MaintenanceServiceConfigSpec)
    - [MetaKeySpec](#talos.resource.definitions.runtime.MetaKeySpec)
    - [MetaLoadedSpec](#talos.resource.definitions.runtime.MetaLoadedSpec)
    - [MountStatusSpec](#talos.resource.definitions.runtime.MountStatusSpec)
    - [PlatformMetadataSpec](#talos.resource.definitions.runtime.PlatformMetadataSpec)
    - [PlatformMetadataSpec.TagsEntry](#talos.resource.definitions.runtime.PlatformMetadataSpec.TagsEntry)
    - [SecurityStateSpec](#talos.resource.definitions.runtime.SecurityStateSpec)
    - [UniqueMachineTokenSpec](#talos.resource.definitions.runtime.UniqueMachineTokenSpec)
    - [UnmetCondition](#talos.resource.definitions.runtime.UnmetCondition)
    - [WatchdogTimerConfigSpec](#talos.resource.definitions.runtime.WatchdogTimerConfigSpec)
    - [WatchdogTimerStatusSpec](#talos.resource.definitions.runtime.WatchdogTimerStatusSpec)
  
- [resource/definitions/secrets/secrets.proto](#resource/definitions/secrets/secrets.proto)
    - [APICertsSpec](#talos.resource.definitions.secrets.APICertsSpec)
    - [CertSANSpec](#talos.resource.definitions.secrets.CertSANSpec)
    - [EtcdCertsSpec](#talos.resource.definitions.secrets.EtcdCertsSpec)
    - [EtcdRootSpec](#talos.resource.definitions.secrets.EtcdRootSpec)
    - [KubeletSpec](#talos.resource.definitions.secrets.KubeletSpec)
    - [KubernetesCertsSpec](#talos.resource.definitions.secrets.KubernetesCertsSpec)
    - [KubernetesDynamicCertsSpec](#talos.resource.definitions.secrets.KubernetesDynamicCertsSpec)
    - [KubernetesRootSpec](#talos.resource.definitions.secrets.KubernetesRootSpec)
    - [MaintenanceRootSpec](#talos.resource.definitions.secrets.MaintenanceRootSpec)
    - [MaintenanceServiceCertsSpec](#talos.resource.definitions.secrets.MaintenanceServiceCertsSpec)
    - [OSRootSpec](#talos.resource.definitions.secrets.OSRootSpec)
    - [TrustdCertsSpec](#talos.resource.definitions.secrets.TrustdCertsSpec)
  
- [resource/definitions/siderolink/siderolink.proto](#resource/definitions/siderolink/siderolink.proto)
    - [ConfigSpec](#talos.resource.definitions.siderolink.ConfigSpec)
    - [StatusSpec](#talos.resource.definitions.siderolink.StatusSpec)
    - [TunnelSpec](#talos.resource.definitions.siderolink.TunnelSpec)
  
- [resource/definitions/time/time.proto](#resource/definitions/time/time.proto)
    - [AdjtimeStatusSpec](#talos.resource.definitions.time.AdjtimeStatusSpec)
    - [StatusSpec](#talos.resource.definitions.time.StatusSpec)
  
- [resource/definitions/v1alpha1/v1alpha1.proto](#resource/definitions/v1alpha1/v1alpha1.proto)
    - [ServiceSpec](#talos.resource.definitions.v1alpha1.ServiceSpec)
  
- [inspect/inspect.proto](#inspect/inspect.proto)
    - [ControllerDependencyEdge](#inspect.ControllerDependencyEdge)
    - [ControllerRuntimeDependenciesResponse](#inspect.ControllerRuntimeDependenciesResponse)
    - [ControllerRuntimeDependency](#inspect.ControllerRuntimeDependency)
  
    - [DependencyEdgeType](#inspect.DependencyEdgeType)
  
    - [InspectService](#inspect.InspectService)
  
- [machine/machine.proto](#machine/machine.proto)
    - [AddressEvent](#machine.AddressEvent)
    - [ApplyConfiguration](#machine.ApplyConfiguration)
    - [ApplyConfigurationRequest](#machine.ApplyConfigurationRequest)
    - [ApplyConfigurationResponse](#machine.ApplyConfigurationResponse)
    - [BPFInstruction](#machine.BPFInstruction)
    - [Bootstrap](#machine.Bootstrap)
    - [BootstrapRequest](#machine.BootstrapRequest)
    - [BootstrapResponse](#machine.BootstrapResponse)
    - [CNIConfig](#machine.CNIConfig)
    - [CPUFreqStats](#machine.CPUFreqStats)
    - [CPUFreqStatsResponse](#machine.CPUFreqStatsResponse)
    - [CPUInfo](#machine.CPUInfo)
    - [CPUInfoResponse](#machine.CPUInfoResponse)
    - [CPUStat](#machine.CPUStat)
    - [CPUsFreqStats](#machine.CPUsFreqStats)
    - [CPUsInfo](#machine.CPUsInfo)
    - [ClusterConfig](#machine.ClusterConfig)
    - [ClusterNetworkConfig](#machine.ClusterNetworkConfig)
    - [ConfigLoadErrorEvent](#machine.ConfigLoadErrorEvent)
    - [ConfigValidationErrorEvent](#machine.ConfigValidationErrorEvent)
    - [ConnectRecord](#machine.ConnectRecord)
    - [ConnectRecord.Process](#machine.ConnectRecord.Process)
    - [Container](#machine.Container)
    - [ContainerInfo](#machine.ContainerInfo)
    - [ContainersRequest](#machine.ContainersRequest)
    - [ContainersResponse](#machine.ContainersResponse)
    - [ControlPlaneConfig](#machine.ControlPlaneConfig)
    - [CopyRequest](#machine.CopyRequest)
    - [DHCPOptionsConfig](#machine.DHCPOptionsConfig)
    - [DiskStat](#machine.DiskStat)
    - [DiskStats](#machine.DiskStats)
    - [DiskStatsResponse](#machine.DiskStatsResponse)
    - [DiskUsageInfo](#machine.DiskUsageInfo)
    - [DiskUsageRequest](#machine.DiskUsageRequest)
    - [DmesgRequest](#machine.DmesgRequest)
    - [EtcdAlarm](#machine.EtcdAlarm)
    - [EtcdAlarmDisarm](#machine.EtcdAlarmDisarm)
    - [EtcdAlarmDisarmResponse](#machine.EtcdAlarmDisarmResponse)
    - [EtcdAlarmListResponse](#machine.EtcdAlarmListResponse)
    - [EtcdDefragment](#machine.EtcdDefragment)
    - [EtcdDefragmentResponse](#machine.EtcdDefragmentResponse)
    - [EtcdForfeitLeadership](#machine.EtcdForfeitLeadership)
    - [EtcdForfeitLeadershipRequest](#machine.EtcdForfeitLeadershipRequest)
    - [EtcdForfeitLeadershipResponse](#machine.EtcdForfeitLeadershipResponse)
    - [EtcdLeaveCluster](#machine.EtcdLeaveCluster)
    - [EtcdLeaveClusterRequest](#machine.EtcdLeaveClusterRequest)
    - [EtcdLeaveClusterResponse](#machine.EtcdLeaveClusterResponse)
    - [EtcdMember](#machine.EtcdMember)
    - [EtcdMemberAlarm](#machine.EtcdMemberAlarm)
    - [EtcdMemberListRequest](#machine.EtcdMemberListRequest)
    - [EtcdMemberListResponse](#machine.EtcdMemberListResponse)
    - [EtcdMemberStatus](#machine.EtcdMemberStatus)
    - [EtcdMembers](#machine.EtcdMembers)
    - [EtcdRecover](#machine.EtcdRecover)
    - [EtcdRecoverResponse](#machine.EtcdRecoverResponse)
    - [EtcdRemoveMember](#machine.EtcdRemoveMember)
    - [EtcdRemoveMemberByID](#machine.EtcdRemoveMemberByID)
    - [EtcdRemoveMemberByIDRequest](#machine.EtcdRemoveMemberByIDRequest)
    - [EtcdRemoveMemberByIDResponse](#machine.EtcdRemoveMemberByIDResponse)
    - [EtcdRemoveMemberRequest](#machine.EtcdRemoveMemberRequest)
    - [EtcdRemoveMemberResponse](#machine.EtcdRemoveMemberResponse)
    - [EtcdSnapshotRequest](#machine.EtcdSnapshotRequest)
    - [EtcdStatus](#machine.EtcdStatus)
    - [EtcdStatusResponse](#machine.EtcdStatusResponse)
    - [Event](#machine.Event)
    - [EventsRequest](#machine.EventsRequest)
    - [FeaturesInfo](#machine.FeaturesInfo)
    - [FileInfo](#machine.FileInfo)
    - [GenerateClientConfiguration](#machine.GenerateClientConfiguration)
    - [GenerateClientConfigurationRequest](#machine.GenerateClientConfigurationRequest)
    - [GenerateClientConfigurationResponse](#machine.GenerateClientConfigurationResponse)
    - [GenerateConfiguration](#machine.GenerateConfiguration)
    - [GenerateConfigurationRequest](#machine.GenerateConfigurationRequest)
    - [GenerateConfigurationResponse](#machine.GenerateConfigurationResponse)
    - [Hostname](#machine.Hostname)
    - [HostnameResponse](#machine.HostnameResponse)
    - [ImageListRequest](#machine.ImageListRequest)
    - [ImageListResponse](#machine.ImageListResponse)
    - [ImagePull](#machine.ImagePull)
    - [ImagePullRequest](#machine.ImagePullRequest)
    - [ImagePullResponse](#machine.ImagePullResponse)
    - [InstallConfig](#machine.InstallConfig)
    - [ListRequest](#machine.ListRequest)
    - [LoadAvg](#machine.LoadAvg)
    - [LoadAvgResponse](#machine.LoadAvgResponse)
    - [LogsContainer](#machine.LogsContainer)
    - [LogsContainersResponse](#machine.LogsContainersResponse)
    - [LogsRequest](#machine.LogsRequest)
    - [MachineConfig](#machine.MachineConfig)
    - [MachineStatusEvent](#machine.MachineStatusEvent)
    - [MachineStatusEvent.MachineStatus](#machine.MachineStatusEvent.MachineStatus)
    - [MachineStatusEvent.MachineStatus.UnmetCondition](#machine.MachineStatusEvent.MachineStatus.UnmetCondition)
    - [MemInfo](#machine.MemInfo)
    - [Memory](#machine.Memory)
    - [MemoryResponse](#machine.MemoryResponse)
    - [MetaDelete](#machine.MetaDelete)
    - [MetaDeleteRequest](#machine.MetaDeleteRequest)
    - [MetaDeleteResponse](#machine.MetaDeleteResponse)
    - [MetaWrite](#machine.MetaWrite)
    - [MetaWriteRequest](#machine.MetaWriteRequest)
    - [MetaWriteResponse](#machine.MetaWriteResponse)
    - [MountStat](#machine.MountStat)
    - [Mounts](#machine.Mounts)
    - [MountsResponse](#machine.MountsResponse)
    - [NetDev](#machine.NetDev)
    - [Netstat](#machine.Netstat)
    - [NetstatRequest](#machine.NetstatRequest)
    - [NetstatRequest.Feature](#machine.NetstatRequest.Feature)
    - [NetstatRequest.L4proto](#machine.NetstatRequest.L4proto)
    - [NetstatRequest.NetNS](#machine.NetstatRequest.NetNS)
    - [NetstatResponse](#machine.NetstatResponse)
    - [NetworkConfig](#machine.NetworkConfig)
    - [NetworkDeviceConfig](#machine.NetworkDeviceConfig)
    - [NetworkDeviceStats](#machine.NetworkDeviceStats)
    - [NetworkDeviceStatsResponse](#machine.NetworkDeviceStatsResponse)
    - [PacketCaptureRequest](#machine.PacketCaptureRequest)
    - [PhaseEvent](#machine.PhaseEvent)
    - [PlatformInfo](#machine.PlatformInfo)
    - [Process](#machine.Process)
    - [ProcessInfo](#machine.ProcessInfo)
    - [ProcessesResponse](#machine.ProcessesResponse)
    - [ReadRequest](#machine.ReadRequest)
    - [Reboot](#machine.Reboot)
    - [RebootRequest](#machine.RebootRequest)
    - [RebootResponse](#machine.RebootResponse)
    - [Reset](#machine.Reset)
    - [ResetPartitionSpec](#machine.ResetPartitionSpec)
    - [ResetRequest](#machine.ResetRequest)
    - [ResetResponse](#machine.ResetResponse)
    - [Restart](#machine.Restart)
    - [RestartEvent](#machine.RestartEvent)
    - [RestartRequest](#machine.RestartRequest)
    - [RestartResponse](#machine.RestartResponse)
    - [Rollback](#machine.Rollback)
    - [RollbackRequest](#machine.RollbackRequest)
    - [RollbackResponse](#machine.RollbackResponse)
    - [RouteConfig](#machine.RouteConfig)
    - [SequenceEvent](#machine.SequenceEvent)
    - [ServiceEvent](#machine.ServiceEvent)
    - [ServiceEvents](#machine.ServiceEvents)
    - [ServiceHealth](#machine.ServiceHealth)
    - [ServiceInfo](#machine.ServiceInfo)
    - [ServiceList](#machine.ServiceList)
    - [ServiceListResponse](#machine.ServiceListResponse)
    - [ServiceRestart](#machine.ServiceRestart)
    - [ServiceRestartRequest](#machine.ServiceRestartRequest)
    - [ServiceRestartResponse](#machine.ServiceRestartResponse)
    - [ServiceStart](#machine.ServiceStart)
    - [ServiceStartRequest](#machine.ServiceStartRequest)
    - [ServiceStartResponse](#machine.ServiceStartResponse)
    - [ServiceStateEvent](#machine.ServiceStateEvent)
    - [ServiceStop](#machine.ServiceStop)
    - [ServiceStopRequest](#machine.ServiceStopRequest)
    - [ServiceStopResponse](#machine.ServiceStopResponse)
    - [Shutdown](#machine.Shutdown)
    - [ShutdownRequest](#machine.ShutdownRequest)
    - [ShutdownResponse](#machine.ShutdownResponse)
    - [SoftIRQStat](#machine.SoftIRQStat)
    - [Stat](#machine.Stat)
    - [Stats](#machine.Stats)
    - [StatsRequest](#machine.StatsRequest)
    - [StatsResponse](#machine.StatsResponse)
    - [SystemStat](#machine.SystemStat)
    - [SystemStatResponse](#machine.SystemStatResponse)
    - [TaskEvent](#machine.TaskEvent)
    - [Upgrade](#machine.Upgrade)
    - [UpgradeRequest](#machine.UpgradeRequest)
    - [UpgradeResponse](#machine.UpgradeResponse)
    - [Version](#machine.Version)
    - [VersionInfo](#machine.VersionInfo)
    - [VersionResponse](#machine.VersionResponse)
    - [Xattr](#machine.Xattr)
  
    - [ApplyConfigurationRequest.Mode](#machine.ApplyConfigurationRequest.Mode)
    - [ConnectRecord.State](#machine.ConnectRecord.State)
    - [ConnectRecord.TimerActive](#machine.ConnectRecord.TimerActive)
    - [EtcdMemberAlarm.AlarmType](#machine.EtcdMemberAlarm.AlarmType)
    - [ListRequest.Type](#machine.ListRequest.Type)
    - [MachineConfig.MachineType](#machine.MachineConfig.MachineType)
    - [MachineStatusEvent.MachineStage](#machine.MachineStatusEvent.MachineStage)
    - [NetstatRequest.Filter](#machine.NetstatRequest.Filter)
    - [PhaseEvent.Action](#machine.PhaseEvent.Action)
    - [RebootRequest.Mode](#machine.RebootRequest.Mode)
    - [ResetRequest.WipeMode](#machine.ResetRequest.WipeMode)
    - [SequenceEvent.Action](#machine.SequenceEvent.Action)
    - [ServiceStateEvent.Action](#machine.ServiceStateEvent.Action)
    - [TaskEvent.Action](#machine.TaskEvent.Action)
    - [UpgradeRequest.RebootMode](#machine.UpgradeRequest.RebootMode)
  
    - [MachineService](#machine.MachineService)
  
- [security/security.proto](#security/security.proto)
    - [CertificateRequest](#securityapi.CertificateRequest)
    - [CertificateResponse](#securityapi.CertificateResponse)
  
    - [SecurityService](#securityapi.SecurityService)
  
- [storage/storage.proto](#storage/storage.proto)
    - [BlockDeviceWipe](#storage.BlockDeviceWipe)
    - [BlockDeviceWipeDescriptor](#storage.BlockDeviceWipeDescriptor)
    - [BlockDeviceWipeRequest](#storage.BlockDeviceWipeRequest)
    - [BlockDeviceWipeResponse](#storage.BlockDeviceWipeResponse)
    - [Disk](#storage.Disk)
    - [Disks](#storage.Disks)
    - [DisksResponse](#storage.DisksResponse)
  
    - [BlockDeviceWipeDescriptor.Method](#storage.BlockDeviceWipeDescriptor.Method)
    - [Disk.DiskType](#storage.Disk.DiskType)
  
    - [StorageService](#storage.StorageService)
  
- [time/time.proto](#time/time.proto)
    - [Time](#time.Time)
    - [TimeRequest](#time.TimeRequest)
    - [TimeResponse](#time.TimeResponse)
  
    - [TimeService](#time.TimeService)
  
- [Scalar Value Types](#scalar-value-types)



<a name="common/common.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## common/common.proto



<a name="common.Data"></a>

### Data



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [Metadata](#common.Metadata) |  |  |
| bytes | [bytes](#bytes) |  |  |






<a name="common.DataResponse"></a>

### DataResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Data](#common.Data) | repeated |  |






<a name="common.Empty"></a>

### Empty



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [Metadata](#common.Metadata) |  |  |






<a name="common.EmptyResponse"></a>

### EmptyResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Empty](#common.Empty) | repeated |  |






<a name="common.Error"></a>

### Error



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| code | [Code](#common.Code) |  |  |
| message | [string](#string) |  |  |
| details | [google.protobuf.Any](#google.protobuf.Any) | repeated |  |






<a name="common.Metadata"></a>

### Metadata
Common metadata message nested in all reply message types


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| hostname | [string](#string) |  | hostname of the server response comes from (injected by proxy) |
| error | [string](#string) |  | error is set if request failed to the upstream (rest of response is undefined) |
| status | [google.rpc.Status](#google.rpc.Status) |  | error as gRPC Status |






<a name="common.NetIP"></a>

### NetIP



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ip | [bytes](#bytes) |  |  |






<a name="common.NetIPPort"></a>

### NetIPPort



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ip | [bytes](#bytes) |  |  |
| port | [int32](#int32) |  |  |






<a name="common.NetIPPrefix"></a>

### NetIPPrefix



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ip | [bytes](#bytes) |  |  |
| prefix_length | [int32](#int32) |  |  |






<a name="common.PEMEncodedCertificate"></a>

### PEMEncodedCertificate



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| crt | [bytes](#bytes) |  |  |






<a name="common.PEMEncodedCertificateAndKey"></a>

### PEMEncodedCertificateAndKey



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| crt | [bytes](#bytes) |  |  |
| key | [bytes](#bytes) |  |  |






<a name="common.PEMEncodedKey"></a>

### PEMEncodedKey



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [bytes](#bytes) |  |  |






<a name="common.URL"></a>

### URL



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| full_path | [string](#string) |  |  |





 <!-- end messages -->


<a name="common.Code"></a>

### Code


| Name | Number | Description |
| ---- | ------ | ----------- |
| FATAL | 0 |  |
| LOCKED | 1 |  |
| CANCELED | 2 |  |



<a name="common.ContainerDriver"></a>

### ContainerDriver


| Name | Number | Description |
| ---- | ------ | ----------- |
| CONTAINERD | 0 |  |
| CRI | 1 |  |



<a name="common.ContainerdNamespace"></a>

### ContainerdNamespace


| Name | Number | Description |
| ---- | ------ | ----------- |
| NS_UNKNOWN | 0 |  |
| NS_SYSTEM | 1 |  |
| NS_CRI | 2 |  |


 <!-- end enums -->


<a name="common/common.proto-extensions"></a>

### File-level Extensions
| Extension | Type | Base | Number | Description |
| --------- | ---- | ---- | ------ | ----------- |
| remove_deprecated_enum | string | .google.protobuf.EnumOptions | 93117 | Indicates the Talos version when this deprecated enum will be removed from API. |
| remove_deprecated_enum_value | string | .google.protobuf.EnumValueOptions | 93117 | Indicates the Talos version when this deprecated enum value will be removed from API. |
| remove_deprecated_field | string | .google.protobuf.FieldOptions | 93117 | Indicates the Talos version when this deprecated filed will be removed from API. |
| remove_deprecated_message | string | .google.protobuf.MessageOptions | 93117 | Indicates the Talos version when this deprecated message will be removed from API. |
| remove_deprecated_method | string | .google.protobuf.MethodOptions | 93117 | Indicates the Talos version when this deprecated method will be removed from API. |
| remove_deprecated_service | string | .google.protobuf.ServiceOptions | 93117 | Indicates the Talos version when this deprecated service will be removed from API. |

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="resource/definitions/block/block.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## resource/definitions/block/block.proto



<a name="talos.resource.definitions.block.DeviceSpec"></a>

### DeviceSpec
DeviceSpec is the spec for devices status.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| type | [string](#string) |  |  |
| major | [int64](#int64) |  |  |
| minor | [int64](#int64) |  |  |
| partition_name | [string](#string) |  |  |
| partition_number | [int64](#int64) |  |  |
| generation | [int64](#int64) |  |  |
| device_path | [string](#string) |  |  |
| parent | [string](#string) |  |  |
| secondaries | [string](#string) | repeated |  |






<a name="talos.resource.definitions.block.DiscoveredVolumeSpec"></a>

### DiscoveredVolumeSpec
DiscoveredVolumeSpec is the spec for DiscoveredVolumes resource.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| size | [uint64](#uint64) |  |  |
| sector_size | [uint64](#uint64) |  |  |
| io_size | [uint64](#uint64) |  |  |
| name | [string](#string) |  |  |
| uuid | [string](#string) |  |  |
| label | [string](#string) |  |  |
| block_size | [uint32](#uint32) |  |  |
| filesystem_block_size | [uint32](#uint32) |  |  |
| probed_size | [uint64](#uint64) |  |  |
| partition_uuid | [string](#string) |  |  |
| partition_type | [string](#string) |  |  |
| partition_label | [string](#string) |  |  |
| partition_index | [uint64](#uint64) |  |  |
| type | [string](#string) |  |  |
| device_path | [string](#string) |  |  |
| parent | [string](#string) |  |  |
| dev_path | [string](#string) |  |  |
| parent_dev_path | [string](#string) |  |  |
| pretty_size | [string](#string) |  |  |






<a name="talos.resource.definitions.block.DiscoveryRefreshRequestSpec"></a>

### DiscoveryRefreshRequestSpec
DiscoveryRefreshRequestSpec is the spec for DiscoveryRefreshRequest.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| request | [int64](#int64) |  |  |






<a name="talos.resource.definitions.block.DiscoveryRefreshStatusSpec"></a>

### DiscoveryRefreshStatusSpec
DiscoveryRefreshStatusSpec is the spec for DiscoveryRefreshStatus status.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| request | [int64](#int64) |  |  |






<a name="talos.resource.definitions.block.DiskSelector"></a>

### DiskSelector
DiskSelector selects a disk for the volume.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| match | [google.api.expr.v1alpha1.CheckedExpr](#google.api.expr.v1alpha1.CheckedExpr) |  |  |






<a name="talos.resource.definitions.block.DiskSpec"></a>

### DiskSpec
DiskSpec is the spec for Disks status.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| size | [uint64](#uint64) |  |  |
| io_size | [uint64](#uint64) |  |  |
| sector_size | [uint64](#uint64) |  |  |
| readonly | [bool](#bool) |  |  |
| model | [string](#string) |  |  |
| serial | [string](#string) |  |  |
| modalias | [string](#string) |  |  |
| wwid | [string](#string) |  |  |
| bus_path | [string](#string) |  |  |
| sub_system | [string](#string) |  |  |
| transport | [string](#string) |  |  |
| rotational | [bool](#bool) |  |  |
| cdrom | [bool](#bool) |  |  |
| dev_path | [string](#string) |  |  |
| pretty_size | [string](#string) |  |  |
| secondary_disks | [string](#string) | repeated |  |
| uuid | [string](#string) |  |  |
| symlinks | [string](#string) | repeated |  |






<a name="talos.resource.definitions.block.EncryptionKey"></a>

### EncryptionKey
EncryptionKey is the spec for volume encryption key.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| slot | [int64](#int64) |  |  |
| type | [talos.resource.definitions.enums.BlockEncryptionKeyType](#talos.resource.definitions.enums.BlockEncryptionKeyType) |  |  |
| static_passphrase | [bytes](#bytes) |  |  |
| kms_endpoint | [string](#string) |  |  |
| tpm_check_secureboot_status_on_enroll | [bool](#bool) |  |  |






<a name="talos.resource.definitions.block.EncryptionSpec"></a>

### EncryptionSpec
EncryptionSpec is the spec for volume encryption.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| provider | [talos.resource.definitions.enums.BlockEncryptionProviderType](#talos.resource.definitions.enums.BlockEncryptionProviderType) |  |  |
| keys | [EncryptionKey](#talos.resource.definitions.block.EncryptionKey) | repeated |  |
| cipher | [string](#string) |  |  |
| key_size | [uint64](#uint64) |  |  |
| block_size | [uint64](#uint64) |  |  |
| perf_options | [string](#string) | repeated |  |






<a name="talos.resource.definitions.block.FilesystemSpec"></a>

### FilesystemSpec
FilesystemSpec is the spec for volume filesystem.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| type | [talos.resource.definitions.enums.BlockFilesystemType](#talos.resource.definitions.enums.BlockFilesystemType) |  |  |
| label | [string](#string) |  |  |






<a name="talos.resource.definitions.block.LocatorSpec"></a>

### LocatorSpec
LocatorSpec is the spec for volume locator.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| match | [google.api.expr.v1alpha1.CheckedExpr](#google.api.expr.v1alpha1.CheckedExpr) |  |  |






<a name="talos.resource.definitions.block.MountRequestSpec"></a>

### MountRequestSpec
MountRequestSpec is the spec for MountRequest.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| volume_id | [string](#string) |  |  |
| parent_mount_id | [string](#string) |  |  |
| requesters | [string](#string) | repeated |  |
| requester_i_ds | [string](#string) | repeated |  |
| read_only | [bool](#bool) |  |  |






<a name="talos.resource.definitions.block.MountSpec"></a>

### MountSpec
MountSpec is the spec for volume mount.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| target_path | [string](#string) |  |  |
| selinux_label | [string](#string) |  |  |
| project_quota_support | [bool](#bool) |  |  |
| parent_id | [string](#string) |  |  |
| file_mode | [uint32](#uint32) |  |  |
| uid | [int64](#int64) |  |  |
| gid | [int64](#int64) |  |  |
| recursive_relabel | [bool](#bool) |  |  |






<a name="talos.resource.definitions.block.MountStatusSpec"></a>

### MountStatusSpec
MountStatusSpec is the spec for MountStatus.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| spec | [MountRequestSpec](#talos.resource.definitions.block.MountRequestSpec) |  |  |
| target | [string](#string) |  |  |
| source | [string](#string) |  |  |
| filesystem | [talos.resource.definitions.enums.BlockFilesystemType](#talos.resource.definitions.enums.BlockFilesystemType) |  |  |
| read_only | [bool](#bool) |  |  |
| project_quota_support | [bool](#bool) |  |  |
| encryption_provider | [talos.resource.definitions.enums.BlockEncryptionProviderType](#talos.resource.definitions.enums.BlockEncryptionProviderType) |  |  |






<a name="talos.resource.definitions.block.PartitionSpec"></a>

### PartitionSpec
PartitionSpec is the spec for volume partitioning.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| min_size | [uint64](#uint64) |  |  |
| max_size | [uint64](#uint64) |  |  |
| grow | [bool](#bool) |  |  |
| label | [string](#string) |  |  |
| type_uuid | [string](#string) |  |  |






<a name="talos.resource.definitions.block.ProvisioningSpec"></a>

### ProvisioningSpec
ProvisioningSpec is the spec for volume provisioning.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| disk_selector | [DiskSelector](#talos.resource.definitions.block.DiskSelector) |  |  |
| partition_spec | [PartitionSpec](#talos.resource.definitions.block.PartitionSpec) |  |  |
| wave | [int64](#int64) |  |  |
| filesystem_spec | [FilesystemSpec](#talos.resource.definitions.block.FilesystemSpec) |  |  |






<a name="talos.resource.definitions.block.SwapStatusSpec"></a>

### SwapStatusSpec
SwapStatusSpec is the spec for SwapStatuss resource.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| device | [string](#string) |  |  |
| size_bytes | [uint64](#uint64) |  |  |
| size_human | [string](#string) |  |  |
| used_bytes | [uint64](#uint64) |  |  |
| used_human | [string](#string) |  |  |
| priority | [int32](#int32) |  |  |
| type | [string](#string) |  |  |






<a name="talos.resource.definitions.block.SymlinkProvisioningSpec"></a>

### SymlinkProvisioningSpec
SymlinkProvisioningSpec is the spec for volume symlink.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| symlink_target_path | [string](#string) |  |  |
| force | [bool](#bool) |  |  |






<a name="talos.resource.definitions.block.SymlinkSpec"></a>

### SymlinkSpec
SymlinkSpec is the spec for Symlinks resource.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| paths | [string](#string) | repeated |  |






<a name="talos.resource.definitions.block.SystemDiskSpec"></a>

### SystemDiskSpec
SystemDiskSpec is the spec for SystemDisks resource.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| disk_id | [string](#string) |  |  |
| dev_path | [string](#string) |  |  |






<a name="talos.resource.definitions.block.UserDiskConfigStatusSpec"></a>

### UserDiskConfigStatusSpec
UserDiskConfigStatusSpec is the spec for UserDiskConfigStatus resource.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ready | [bool](#bool) |  |  |
| torn_down | [bool](#bool) |  |  |






<a name="talos.resource.definitions.block.VolumeConfigSpec"></a>

### VolumeConfigSpec
VolumeConfigSpec is the spec for VolumeConfig resource.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| parent_id | [string](#string) |  |  |
| type | [talos.resource.definitions.enums.BlockVolumeType](#talos.resource.definitions.enums.BlockVolumeType) |  |  |
| provisioning | [ProvisioningSpec](#talos.resource.definitions.block.ProvisioningSpec) |  |  |
| locator | [LocatorSpec](#talos.resource.definitions.block.LocatorSpec) |  |  |
| mount | [MountSpec](#talos.resource.definitions.block.MountSpec) |  |  |
| encryption | [EncryptionSpec](#talos.resource.definitions.block.EncryptionSpec) |  |  |
| symlink | [SymlinkProvisioningSpec](#talos.resource.definitions.block.SymlinkProvisioningSpec) |  |  |






<a name="talos.resource.definitions.block.VolumeMountRequestSpec"></a>

### VolumeMountRequestSpec
VolumeMountRequestSpec is the spec for VolumeMountRequest.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| volume_id | [string](#string) |  |  |
| requester | [string](#string) |  |  |
| read_only | [bool](#bool) |  |  |






<a name="talos.resource.definitions.block.VolumeMountStatusSpec"></a>

### VolumeMountStatusSpec
VolumeMountStatusSpec is the spec for VolumeMountStatus.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| volume_id | [string](#string) |  |  |
| requester | [string](#string) |  |  |
| target | [string](#string) |  |  |
| read_only | [bool](#bool) |  |  |






<a name="talos.resource.definitions.block.VolumeStatusSpec"></a>

### VolumeStatusSpec
VolumeStatusSpec is the spec for VolumeStatus resource.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| phase | [talos.resource.definitions.enums.BlockVolumePhase](#talos.resource.definitions.enums.BlockVolumePhase) |  |  |
| location | [string](#string) |  |  |
| error_message | [string](#string) |  |  |
| uuid | [string](#string) |  |  |
| partition_uuid | [string](#string) |  |  |
| pre_fail_phase | [talos.resource.definitions.enums.BlockVolumePhase](#talos.resource.definitions.enums.BlockVolumePhase) |  |  |
| parent_location | [string](#string) |  |  |
| partition_index | [int64](#int64) |  |  |
| size | [uint64](#uint64) |  |  |
| filesystem | [talos.resource.definitions.enums.BlockFilesystemType](#talos.resource.definitions.enums.BlockFilesystemType) |  |  |
| mount_location | [string](#string) |  |  |
| encryption_provider | [talos.resource.definitions.enums.BlockEncryptionProviderType](#talos.resource.definitions.enums.BlockEncryptionProviderType) |  |  |
| pretty_size | [string](#string) |  |  |
| encryption_failed_syncs | [string](#string) | repeated |  |
| mount_spec | [MountSpec](#talos.resource.definitions.block.MountSpec) |  |  |
| type | [talos.resource.definitions.enums.BlockVolumeType](#talos.resource.definitions.enums.BlockVolumeType) |  |  |
| configured_encryption_keys | [string](#string) | repeated |  |
| symlink_spec | [SymlinkProvisioningSpec](#talos.resource.definitions.block.SymlinkProvisioningSpec) |  |  |
| parent_id | [string](#string) |  |  |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="resource/definitions/cluster/cluster.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## resource/definitions/cluster/cluster.proto



<a name="talos.resource.definitions.cluster.AffiliateSpec"></a>

### AffiliateSpec
AffiliateSpec describes Affiliate state.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  |  |
| addresses | [common.NetIP](#common.NetIP) | repeated |  |
| hostname | [string](#string) |  |  |
| nodename | [string](#string) |  |  |
| operating_system | [string](#string) |  |  |
| machine_type | [talos.resource.definitions.enums.MachineType](#talos.resource.definitions.enums.MachineType) |  |  |
| kube_span | [KubeSpanAffiliateSpec](#talos.resource.definitions.cluster.KubeSpanAffiliateSpec) |  |  |
| control_plane | [ControlPlane](#talos.resource.definitions.cluster.ControlPlane) |  |  |






<a name="talos.resource.definitions.cluster.ConfigSpec"></a>

### ConfigSpec
ConfigSpec describes KubeSpan configuration.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| discovery_enabled | [bool](#bool) |  |  |
| registry_kubernetes_enabled | [bool](#bool) |  |  |
| registry_service_enabled | [bool](#bool) |  |  |
| service_endpoint | [string](#string) |  |  |
| service_endpoint_insecure | [bool](#bool) |  |  |
| service_encryption_key | [bytes](#bytes) |  |  |
| service_cluster_id | [string](#string) |  |  |






<a name="talos.resource.definitions.cluster.ControlPlane"></a>

### ControlPlane
ControlPlane describes ControlPlane data if any.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| api_server_port | [int64](#int64) |  |  |






<a name="talos.resource.definitions.cluster.IdentitySpec"></a>

### IdentitySpec
IdentitySpec describes status of rendered secrets.

Note: IdentitySpec is persisted on disk in the STATE partition,
so YAML serialization should be kept backwards compatible.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  |  |






<a name="talos.resource.definitions.cluster.InfoSpec"></a>

### InfoSpec
InfoSpec describes cluster information.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| cluster_id | [string](#string) |  |  |
| cluster_name | [string](#string) |  |  |






<a name="talos.resource.definitions.cluster.KubeSpanAffiliateSpec"></a>

### KubeSpanAffiliateSpec
KubeSpanAffiliateSpec describes additional information specific for the KubeSpan.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| public_key | [string](#string) |  |  |
| address | [common.NetIP](#common.NetIP) |  |  |
| additional_addresses | [common.NetIPPrefix](#common.NetIPPrefix) | repeated |  |
| endpoints | [common.NetIPPort](#common.NetIPPort) | repeated |  |






<a name="talos.resource.definitions.cluster.MemberSpec"></a>

### MemberSpec
MemberSpec describes Member state.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  |  |
| addresses | [common.NetIP](#common.NetIP) | repeated |  |
| hostname | [string](#string) |  |  |
| machine_type | [talos.resource.definitions.enums.MachineType](#talos.resource.definitions.enums.MachineType) |  |  |
| operating_system | [string](#string) |  |  |
| control_plane | [ControlPlane](#talos.resource.definitions.cluster.ControlPlane) |  |  |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="resource/definitions/cri/cri.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## resource/definitions/cri/cri.proto



<a name="talos.resource.definitions.cri.ImageCacheConfigSpec"></a>

### ImageCacheConfigSpec
ImageCacheConfigSpec represents the ImageCacheConfig.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| status | [talos.resource.definitions.enums.CriImageCacheStatus](#talos.resource.definitions.enums.CriImageCacheStatus) |  |  |
| roots | [string](#string) | repeated |  |
| copy_status | [talos.resource.definitions.enums.CriImageCacheCopyStatus](#talos.resource.definitions.enums.CriImageCacheCopyStatus) |  |  |






<a name="talos.resource.definitions.cri.RegistriesConfigSpec"></a>

### RegistriesConfigSpec
RegistriesConfigSpec describes status of rendered secrets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| registry_mirrors | [RegistriesConfigSpec.RegistryMirrorsEntry](#talos.resource.definitions.cri.RegistriesConfigSpec.RegistryMirrorsEntry) | repeated |  |
| registry_config | [RegistriesConfigSpec.RegistryConfigEntry](#talos.resource.definitions.cri.RegistriesConfigSpec.RegistryConfigEntry) | repeated |  |






<a name="talos.resource.definitions.cri.RegistriesConfigSpec.RegistryConfigEntry"></a>

### RegistriesConfigSpec.RegistryConfigEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [RegistryConfig](#talos.resource.definitions.cri.RegistryConfig) |  |  |






<a name="talos.resource.definitions.cri.RegistriesConfigSpec.RegistryMirrorsEntry"></a>

### RegistriesConfigSpec.RegistryMirrorsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [RegistryMirrorConfig](#talos.resource.definitions.cri.RegistryMirrorConfig) |  |  |






<a name="talos.resource.definitions.cri.RegistryAuthConfig"></a>

### RegistryAuthConfig
RegistryAuthConfig specifies authentication configuration for a registry.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| registry_username | [string](#string) |  |  |
| registry_password | [string](#string) |  |  |
| registry_auth | [string](#string) |  |  |
| registry_identity_token | [string](#string) |  |  |






<a name="talos.resource.definitions.cri.RegistryConfig"></a>

### RegistryConfig
RegistryConfig specifies auth & TLS config per registry.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| registry_tls | [RegistryTLSConfig](#talos.resource.definitions.cri.RegistryTLSConfig) |  |  |
| registry_auth | [RegistryAuthConfig](#talos.resource.definitions.cri.RegistryAuthConfig) |  |  |






<a name="talos.resource.definitions.cri.RegistryEndpointConfig"></a>

### RegistryEndpointConfig
RegistryEndpointConfig represents a single registry endpoint.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| endpoint_endpoint | [string](#string) |  |  |
| endpoint_override_path | [bool](#bool) |  |  |






<a name="talos.resource.definitions.cri.RegistryMirrorConfig"></a>

### RegistryMirrorConfig
RegistryMirrorConfig represents mirror configuration for a registry.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| mirror_endpoints | [RegistryEndpointConfig](#talos.resource.definitions.cri.RegistryEndpointConfig) | repeated |  |
| mirror_skip_fallback | [bool](#bool) |  |  |






<a name="talos.resource.definitions.cri.RegistryTLSConfig"></a>

### RegistryTLSConfig
RegistryTLSConfig specifies TLS config for HTTPS registries.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tls_client_identity | [common.PEMEncodedCertificateAndKey](#common.PEMEncodedCertificateAndKey) |  |  |
| tlsca | [bytes](#bytes) |  |  |
| tls_insecure_skip_verify | [bool](#bool) |  |  |






<a name="talos.resource.definitions.cri.SeccompProfileSpec"></a>

### SeccompProfileSpec
SeccompProfileSpec represents the SeccompProfile.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| value | [google.protobuf.Struct](#google.protobuf.Struct) |  |  |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="resource/definitions/enums/enums.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## resource/definitions/enums/enums.proto


 <!-- end messages -->


<a name="talos.resource.definitions.enums.BlockEncryptionKeyType"></a>

### BlockEncryptionKeyType
BlockEncryptionKeyType describes encryption key type.

| Name | Number | Description |
| ---- | ------ | ----------- |
| ENCRYPTION_KEY_STATIC | 0 |  |
| ENCRYPTION_KEY_NODE_ID | 1 |  |
| ENCRYPTION_KEY_KMS | 2 |  |
| ENCRYPTION_KEY_TPM | 3 |  |



<a name="talos.resource.definitions.enums.BlockEncryptionProviderType"></a>

### BlockEncryptionProviderType
BlockEncryptionProviderType describes encryption provider type.

| Name | Number | Description |
| ---- | ------ | ----------- |
| ENCRYPTION_PROVIDER_NONE | 0 |  |
| ENCRYPTION_PROVIDER_LUKS2 | 1 |  |



<a name="talos.resource.definitions.enums.BlockFilesystemType"></a>

### BlockFilesystemType
BlockFilesystemType describes filesystem type.

| Name | Number | Description |
| ---- | ------ | ----------- |
| FILESYSTEM_TYPE_NONE | 0 |  |
| FILESYSTEM_TYPE_XFS | 1 |  |
| FILESYSTEM_TYPE_VFAT | 2 |  |
| FILESYSTEM_TYPE_EXT4 | 3 |  |
| FILESYSTEM_TYPE_ISO9660 | 4 |  |
| FILESYSTEM_TYPE_SWAP | 5 |  |



<a name="talos.resource.definitions.enums.BlockVolumePhase"></a>

### BlockVolumePhase
BlockVolumePhase describes volume phase.

| Name | Number | Description |
| ---- | ------ | ----------- |
| VOLUME_PHASE_WAITING | 0 |  |
| VOLUME_PHASE_FAILED | 1 |  |
| VOLUME_PHASE_MISSING | 2 |  |
| VOLUME_PHASE_LOCATED | 3 |  |
| VOLUME_PHASE_PROVISIONED | 4 |  |
| VOLUME_PHASE_PREPARED | 5 |  |
| VOLUME_PHASE_READY | 6 |  |
| VOLUME_PHASE_CLOSED | 7 |  |



<a name="talos.resource.definitions.enums.BlockVolumeType"></a>

### BlockVolumeType
BlockVolumeType describes volume type.

| Name | Number | Description |
| ---- | ------ | ----------- |
| VOLUME_TYPE_PARTITION | 0 |  |
| VOLUME_TYPE_DISK | 1 |  |
| VOLUME_TYPE_TMPFS | 2 |  |
| VOLUME_TYPE_DIRECTORY | 3 |  |
| VOLUME_TYPE_SYMLINK | 4 |  |
| VOLUME_TYPE_OVERLAY | 5 |  |



<a name="talos.resource.definitions.enums.CriImageCacheCopyStatus"></a>

### CriImageCacheCopyStatus
CriImageCacheCopyStatus describes image cache copy status type.

| Name | Number | Description |
| ---- | ------ | ----------- |
| IMAGE_CACHE_COPY_STATUS_UNKNOWN | 0 |  |
| IMAGE_CACHE_COPY_STATUS_SKIPPED | 1 |  |
| IMAGE_CACHE_COPY_STATUS_PENDING | 2 |  |
| IMAGE_CACHE_COPY_STATUS_READY | 3 |  |



<a name="talos.resource.definitions.enums.CriImageCacheStatus"></a>

### CriImageCacheStatus
CriImageCacheStatus describes image cache status type.

| Name | Number | Description |
| ---- | ------ | ----------- |
| IMAGE_CACHE_STATUS_UNKNOWN | 0 |  |
| IMAGE_CACHE_STATUS_DISABLED | 1 |  |
| IMAGE_CACHE_STATUS_PREPARING | 2 |  |
| IMAGE_CACHE_STATUS_READY | 3 |  |



<a name="talos.resource.definitions.enums.KubespanPeerState"></a>

### KubespanPeerState
KubespanPeerState is KubeSpan peer current state.

| Name | Number | Description |
| ---- | ------ | ----------- |
| PEER_STATE_UNKNOWN | 0 |  |
| PEER_STATE_UP | 1 |  |
| PEER_STATE_DOWN | 2 |  |



<a name="talos.resource.definitions.enums.MachineType"></a>

### MachineType
MachineType represents a machine type.

| Name | Number | Description |
| ---- | ------ | ----------- |
| TYPE_UNKNOWN | 0 | TypeUnknown represents undefined node type, when there is no machine configuration yet. |
| TYPE_INIT | 1 | TypeInit type designates the first control plane node to come up. You can think of it like a bootstrap node. This node will perform the initial steps to bootstrap the cluster -- generation of TLS assets, starting of the control plane, etc. |
| TYPE_CONTROL_PLANE | 2 | TypeControlPlane designates the node as a control plane member. This means it will host etcd along with the Kubernetes controlplane components such as API Server, Controller Manager, Scheduler. |
| TYPE_WORKER | 3 | TypeWorker designates the node as a worker node. This means it will be an available compute node for scheduling workloads. |



<a name="talos.resource.definitions.enums.NethelpersADSelect"></a>

### NethelpersADSelect
NethelpersADSelect is ADSelect.

| Name | Number | Description |
| ---- | ------ | ----------- |
| AD_SELECT_STABLE | 0 |  |
| AD_SELECT_BANDWIDTH | 1 |  |
| AD_SELECT_COUNT | 2 |  |



<a name="talos.resource.definitions.enums.NethelpersARPAllTargets"></a>

### NethelpersARPAllTargets
NethelpersARPAllTargets is an ARP targets mode.

| Name | Number | Description |
| ---- | ------ | ----------- |
| ARP_ALL_TARGETS_ANY | 0 |  |
| ARP_ALL_TARGETS_ALL | 1 |  |



<a name="talos.resource.definitions.enums.NethelpersARPValidate"></a>

### NethelpersARPValidate
NethelpersARPValidate is an ARP Validation mode.

| Name | Number | Description |
| ---- | ------ | ----------- |
| ARP_VALIDATE_NONE | 0 |  |
| ARP_VALIDATE_ACTIVE | 1 |  |
| ARP_VALIDATE_BACKUP | 2 |  |
| ARP_VALIDATE_ALL | 3 |  |



<a name="talos.resource.definitions.enums.NethelpersAddressFlag"></a>

### NethelpersAddressFlag
NethelpersAddressFlag wraps IFF_* constants.

| Name | Number | Description |
| ---- | ------ | ----------- |
| NETHELPERS_ADDRESSFLAG_UNSPECIFIED | 0 |  |
| ADDRESS_TEMPORARY | 1 |  |
| ADDRESS_NO_DAD | 2 |  |
| ADDRESS_OPTIMISTIC | 4 |  |
| ADDRESS_DAD_FAILED | 8 |  |
| ADDRESS_HOME | 16 |  |
| ADDRESS_DEPRECATED | 32 |  |
| ADDRESS_TENTATIVE | 64 |  |
| ADDRESS_PERMANENT | 128 |  |
| ADDRESS_MANAGEMENT_TEMP | 256 |  |
| ADDRESS_NO_PREFIX_ROUTE | 512 |  |
| ADDRESS_MC_AUTO_JOIN | 1024 |  |
| ADDRESS_STABLE_PRIVACY | 2048 |  |



<a name="talos.resource.definitions.enums.NethelpersAddressSortAlgorithm"></a>

### NethelpersAddressSortAlgorithm
NethelpersAddressSortAlgorithm is an internal address sorting algorithm.

| Name | Number | Description |
| ---- | ------ | ----------- |
| ADDRESS_SORT_ALGORITHM_V1 | 0 |  |
| ADDRESS_SORT_ALGORITHM_V2 | 1 |  |



<a name="talos.resource.definitions.enums.NethelpersBondMode"></a>

### NethelpersBondMode
NethelpersBondMode is a bond mode.

| Name | Number | Description |
| ---- | ------ | ----------- |
| BOND_MODE_ROUNDROBIN | 0 |  |
| BOND_MODE_ACTIVE_BACKUP | 1 |  |
| BOND_MODE_XOR | 2 |  |
| BOND_MODE_BROADCAST | 3 |  |
| BOND_MODE8023_AD | 4 |  |
| BOND_MODE_TLB | 5 |  |
| BOND_MODE_ALB | 6 |  |



<a name="talos.resource.definitions.enums.NethelpersBondXmitHashPolicy"></a>

### NethelpersBondXmitHashPolicy
NethelpersBondXmitHashPolicy is a bond hash policy.

| Name | Number | Description |
| ---- | ------ | ----------- |
| BOND_XMIT_POLICY_LAYER2 | 0 |  |
| BOND_XMIT_POLICY_LAYER34 | 1 |  |
| BOND_XMIT_POLICY_LAYER23 | 2 |  |
| BOND_XMIT_POLICY_ENCAP23 | 3 |  |
| BOND_XMIT_POLICY_ENCAP34 | 4 |  |



<a name="talos.resource.definitions.enums.NethelpersConntrackState"></a>

### NethelpersConntrackState
NethelpersConntrackState is a conntrack state.

| Name | Number | Description |
| ---- | ------ | ----------- |
| NETHELPERS_CONNTRACKSTATE_UNSPECIFIED | 0 |  |
| CONNTRACK_STATE_NEW | 8 |  |
| CONNTRACK_STATE_RELATED | 4 |  |
| CONNTRACK_STATE_ESTABLISHED | 2 |  |
| CONNTRACK_STATE_INVALID | 1 |  |



<a name="talos.resource.definitions.enums.NethelpersDuplex"></a>

### NethelpersDuplex
NethelpersDuplex wraps ethtool.Duplex for YAML marshaling.

| Name | Number | Description |
| ---- | ------ | ----------- |
| HALF | 0 |  |
| FULL | 1 |  |
| UNKNOWN | 255 |  |



<a name="talos.resource.definitions.enums.NethelpersFailOverMAC"></a>

### NethelpersFailOverMAC
NethelpersFailOverMAC is a MAC failover mode.

| Name | Number | Description |
| ---- | ------ | ----------- |
| FAIL_OVER_MAC_NONE | 0 |  |
| FAIL_OVER_MAC_ACTIVE | 1 |  |
| FAIL_OVER_MAC_FOLLOW | 2 |  |



<a name="talos.resource.definitions.enums.NethelpersFamily"></a>

### NethelpersFamily
NethelpersFamily is a network family.

| Name | Number | Description |
| ---- | ------ | ----------- |
| NETHELPERS_FAMILY_UNSPECIFIED | 0 |  |
| FAMILY_INET4 | 2 |  |
| FAMILY_INET6 | 10 |  |



<a name="talos.resource.definitions.enums.NethelpersLACPRate"></a>

### NethelpersLACPRate
NethelpersLACPRate is a LACP rate.

| Name | Number | Description |
| ---- | ------ | ----------- |
| LACP_RATE_SLOW | 0 |  |
| LACP_RATE_FAST | 1 |  |



<a name="talos.resource.definitions.enums.NethelpersLinkType"></a>

### NethelpersLinkType
NethelpersLinkType is a link type.

| Name | Number | Description |
| ---- | ------ | ----------- |
| LINK_NETROM | 0 |  |
| LINK_ETHER | 1 |  |
| LINK_EETHER | 2 |  |
| LINK_AX25 | 3 |  |
| LINK_PRONET | 4 |  |
| LINK_CHAOS | 5 |  |
| LINK_IEE802 | 6 |  |
| LINK_ARCNET | 7 |  |
| LINK_ATALK | 8 |  |
| LINK_DLCI | 15 |  |
| LINK_ATM | 19 |  |
| LINK_METRICOM | 23 |  |
| LINK_IEEE1394 | 24 |  |
| LINK_EUI64 | 27 |  |
| LINK_INFINIBAND | 32 |  |
| LINK_SLIP | 256 |  |
| LINK_CSLIP | 257 |  |
| LINK_SLIP6 | 258 |  |
| LINK_CSLIP6 | 259 |  |
| LINK_RSRVD | 260 |  |
| LINK_ADAPT | 264 |  |
| LINK_ROSE | 270 |  |
| LINK_X25 | 271 |  |
| LINK_HWX25 | 272 |  |
| LINK_CAN | 280 |  |
| LINK_PPP | 512 |  |
| LINK_CISCO | 513 |  |
| LINK_HDLC | 513 |  |
| LINK_LAPB | 516 |  |
| LINK_DDCMP | 517 |  |
| LINK_RAWHDLC | 518 |  |
| LINK_TUNNEL | 768 |  |
| LINK_TUNNEL6 | 769 |  |
| LINK_FRAD | 770 |  |
| LINK_SKIP | 771 |  |
| LINK_LOOPBCK | 772 |  |
| LINK_LOCALTLK | 773 |  |
| LINK_FDDI | 774 |  |
| LINK_BIF | 775 |  |
| LINK_SIT | 776 |  |
| LINK_IPDDP | 777 |  |
| LINK_IPGRE | 778 |  |
| LINK_PIMREG | 779 |  |
| LINK_HIPPI | 780 |  |
| LINK_ASH | 781 |  |
| LINK_ECONET | 782 |  |
| LINK_IRDA | 783 |  |
| LINK_FCPP | 784 |  |
| LINK_FCAL | 785 |  |
| LINK_FCPL | 786 |  |
| LINK_FCFABRIC | 787 |  |
| LINK_FCFABRIC1 | 788 |  |
| LINK_FCFABRIC2 | 789 |  |
| LINK_FCFABRIC3 | 790 |  |
| LINK_FCFABRIC4 | 791 |  |
| LINK_FCFABRIC5 | 792 |  |
| LINK_FCFABRIC6 | 793 |  |
| LINK_FCFABRIC7 | 794 |  |
| LINK_FCFABRIC8 | 795 |  |
| LINK_FCFABRIC9 | 796 |  |
| LINK_FCFABRIC10 | 797 |  |
| LINK_FCFABRIC11 | 798 |  |
| LINK_FCFABRIC12 | 799 |  |
| LINK_IEE802TR | 800 |  |
| LINK_IEE80211 | 801 |  |
| LINK_IEE80211PRISM | 802 |  |
| LINK_IEE80211_RADIOTAP | 803 |  |
| LINK_IEE8021154 | 804 |  |
| LINK_IEE8021154MONITOR | 805 |  |
| LINK_PHONET | 820 |  |
| LINK_PHONETPIPE | 821 |  |
| LINK_CAIF | 822 |  |
| LINK_IP6GRE | 823 |  |
| LINK_NETLINK | 824 |  |
| LINK6_LOWPAN | 825 |  |
| LINK_VOID | 65535 |  |
| LINK_NONE | 65534 |  |



<a name="talos.resource.definitions.enums.NethelpersMatchOperator"></a>

### NethelpersMatchOperator
NethelpersMatchOperator is a netfilter match operator.

| Name | Number | Description |
| ---- | ------ | ----------- |
| OPERATOR_EQUAL | 0 |  |
| OPERATOR_NOT_EQUAL | 1 |  |



<a name="talos.resource.definitions.enums.NethelpersNfTablesChainHook"></a>

### NethelpersNfTablesChainHook
NethelpersNfTablesChainHook wraps nftables.ChainHook for YAML marshaling.

| Name | Number | Description |
| ---- | ------ | ----------- |
| CHAIN_HOOK_PREROUTING | 0 |  |
| CHAIN_HOOK_INPUT | 1 |  |
| CHAIN_HOOK_FORWARD | 2 |  |
| CHAIN_HOOK_OUTPUT | 3 |  |
| CHAIN_HOOK_POSTROUTING | 4 |  |



<a name="talos.resource.definitions.enums.NethelpersNfTablesChainPriority"></a>

### NethelpersNfTablesChainPriority
NethelpersNfTablesChainPriority wraps nftables.ChainPriority for YAML marshaling.

| Name | Number | Description |
| ---- | ------ | ----------- |
| NETHELPERS_NFTABLESCHAINPRIORITY_UNSPECIFIED | 0 |  |
| CHAIN_PRIORITY_FIRST | -2147483648 |  |
| CHAIN_PRIORITY_CONNTRACK_DEFRAG | -400 |  |
| CHAIN_PRIORITY_RAW | -300 |  |
| CHAIN_PRIORITY_SE_LINUX_FIRST | -225 |  |
| CHAIN_PRIORITY_CONNTRACK | -200 |  |
| CHAIN_PRIORITY_MANGLE | -150 |  |
| CHAIN_PRIORITY_NAT_DEST | -100 |  |
| CHAIN_PRIORITY_FILTER | 0 |  |
| CHAIN_PRIORITY_SECURITY | 50 |  |
| CHAIN_PRIORITY_NAT_SOURCE | 100 |  |
| CHAIN_PRIORITY_SE_LINUX_LAST | 225 |  |
| CHAIN_PRIORITY_CONNTRACK_HELPER | 300 |  |
| CHAIN_PRIORITY_LAST | 2147483647 |  |



<a name="talos.resource.definitions.enums.NethelpersNfTablesVerdict"></a>

### NethelpersNfTablesVerdict
NethelpersNfTablesVerdict wraps nftables.Verdict for YAML marshaling.

| Name | Number | Description |
| ---- | ------ | ----------- |
| VERDICT_DROP | 0 |  |
| VERDICT_ACCEPT | 1 |  |



<a name="talos.resource.definitions.enums.NethelpersOperationalState"></a>

### NethelpersOperationalState
NethelpersOperationalState wraps rtnetlink.OperationalState for YAML marshaling.

| Name | Number | Description |
| ---- | ------ | ----------- |
| OPER_STATE_UNKNOWN | 0 |  |
| OPER_STATE_NOT_PRESENT | 1 |  |
| OPER_STATE_DOWN | 2 |  |
| OPER_STATE_LOWER_LAYER_DOWN | 3 |  |
| OPER_STATE_TESTING | 4 |  |
| OPER_STATE_DORMANT | 5 |  |
| OPER_STATE_UP | 6 |  |



<a name="talos.resource.definitions.enums.NethelpersPort"></a>

### NethelpersPort
NethelpersPort wraps ethtool.Port for YAML marshaling.

| Name | Number | Description |
| ---- | ------ | ----------- |
| TWISTED_PAIR | 0 |  |
| AUI | 1 |  |
| MII | 2 |  |
| FIBRE | 3 |  |
| BNC | 4 |  |
| DIRECT_ATTACH | 5 |  |
| NONE | 239 |  |
| OTHER | 255 |  |



<a name="talos.resource.definitions.enums.NethelpersPrimaryReselect"></a>

### NethelpersPrimaryReselect
NethelpersPrimaryReselect is an ARP targets mode.

| Name | Number | Description |
| ---- | ------ | ----------- |
| PRIMARY_RESELECT_ALWAYS | 0 |  |
| PRIMARY_RESELECT_BETTER | 1 |  |
| PRIMARY_RESELECT_FAILURE | 2 |  |



<a name="talos.resource.definitions.enums.NethelpersProtocol"></a>

### NethelpersProtocol
NethelpersProtocol is a inet protocol.

| Name | Number | Description |
| ---- | ------ | ----------- |
| NETHELPERS_PROTOCOL_UNSPECIFIED | 0 |  |
| PROTOCOL_ICMP | 1 |  |
| PROTOCOL_TCP | 6 |  |
| PROTOCOL_UDP | 17 |  |
| PROTOCOL_ICM_PV6 | 58 |  |



<a name="talos.resource.definitions.enums.NethelpersRouteFlag"></a>

### NethelpersRouteFlag
NethelpersRouteFlag wraps RTM_F_* constants.

| Name | Number | Description |
| ---- | ------ | ----------- |
| NETHELPERS_ROUTEFLAG_UNSPECIFIED | 0 |  |
| ROUTE_NOTIFY | 256 |  |
| ROUTE_CLONED | 512 |  |
| ROUTE_EQUALIZE | 1024 |  |
| ROUTE_PREFIX | 2048 |  |
| ROUTE_LOOKUP_TABLE | 4096 |  |
| ROUTE_FIB_MATCH | 8192 |  |
| ROUTE_OFFLOAD | 16384 |  |
| ROUTE_TRAP | 32768 |  |



<a name="talos.resource.definitions.enums.NethelpersRouteProtocol"></a>

### NethelpersRouteProtocol
NethelpersRouteProtocol is a routing protocol.

| Name | Number | Description |
| ---- | ------ | ----------- |
| PROTOCOL_UNSPEC | 0 |  |
| PROTOCOL_REDIRECT | 1 |  |
| PROTOCOL_KERNEL | 2 |  |
| PROTOCOL_BOOT | 3 |  |
| PROTOCOL_STATIC | 4 |  |
| PROTOCOL_RA | 9 |  |
| PROTOCOL_MRT | 10 |  |
| PROTOCOL_ZEBRA | 11 |  |
| PROTOCOL_BIRD | 12 |  |
| PROTOCOL_DNROUTED | 13 |  |
| PROTOCOL_XORP | 14 |  |
| PROTOCOL_NTK | 15 |  |
| PROTOCOL_DHCP | 16 |  |
| PROTOCOL_MRTD | 17 |  |
| PROTOCOL_KEEPALIVED | 18 |  |
| PROTOCOL_BABEL | 42 |  |
| PROTOCOL_OPENR | 99 |  |
| PROTOCOL_BGP | 186 |  |
| PROTOCOL_ISIS | 187 |  |
| PROTOCOL_OSPF | 188 |  |
| PROTOCOL_RIP | 189 |  |
| PROTOCOL_EIGRP | 192 |  |



<a name="talos.resource.definitions.enums.NethelpersRouteType"></a>

### NethelpersRouteType
NethelpersRouteType is a route type.

| Name | Number | Description |
| ---- | ------ | ----------- |
| TYPE_UNSPEC | 0 |  |
| TYPE_UNICAST | 1 |  |
| TYPE_LOCAL | 2 |  |
| TYPE_BROADCAST | 3 |  |
| TYPE_ANYCAST | 4 |  |
| TYPE_MULTICAST | 5 |  |
| TYPE_BLACKHOLE | 6 |  |
| TYPE_UNREACHABLE | 7 |  |
| TYPE_PROHIBIT | 8 |  |
| TYPE_THROW | 9 |  |
| TYPE_NAT | 10 |  |
| TYPE_X_RESOLVE | 11 |  |



<a name="talos.resource.definitions.enums.NethelpersRoutingTable"></a>

### NethelpersRoutingTable
NethelpersRoutingTable is a routing table ID.

| Name | Number | Description |
| ---- | ------ | ----------- |
| TABLE_UNSPEC | 0 |  |
| TABLE_DEFAULT | 253 |  |
| TABLE_MAIN | 254 |  |
| TABLE_LOCAL | 255 |  |



<a name="talos.resource.definitions.enums.NethelpersScope"></a>

### NethelpersScope
NethelpersScope is an address scope.

| Name | Number | Description |
| ---- | ------ | ----------- |
| SCOPE_GLOBAL | 0 |  |
| SCOPE_SITE | 200 |  |
| SCOPE_LINK | 253 |  |
| SCOPE_HOST | 254 |  |
| SCOPE_NOWHERE | 255 |  |



<a name="talos.resource.definitions.enums.NethelpersVLANProtocol"></a>

### NethelpersVLANProtocol
NethelpersVLANProtocol is a VLAN protocol.

| Name | Number | Description |
| ---- | ------ | ----------- |
| NETHELPERS_VLANPROTOCOL_UNSPECIFIED | 0 |  |
| VLAN_PROTOCOL8021_Q | 33024 |  |
| VLAN_PROTOCOL8021_AD | 34984 |  |



<a name="talos.resource.definitions.enums.NetworkConfigLayer"></a>

### NetworkConfigLayer
NetworkConfigLayer describes network configuration layers, with lowest priority first.

| Name | Number | Description |
| ---- | ------ | ----------- |
| CONFIG_DEFAULT | 0 |  |
| CONFIG_CMDLINE | 1 |  |
| CONFIG_PLATFORM | 2 |  |
| CONFIG_OPERATOR | 3 |  |
| CONFIG_MACHINE_CONFIGURATION | 4 |  |



<a name="talos.resource.definitions.enums.NetworkOperator"></a>

### NetworkOperator
NetworkOperator enumerates Talos network operators.

| Name | Number | Description |
| ---- | ------ | ----------- |
| OPERATOR_DHCP4 | 0 |  |
| OPERATOR_DHCP6 | 1 |  |
| OPERATOR_VIP | 2 |  |



<a name="talos.resource.definitions.enums.RuntimeMachineStage"></a>

### RuntimeMachineStage
RuntimeMachineStage describes the stage of the machine boot/run process.

| Name | Number | Description |
| ---- | ------ | ----------- |
| MACHINE_STAGE_UNKNOWN | 0 |  |
| MACHINE_STAGE_BOOTING | 1 |  |
| MACHINE_STAGE_INSTALLING | 2 |  |
| MACHINE_STAGE_MAINTENANCE | 3 |  |
| MACHINE_STAGE_RUNNING | 4 |  |
| MACHINE_STAGE_REBOOTING | 5 |  |
| MACHINE_STAGE_SHUTTING_DOWN | 6 |  |
| MACHINE_STAGE_RESETTING | 7 |  |
| MACHINE_STAGE_UPGRADING | 8 |  |



<a name="talos.resource.definitions.enums.RuntimeSELinuxState"></a>

### RuntimeSELinuxState
RuntimeSELinuxState describes the current SELinux status.

| Name | Number | Description |
| ---- | ------ | ----------- |
| SE_LINUX_STATE_DISABLED | 0 |  |
| SE_LINUX_STATE_PERMISSIVE | 1 |  |
| SE_LINUX_STATE_ENFORCING | 2 |  |


 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="resource/definitions/etcd/etcd.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## resource/definitions/etcd/etcd.proto



<a name="talos.resource.definitions.etcd.ConfigSpec"></a>

### ConfigSpec
ConfigSpec describes (some) configuration settings of etcd.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| advertise_valid_subnets | [string](#string) | repeated |  |
| advertise_exclude_subnets | [string](#string) | repeated |  |
| image | [string](#string) |  |  |
| extra_args | [ConfigSpec.ExtraArgsEntry](#talos.resource.definitions.etcd.ConfigSpec.ExtraArgsEntry) | repeated |  |
| listen_valid_subnets | [string](#string) | repeated |  |
| listen_exclude_subnets | [string](#string) | repeated |  |






<a name="talos.resource.definitions.etcd.ConfigSpec.ExtraArgsEntry"></a>

### ConfigSpec.ExtraArgsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="talos.resource.definitions.etcd.MemberSpec"></a>

### MemberSpec
MemberSpec holds information about an etcd member.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| member_id | [string](#string) |  |  |






<a name="talos.resource.definitions.etcd.PKIStatusSpec"></a>

### PKIStatusSpec
PKIStatusSpec describes status of rendered secrets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ready | [bool](#bool) |  |  |
| version | [string](#string) |  |  |






<a name="talos.resource.definitions.etcd.SpecSpec"></a>

### SpecSpec
SpecSpec describes (some) Specuration settings of etcd.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| advertised_addresses | [common.NetIP](#common.NetIP) | repeated |  |
| image | [string](#string) |  |  |
| extra_args | [SpecSpec.ExtraArgsEntry](#talos.resource.definitions.etcd.SpecSpec.ExtraArgsEntry) | repeated |  |
| listen_peer_addresses | [common.NetIP](#common.NetIP) | repeated |  |
| listen_client_addresses | [common.NetIP](#common.NetIP) | repeated |  |






<a name="talos.resource.definitions.etcd.SpecSpec.ExtraArgsEntry"></a>

### SpecSpec.ExtraArgsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="resource/definitions/extensions/extensions.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## resource/definitions/extensions/extensions.proto



<a name="talos.resource.definitions.extensions.Compatibility"></a>

### Compatibility
Compatibility describes extension compatibility.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| talos | [Constraint](#talos.resource.definitions.extensions.Constraint) |  |  |






<a name="talos.resource.definitions.extensions.Constraint"></a>

### Constraint
Constraint describes compatibility constraint.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| version | [string](#string) |  |  |






<a name="talos.resource.definitions.extensions.Layer"></a>

### Layer
Layer defines overlay mount layer.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| image | [string](#string) |  |  |
| metadata | [Metadata](#talos.resource.definitions.extensions.Metadata) |  |  |






<a name="talos.resource.definitions.extensions.Metadata"></a>

### Metadata
Metadata describes base extension metadata.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| version | [string](#string) |  |  |
| author | [string](#string) |  |  |
| description | [string](#string) |  |  |
| compatibility | [Compatibility](#talos.resource.definitions.extensions.Compatibility) |  |  |
| extra_info | [string](#string) |  |  |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="resource/definitions/files/files.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## resource/definitions/files/files.proto



<a name="talos.resource.definitions.files.EtcFileSpecSpec"></a>

### EtcFileSpecSpec
EtcFileSpecSpec describes status of rendered secrets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| contents | [bytes](#bytes) |  |  |
| mode | [uint32](#uint32) |  |  |
| selinux_label | [string](#string) |  |  |






<a name="talos.resource.definitions.files.EtcFileStatusSpec"></a>

### EtcFileStatusSpec
EtcFileStatusSpec describes status of rendered secrets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| spec_version | [string](#string) |  |  |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="resource/definitions/hardware/hardware.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## resource/definitions/hardware/hardware.proto



<a name="talos.resource.definitions.hardware.MemoryModuleSpec"></a>

### MemoryModuleSpec
MemoryModuleSpec represents a single Memory.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| size | [uint32](#uint32) |  |  |
| device_locator | [string](#string) |  |  |
| bank_locator | [string](#string) |  |  |
| speed | [uint32](#uint32) |  |  |
| manufacturer | [string](#string) |  |  |
| serial_number | [string](#string) |  |  |
| asset_tag | [string](#string) |  |  |
| product_name | [string](#string) |  |  |






<a name="talos.resource.definitions.hardware.PCIDeviceSpec"></a>

### PCIDeviceSpec
PCIDeviceSpec represents a single processor.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| class | [string](#string) |  |  |
| subclass | [string](#string) |  |  |
| vendor | [string](#string) |  |  |
| product | [string](#string) |  |  |
| class_id | [string](#string) |  |  |
| subclass_id | [string](#string) |  |  |
| vendor_id | [string](#string) |  |  |
| product_id | [string](#string) |  |  |
| driver | [string](#string) |  |  |






<a name="talos.resource.definitions.hardware.PCIDriverRebindConfigSpec"></a>

### PCIDriverRebindConfigSpec
PCIDriverRebindConfigSpec describes PCI rebind configuration.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| pciid | [string](#string) |  |  |
| target_driver | [string](#string) |  |  |






<a name="talos.resource.definitions.hardware.PCIDriverRebindStatusSpec"></a>

### PCIDriverRebindStatusSpec
PCIDriverRebindStatusSpec describes status of rebinded drivers.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| pciid | [string](#string) |  |  |
| target_driver | [string](#string) |  |  |






<a name="talos.resource.definitions.hardware.ProcessorSpec"></a>

### ProcessorSpec
ProcessorSpec represents a single processor.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| socket | [string](#string) |  |  |
| manufacturer | [string](#string) |  |  |
| product_name | [string](#string) |  |  |
| max_speed | [uint32](#uint32) |  |  |
| boot_speed | [uint32](#uint32) |  |  |
| status | [uint32](#uint32) |  |  |
| serial_number | [string](#string) |  |  |
| asset_tag | [string](#string) |  |  |
| part_number | [string](#string) |  |  |
| core_count | [uint32](#uint32) |  |  |
| core_enabled | [uint32](#uint32) |  |  |
| thread_count | [uint32](#uint32) |  |  |






<a name="talos.resource.definitions.hardware.SystemInformationSpec"></a>

### SystemInformationSpec
SystemInformationSpec represents the system information obtained from smbios.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| manufacturer | [string](#string) |  |  |
| product_name | [string](#string) |  |  |
| version | [string](#string) |  |  |
| serial_number | [string](#string) |  |  |
| uuid | [string](#string) |  |  |
| wake_up_type | [string](#string) |  |  |
| sku_number | [string](#string) |  |  |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="resource/definitions/k8s/k8s.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## resource/definitions/k8s/k8s.proto



<a name="talos.resource.definitions.k8s.APIServerConfigSpec"></a>

### APIServerConfigSpec
APIServerConfigSpec is configuration for kube-apiserver.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| image | [string](#string) |  |  |
| cloud_provider | [string](#string) |  |  |
| control_plane_endpoint | [string](#string) |  |  |
| etcd_servers | [string](#string) | repeated |  |
| local_port | [int64](#int64) |  |  |
| service_cid_rs | [string](#string) | repeated |  |
| extra_args | [APIServerConfigSpec.ExtraArgsEntry](#talos.resource.definitions.k8s.APIServerConfigSpec.ExtraArgsEntry) | repeated |  |
| extra_volumes | [ExtraVolume](#talos.resource.definitions.k8s.ExtraVolume) | repeated |  |
| environment_variables | [APIServerConfigSpec.EnvironmentVariablesEntry](#talos.resource.definitions.k8s.APIServerConfigSpec.EnvironmentVariablesEntry) | repeated |  |
| pod_security_policy_enabled | [bool](#bool) |  |  |
| advertised_address | [string](#string) |  |  |
| resources | [Resources](#talos.resource.definitions.k8s.Resources) |  |  |






<a name="talos.resource.definitions.k8s.APIServerConfigSpec.EnvironmentVariablesEntry"></a>

### APIServerConfigSpec.EnvironmentVariablesEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="talos.resource.definitions.k8s.APIServerConfigSpec.ExtraArgsEntry"></a>

### APIServerConfigSpec.ExtraArgsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="talos.resource.definitions.k8s.AdmissionControlConfigSpec"></a>

### AdmissionControlConfigSpec
AdmissionControlConfigSpec is configuration for kube-apiserver.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| config | [AdmissionPluginSpec](#talos.resource.definitions.k8s.AdmissionPluginSpec) | repeated |  |






<a name="talos.resource.definitions.k8s.AdmissionPluginSpec"></a>

### AdmissionPluginSpec
AdmissionPluginSpec is a single admission plugin configuration Admission Control plugins.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| configuration | [google.protobuf.Struct](#google.protobuf.Struct) |  |  |






<a name="talos.resource.definitions.k8s.AuditPolicyConfigSpec"></a>

### AuditPolicyConfigSpec
AuditPolicyConfigSpec is audit policy configuration for kube-apiserver.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| config | [google.protobuf.Struct](#google.protobuf.Struct) |  |  |






<a name="talos.resource.definitions.k8s.AuthorizationAuthorizersSpec"></a>

### AuthorizationAuthorizersSpec
AuthorizationAuthorizersSpec is a configuration of authorization authorizers.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| type | [string](#string) |  |  |
| name | [string](#string) |  |  |
| webhook | [google.protobuf.Struct](#google.protobuf.Struct) |  |  |






<a name="talos.resource.definitions.k8s.AuthorizationConfigSpec"></a>

### AuthorizationConfigSpec
AuthorizationConfigSpec is authorization configuration for kube-apiserver.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| image | [string](#string) |  |  |
| config | [AuthorizationAuthorizersSpec](#talos.resource.definitions.k8s.AuthorizationAuthorizersSpec) | repeated |  |






<a name="talos.resource.definitions.k8s.BootstrapManifestsConfigSpec"></a>

### BootstrapManifestsConfigSpec
BootstrapManifestsConfigSpec is configuration for bootstrap manifests.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| server | [string](#string) |  |  |
| cluster_domain | [string](#string) |  |  |
| pod_cid_rs | [string](#string) | repeated |  |
| proxy_enabled | [bool](#bool) |  |  |
| proxy_image | [string](#string) |  |  |
| proxy_args | [string](#string) | repeated |  |
| core_dns_enabled | [bool](#bool) |  |  |
| core_dns_image | [string](#string) |  |  |
| dns_service_ip | [string](#string) |  |  |
| dns_service_i_pv6 | [string](#string) |  |  |
| flannel_enabled | [bool](#bool) |  |  |
| flannel_image | [string](#string) |  |  |
| pod_security_policy_enabled | [bool](#bool) |  |  |
| talos_api_service_enabled | [bool](#bool) |  |  |
| flannel_extra_args | [string](#string) | repeated |  |
| flannel_kube_service_host | [string](#string) |  |  |
| flannel_kube_service_port | [string](#string) |  |  |






<a name="talos.resource.definitions.k8s.ConfigStatusSpec"></a>

### ConfigStatusSpec
ConfigStatusSpec describes status of rendered secrets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ready | [bool](#bool) |  |  |
| version | [string](#string) |  |  |






<a name="talos.resource.definitions.k8s.ControllerManagerConfigSpec"></a>

### ControllerManagerConfigSpec
ControllerManagerConfigSpec is configuration for kube-controller-manager.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| enabled | [bool](#bool) |  |  |
| image | [string](#string) |  |  |
| cloud_provider | [string](#string) |  |  |
| pod_cid_rs | [string](#string) | repeated |  |
| service_cid_rs | [string](#string) | repeated |  |
| extra_args | [ControllerManagerConfigSpec.ExtraArgsEntry](#talos.resource.definitions.k8s.ControllerManagerConfigSpec.ExtraArgsEntry) | repeated |  |
| extra_volumes | [ExtraVolume](#talos.resource.definitions.k8s.ExtraVolume) | repeated |  |
| environment_variables | [ControllerManagerConfigSpec.EnvironmentVariablesEntry](#talos.resource.definitions.k8s.ControllerManagerConfigSpec.EnvironmentVariablesEntry) | repeated |  |
| resources | [Resources](#talos.resource.definitions.k8s.Resources) |  |  |






<a name="talos.resource.definitions.k8s.ControllerManagerConfigSpec.EnvironmentVariablesEntry"></a>

### ControllerManagerConfigSpec.EnvironmentVariablesEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="talos.resource.definitions.k8s.ControllerManagerConfigSpec.ExtraArgsEntry"></a>

### ControllerManagerConfigSpec.ExtraArgsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="talos.resource.definitions.k8s.EndpointSpec"></a>

### EndpointSpec
EndpointSpec describes status of rendered secrets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| addresses | [common.NetIP](#common.NetIP) | repeated |  |






<a name="talos.resource.definitions.k8s.ExtraManifest"></a>

### ExtraManifest
ExtraManifest defines a single extra manifest to download.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| url | [string](#string) |  |  |
| priority | [string](#string) |  |  |
| extra_headers | [ExtraManifest.ExtraHeadersEntry](#talos.resource.definitions.k8s.ExtraManifest.ExtraHeadersEntry) | repeated |  |
| inline_manifest | [string](#string) |  |  |






<a name="talos.resource.definitions.k8s.ExtraManifest.ExtraHeadersEntry"></a>

### ExtraManifest.ExtraHeadersEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="talos.resource.definitions.k8s.ExtraManifestsConfigSpec"></a>

### ExtraManifestsConfigSpec
ExtraManifestsConfigSpec is configuration for extra bootstrap manifests.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| extra_manifests | [ExtraManifest](#talos.resource.definitions.k8s.ExtraManifest) | repeated |  |






<a name="talos.resource.definitions.k8s.ExtraVolume"></a>

### ExtraVolume
ExtraVolume is a configuration of extra volume.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| host_path | [string](#string) |  |  |
| mount_path | [string](#string) |  |  |
| read_only | [bool](#bool) |  |  |






<a name="talos.resource.definitions.k8s.KubePrismConfigSpec"></a>

### KubePrismConfigSpec
KubePrismConfigSpec describes KubePrismConfig data.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| host | [string](#string) |  |  |
| port | [int64](#int64) |  |  |
| endpoints | [KubePrismEndpoint](#talos.resource.definitions.k8s.KubePrismEndpoint) | repeated |  |






<a name="talos.resource.definitions.k8s.KubePrismEndpoint"></a>

### KubePrismEndpoint
KubePrismEndpoint holds data for control plane endpoint.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| host | [string](#string) |  |  |
| port | [uint32](#uint32) |  |  |






<a name="talos.resource.definitions.k8s.KubePrismEndpointsSpec"></a>

### KubePrismEndpointsSpec
KubePrismEndpointsSpec describes KubePrismEndpoints configuration.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| endpoints | [KubePrismEndpoint](#talos.resource.definitions.k8s.KubePrismEndpoint) | repeated |  |






<a name="talos.resource.definitions.k8s.KubePrismStatusesSpec"></a>

### KubePrismStatusesSpec
KubePrismStatusesSpec describes KubePrismStatuses data.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| host | [string](#string) |  |  |
| healthy | [bool](#bool) |  |  |






<a name="talos.resource.definitions.k8s.KubeletConfigSpec"></a>

### KubeletConfigSpec
KubeletConfigSpec holds the source of kubelet configuration.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| image | [string](#string) |  |  |
| cluster_dns | [string](#string) | repeated |  |
| cluster_domain | [string](#string) |  |  |
| extra_args | [KubeletConfigSpec.ExtraArgsEntry](#talos.resource.definitions.k8s.KubeletConfigSpec.ExtraArgsEntry) | repeated |  |
| extra_mounts | [talos.resource.definitions.proto.Mount](#talos.resource.definitions.proto.Mount) | repeated |  |
| extra_config | [google.protobuf.Struct](#google.protobuf.Struct) |  |  |
| cloud_provider_external | [bool](#bool) |  |  |
| default_runtime_seccomp_enabled | [bool](#bool) |  |  |
| skip_node_registration | [bool](#bool) |  |  |
| static_pod_list_url | [string](#string) |  |  |
| disable_manifests_directory | [bool](#bool) |  |  |
| enable_fs_quota_monitoring | [bool](#bool) |  |  |
| credential_provider_config | [google.protobuf.Struct](#google.protobuf.Struct) |  |  |
| allow_scheduling_on_control_plane | [bool](#bool) |  |  |






<a name="talos.resource.definitions.k8s.KubeletConfigSpec.ExtraArgsEntry"></a>

### KubeletConfigSpec.ExtraArgsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="talos.resource.definitions.k8s.KubeletSpecSpec"></a>

### KubeletSpecSpec
KubeletSpecSpec holds the source of kubelet configuration.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| image | [string](#string) |  |  |
| args | [string](#string) | repeated |  |
| extra_mounts | [talos.resource.definitions.proto.Mount](#talos.resource.definitions.proto.Mount) | repeated |  |
| expected_nodename | [string](#string) |  |  |
| config | [google.protobuf.Struct](#google.protobuf.Struct) |  |  |
| credential_provider_config | [google.protobuf.Struct](#google.protobuf.Struct) |  |  |






<a name="talos.resource.definitions.k8s.ManifestSpec"></a>

### ManifestSpec
ManifestSpec holds the Kubernetes resources spec.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| items | [SingleManifest](#talos.resource.definitions.k8s.SingleManifest) | repeated |  |






<a name="talos.resource.definitions.k8s.ManifestStatusSpec"></a>

### ManifestStatusSpec
ManifestStatusSpec describes manifest application status.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| manifests_applied | [string](#string) | repeated |  |






<a name="talos.resource.definitions.k8s.NodeAnnotationSpecSpec"></a>

### NodeAnnotationSpecSpec
NodeAnnotationSpecSpec represents an annoation that's attached to a Talos node.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="talos.resource.definitions.k8s.NodeIPConfigSpec"></a>

### NodeIPConfigSpec
NodeIPConfigSpec holds the Node IP specification.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| valid_subnets | [string](#string) | repeated |  |
| exclude_subnets | [string](#string) | repeated |  |






<a name="talos.resource.definitions.k8s.NodeIPSpec"></a>

### NodeIPSpec
NodeIPSpec holds the Node IP specification.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| addresses | [common.NetIP](#common.NetIP) | repeated |  |






<a name="talos.resource.definitions.k8s.NodeLabelSpecSpec"></a>

### NodeLabelSpecSpec
NodeLabelSpecSpec represents a label that's attached to a Talos node.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="talos.resource.definitions.k8s.NodeStatusSpec"></a>

### NodeStatusSpec
NodeStatusSpec describes Kubernetes NodeStatus.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| nodename | [string](#string) |  |  |
| node_ready | [bool](#bool) |  |  |
| unschedulable | [bool](#bool) |  |  |
| labels | [NodeStatusSpec.LabelsEntry](#talos.resource.definitions.k8s.NodeStatusSpec.LabelsEntry) | repeated |  |
| annotations | [NodeStatusSpec.AnnotationsEntry](#talos.resource.definitions.k8s.NodeStatusSpec.AnnotationsEntry) | repeated |  |






<a name="talos.resource.definitions.k8s.NodeStatusSpec.AnnotationsEntry"></a>

### NodeStatusSpec.AnnotationsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="talos.resource.definitions.k8s.NodeStatusSpec.LabelsEntry"></a>

### NodeStatusSpec.LabelsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="talos.resource.definitions.k8s.NodeTaintSpecSpec"></a>

### NodeTaintSpecSpec
NodeTaintSpecSpec represents a label that's attached to a Talos node.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| effect | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="talos.resource.definitions.k8s.NodenameSpec"></a>

### NodenameSpec
NodenameSpec describes Kubernetes nodename.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| nodename | [string](#string) |  |  |
| hostname_version | [string](#string) |  |  |
| skip_node_registration | [bool](#bool) |  |  |






<a name="talos.resource.definitions.k8s.Resources"></a>

### Resources
Resources is a configuration of cpu and memory resources.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| requests | [Resources.RequestsEntry](#talos.resource.definitions.k8s.Resources.RequestsEntry) | repeated |  |
| limits | [Resources.LimitsEntry](#talos.resource.definitions.k8s.Resources.LimitsEntry) | repeated |  |






<a name="talos.resource.definitions.k8s.Resources.LimitsEntry"></a>

### Resources.LimitsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="talos.resource.definitions.k8s.Resources.RequestsEntry"></a>

### Resources.RequestsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="talos.resource.definitions.k8s.SchedulerConfigSpec"></a>

### SchedulerConfigSpec
SchedulerConfigSpec is configuration for kube-scheduler.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| enabled | [bool](#bool) |  |  |
| image | [string](#string) |  |  |
| extra_args | [SchedulerConfigSpec.ExtraArgsEntry](#talos.resource.definitions.k8s.SchedulerConfigSpec.ExtraArgsEntry) | repeated |  |
| extra_volumes | [ExtraVolume](#talos.resource.definitions.k8s.ExtraVolume) | repeated |  |
| environment_variables | [SchedulerConfigSpec.EnvironmentVariablesEntry](#talos.resource.definitions.k8s.SchedulerConfigSpec.EnvironmentVariablesEntry) | repeated |  |
| resources | [Resources](#talos.resource.definitions.k8s.Resources) |  |  |
| config | [google.protobuf.Struct](#google.protobuf.Struct) |  |  |






<a name="talos.resource.definitions.k8s.SchedulerConfigSpec.EnvironmentVariablesEntry"></a>

### SchedulerConfigSpec.EnvironmentVariablesEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="talos.resource.definitions.k8s.SchedulerConfigSpec.ExtraArgsEntry"></a>

### SchedulerConfigSpec.ExtraArgsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="talos.resource.definitions.k8s.SecretsStatusSpec"></a>

### SecretsStatusSpec
SecretsStatusSpec describes status of rendered secrets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ready | [bool](#bool) |  |  |
| version | [string](#string) |  |  |






<a name="talos.resource.definitions.k8s.SingleManifest"></a>

### SingleManifest
SingleManifest is a single manifest.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| object | [google.protobuf.Struct](#google.protobuf.Struct) |  |  |






<a name="talos.resource.definitions.k8s.StaticPodServerStatusSpec"></a>

### StaticPodServerStatusSpec
StaticPodServerStatusSpec describes static pod spec, it contains marshaled *v1.Pod spec.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| url | [string](#string) |  |  |






<a name="talos.resource.definitions.k8s.StaticPodSpec"></a>

### StaticPodSpec
StaticPodSpec describes static pod spec, it contains marshaled *v1.Pod spec.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| pod | [google.protobuf.Struct](#google.protobuf.Struct) |  |  |






<a name="talos.resource.definitions.k8s.StaticPodStatusSpec"></a>

### StaticPodStatusSpec
StaticPodStatusSpec describes kubelet static pod status.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| pod_status | [google.protobuf.Struct](#google.protobuf.Struct) |  |  |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="resource/definitions/kubeaccess/kubeaccess.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## resource/definitions/kubeaccess/kubeaccess.proto



<a name="talos.resource.definitions.kubeaccess.ConfigSpec"></a>

### ConfigSpec
ConfigSpec describes KubeSpan configuration..


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| enabled | [bool](#bool) |  |  |
| allowed_api_roles | [string](#string) | repeated |  |
| allowed_kubernetes_namespaces | [string](#string) | repeated |  |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="resource/definitions/kubespan/kubespan.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## resource/definitions/kubespan/kubespan.proto



<a name="talos.resource.definitions.kubespan.ConfigSpec"></a>

### ConfigSpec
ConfigSpec describes KubeSpan configuration..


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| enabled | [bool](#bool) |  |  |
| cluster_id | [string](#string) |  |  |
| shared_secret | [string](#string) |  |  |
| force_routing | [bool](#bool) |  |  |
| advertise_kubernetes_networks | [bool](#bool) |  |  |
| mtu | [uint32](#uint32) |  |  |
| endpoint_filters | [string](#string) | repeated |  |
| harvest_extra_endpoints | [bool](#bool) |  |  |
| extra_endpoints | [common.NetIPPort](#common.NetIPPort) | repeated |  |






<a name="talos.resource.definitions.kubespan.EndpointSpec"></a>

### EndpointSpec
EndpointSpec describes Endpoint state.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| affiliate_id | [string](#string) |  |  |
| endpoint | [common.NetIPPort](#common.NetIPPort) |  |  |






<a name="talos.resource.definitions.kubespan.IdentitySpec"></a>

### IdentitySpec
IdentitySpec describes KubeSpan keys and address.

Note: IdentitySpec is persisted on disk in the STATE partition,
so YAML serialization should be kept backwards compatible.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| address | [common.NetIPPrefix](#common.NetIPPrefix) |  |  |
| subnet | [common.NetIPPrefix](#common.NetIPPrefix) |  |  |
| private_key | [string](#string) |  |  |
| public_key | [string](#string) |  |  |






<a name="talos.resource.definitions.kubespan.PeerSpecSpec"></a>

### PeerSpecSpec
PeerSpecSpec describes PeerSpec state.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| address | [common.NetIP](#common.NetIP) |  |  |
| allowed_ips | [common.NetIPPrefix](#common.NetIPPrefix) | repeated |  |
| endpoints | [common.NetIPPort](#common.NetIPPort) | repeated |  |
| label | [string](#string) |  |  |






<a name="talos.resource.definitions.kubespan.PeerStatusSpec"></a>

### PeerStatusSpec
PeerStatusSpec describes PeerStatus state.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| endpoint | [common.NetIPPort](#common.NetIPPort) |  |  |
| label | [string](#string) |  |  |
| state | [talos.resource.definitions.enums.KubespanPeerState](#talos.resource.definitions.enums.KubespanPeerState) |  |  |
| receive_bytes | [int64](#int64) |  |  |
| transmit_bytes | [int64](#int64) |  |  |
| last_handshake_time | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  |  |
| last_used_endpoint | [common.NetIPPort](#common.NetIPPort) |  |  |
| last_endpoint_change | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  |  |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="resource/definitions/network/network.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## resource/definitions/network/network.proto



<a name="talos.resource.definitions.network.AddressSpecSpec"></a>

### AddressSpecSpec
AddressSpecSpec describes status of rendered secrets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| address | [common.NetIPPrefix](#common.NetIPPrefix) |  |  |
| link_name | [string](#string) |  |  |
| family | [talos.resource.definitions.enums.NethelpersFamily](#talos.resource.definitions.enums.NethelpersFamily) |  |  |
| scope | [talos.resource.definitions.enums.NethelpersScope](#talos.resource.definitions.enums.NethelpersScope) |  |  |
| flags | [uint32](#uint32) |  |  |
| announce_with_arp | [bool](#bool) |  |  |
| config_layer | [talos.resource.definitions.enums.NetworkConfigLayer](#talos.resource.definitions.enums.NetworkConfigLayer) |  |  |
| priority | [uint32](#uint32) |  |  |






<a name="talos.resource.definitions.network.AddressStatusSpec"></a>

### AddressStatusSpec
AddressStatusSpec describes status of rendered secrets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| address | [common.NetIPPrefix](#common.NetIPPrefix) |  |  |
| local | [common.NetIP](#common.NetIP) |  |  |
| broadcast | [common.NetIP](#common.NetIP) |  |  |
| anycast | [common.NetIP](#common.NetIP) |  |  |
| multicast | [common.NetIP](#common.NetIP) |  |  |
| link_index | [uint32](#uint32) |  |  |
| link_name | [string](#string) |  |  |
| family | [talos.resource.definitions.enums.NethelpersFamily](#talos.resource.definitions.enums.NethelpersFamily) |  |  |
| scope | [talos.resource.definitions.enums.NethelpersScope](#talos.resource.definitions.enums.NethelpersScope) |  |  |
| flags | [uint32](#uint32) |  |  |
| priority | [uint32](#uint32) |  |  |






<a name="talos.resource.definitions.network.BondMasterSpec"></a>

### BondMasterSpec
BondMasterSpec describes bond settings if Kind == "bond".


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| mode | [talos.resource.definitions.enums.NethelpersBondMode](#talos.resource.definitions.enums.NethelpersBondMode) |  |  |
| hash_policy | [talos.resource.definitions.enums.NethelpersBondXmitHashPolicy](#talos.resource.definitions.enums.NethelpersBondXmitHashPolicy) |  |  |
| lacp_rate | [talos.resource.definitions.enums.NethelpersLACPRate](#talos.resource.definitions.enums.NethelpersLACPRate) |  |  |
| arp_validate | [talos.resource.definitions.enums.NethelpersARPValidate](#talos.resource.definitions.enums.NethelpersARPValidate) |  |  |
| arp_all_targets | [talos.resource.definitions.enums.NethelpersARPAllTargets](#talos.resource.definitions.enums.NethelpersARPAllTargets) |  |  |
| primary_index | [uint32](#uint32) |  |  |
| primary_reselect | [talos.resource.definitions.enums.NethelpersPrimaryReselect](#talos.resource.definitions.enums.NethelpersPrimaryReselect) |  |  |
| fail_over_mac | [talos.resource.definitions.enums.NethelpersFailOverMAC](#talos.resource.definitions.enums.NethelpersFailOverMAC) |  |  |
| ad_select | [talos.resource.definitions.enums.NethelpersADSelect](#talos.resource.definitions.enums.NethelpersADSelect) |  |  |
| mii_mon | [uint32](#uint32) |  |  |
| up_delay | [uint32](#uint32) |  |  |
| down_delay | [uint32](#uint32) |  |  |
| arp_interval | [uint32](#uint32) |  |  |
| resend_igmp | [uint32](#uint32) |  |  |
| min_links | [uint32](#uint32) |  |  |
| lp_interval | [uint32](#uint32) |  |  |
| packets_per_slave | [uint32](#uint32) |  |  |
| num_peer_notif | [fixed32](#fixed32) |  |  |
| tlb_dynamic_lb | [fixed32](#fixed32) |  |  |
| all_slaves_active | [fixed32](#fixed32) |  |  |
| use_carrier | [bool](#bool) |  |  |
| ad_actor_sys_prio | [fixed32](#fixed32) |  |  |
| ad_user_port_key | [fixed32](#fixed32) |  |  |
| peer_notify_delay | [uint32](#uint32) |  |  |






<a name="talos.resource.definitions.network.BondSlave"></a>

### BondSlave
BondSlave contains a bond's master name and slave index.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| master_name | [string](#string) |  |  |
| slave_index | [int64](#int64) |  |  |






<a name="talos.resource.definitions.network.BridgeMasterSpec"></a>

### BridgeMasterSpec
BridgeMasterSpec describes bridge settings if Kind == "bridge".


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| stp | [STPSpec](#talos.resource.definitions.network.STPSpec) |  |  |
| vlan | [BridgeVLANSpec](#talos.resource.definitions.network.BridgeVLANSpec) |  |  |






<a name="talos.resource.definitions.network.BridgeSlave"></a>

### BridgeSlave
BridgeSlave contains the name of the master bridge of a bridged interface


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| master_name | [string](#string) |  |  |






<a name="talos.resource.definitions.network.BridgeVLANSpec"></a>

### BridgeVLANSpec
BridgeVLANSpec describes VLAN settings of a bridge.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| filtering_enabled | [bool](#bool) |  |  |






<a name="talos.resource.definitions.network.DHCP4OperatorSpec"></a>

### DHCP4OperatorSpec
DHCP4OperatorSpec describes DHCP4 operator options.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| route_metric | [uint32](#uint32) |  |  |
| skip_hostname_request | [bool](#bool) |  |  |






<a name="talos.resource.definitions.network.DHCP6OperatorSpec"></a>

### DHCP6OperatorSpec
DHCP6OperatorSpec describes DHCP6 operator options.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| duid | [string](#string) |  |  |
| route_metric | [uint32](#uint32) |  |  |
| skip_hostname_request | [bool](#bool) |  |  |






<a name="talos.resource.definitions.network.DNSResolveCacheSpec"></a>

### DNSResolveCacheSpec
DNSResolveCacheSpec describes DNS servers status.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| status | [string](#string) |  |  |






<a name="talos.resource.definitions.network.EthernetChannelsSpec"></a>

### EthernetChannelsSpec
EthernetChannelsSpec describes config of Ethernet channels.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| rx | [uint32](#uint32) |  |  |
| tx | [uint32](#uint32) |  |  |
| other | [uint32](#uint32) |  |  |
| combined | [uint32](#uint32) |  |  |






<a name="talos.resource.definitions.network.EthernetChannelsStatus"></a>

### EthernetChannelsStatus
EthernetChannelsStatus describes status of Ethernet channels.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| rx_max | [uint32](#uint32) |  |  |
| tx_max | [uint32](#uint32) |  |  |
| other_max | [uint32](#uint32) |  |  |
| combined_max | [uint32](#uint32) |  |  |
| rx | [uint32](#uint32) |  |  |
| tx | [uint32](#uint32) |  |  |
| other | [uint32](#uint32) |  |  |
| combined | [uint32](#uint32) |  |  |






<a name="talos.resource.definitions.network.EthernetFeatureStatus"></a>

### EthernetFeatureStatus
EthernetFeatureStatus describes status of Ethernet features.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| status | [string](#string) |  |  |






<a name="talos.resource.definitions.network.EthernetRingsSpec"></a>

### EthernetRingsSpec
EthernetRingsSpec describes config of Ethernet rings.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| rx | [uint32](#uint32) |  |  |
| rx_mini | [uint32](#uint32) |  |  |
| rx_jumbo | [uint32](#uint32) |  |  |
| tx | [uint32](#uint32) |  |  |
| rx_buf_len | [uint32](#uint32) |  |  |
| cqe_size | [uint32](#uint32) |  |  |
| tx_push | [bool](#bool) |  |  |
| rx_push | [bool](#bool) |  |  |
| tx_push_buf_len | [uint32](#uint32) |  |  |
| tcp_data_split | [bool](#bool) |  |  |






<a name="talos.resource.definitions.network.EthernetRingsStatus"></a>

### EthernetRingsStatus
EthernetRingsStatus describes status of Ethernet rings.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| rx_max | [uint32](#uint32) |  |  |
| rx_mini_max | [uint32](#uint32) |  |  |
| rx_jumbo_max | [uint32](#uint32) |  |  |
| tx_max | [uint32](#uint32) |  |  |
| tx_push_buf_len_max | [uint32](#uint32) |  |  |
| rx | [uint32](#uint32) |  |  |
| rx_mini | [uint32](#uint32) |  |  |
| rx_jumbo | [uint32](#uint32) |  |  |
| tx | [uint32](#uint32) |  |  |
| rx_buf_len | [uint32](#uint32) |  |  |
| cqe_size | [uint32](#uint32) |  |  |
| tx_push | [bool](#bool) |  |  |
| rx_push | [bool](#bool) |  |  |
| tx_push_buf_len | [uint32](#uint32) |  |  |
| tcp_data_split | [bool](#bool) |  |  |






<a name="talos.resource.definitions.network.EthernetSpecSpec"></a>

### EthernetSpecSpec
EthernetSpecSpec describes config of Ethernet link.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| rings | [EthernetRingsSpec](#talos.resource.definitions.network.EthernetRingsSpec) |  |  |
| features | [EthernetSpecSpec.FeaturesEntry](#talos.resource.definitions.network.EthernetSpecSpec.FeaturesEntry) | repeated |  |
| channels | [EthernetChannelsSpec](#talos.resource.definitions.network.EthernetChannelsSpec) |  |  |






<a name="talos.resource.definitions.network.EthernetSpecSpec.FeaturesEntry"></a>

### EthernetSpecSpec.FeaturesEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [bool](#bool) |  |  |






<a name="talos.resource.definitions.network.EthernetStatusSpec"></a>

### EthernetStatusSpec
EthernetStatusSpec describes status of rendered secrets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| link_state | [bool](#bool) |  |  |
| speed_megabits | [int64](#int64) |  |  |
| port | [talos.resource.definitions.enums.NethelpersPort](#talos.resource.definitions.enums.NethelpersPort) |  |  |
| duplex | [talos.resource.definitions.enums.NethelpersDuplex](#talos.resource.definitions.enums.NethelpersDuplex) |  |  |
| our_modes | [string](#string) | repeated |  |
| peer_modes | [string](#string) | repeated |  |
| rings | [EthernetRingsStatus](#talos.resource.definitions.network.EthernetRingsStatus) |  |  |
| features | [EthernetFeatureStatus](#talos.resource.definitions.network.EthernetFeatureStatus) | repeated |  |
| channels | [EthernetChannelsStatus](#talos.resource.definitions.network.EthernetChannelsStatus) |  |  |






<a name="talos.resource.definitions.network.HardwareAddrSpec"></a>

### HardwareAddrSpec
HardwareAddrSpec describes spec for the link.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| hardware_addr | [bytes](#bytes) |  |  |






<a name="talos.resource.definitions.network.HostDNSConfigSpec"></a>

### HostDNSConfigSpec
HostDNSConfigSpec describes host DNS config.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| enabled | [bool](#bool) |  |  |
| listen_addresses | [common.NetIPPort](#common.NetIPPort) | repeated |  |
| service_host_dns_address | [common.NetIP](#common.NetIP) |  |  |
| resolve_member_names | [bool](#bool) |  |  |






<a name="talos.resource.definitions.network.HostnameSpecSpec"></a>

### HostnameSpecSpec
HostnameSpecSpec describes node hostname.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| hostname | [string](#string) |  |  |
| domainname | [string](#string) |  |  |
| config_layer | [talos.resource.definitions.enums.NetworkConfigLayer](#talos.resource.definitions.enums.NetworkConfigLayer) |  |  |






<a name="talos.resource.definitions.network.HostnameStatusSpec"></a>

### HostnameStatusSpec
HostnameStatusSpec describes node hostname.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| hostname | [string](#string) |  |  |
| domainname | [string](#string) |  |  |






<a name="talos.resource.definitions.network.LinkRefreshSpec"></a>

### LinkRefreshSpec
LinkRefreshSpec describes status of rendered secrets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| generation | [int64](#int64) |  |  |






<a name="talos.resource.definitions.network.LinkSpecSpec"></a>

### LinkSpecSpec
LinkSpecSpec describes spec for the link.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| logical | [bool](#bool) |  |  |
| up | [bool](#bool) |  |  |
| mtu | [uint32](#uint32) |  |  |
| kind | [string](#string) |  |  |
| type | [talos.resource.definitions.enums.NethelpersLinkType](#talos.resource.definitions.enums.NethelpersLinkType) |  |  |
| parent_name | [string](#string) |  |  |
| bond_slave | [BondSlave](#talos.resource.definitions.network.BondSlave) |  |  |
| bridge_slave | [BridgeSlave](#talos.resource.definitions.network.BridgeSlave) |  |  |
| vlan | [VLANSpec](#talos.resource.definitions.network.VLANSpec) |  |  |
| bond_master | [BondMasterSpec](#talos.resource.definitions.network.BondMasterSpec) |  |  |
| bridge_master | [BridgeMasterSpec](#talos.resource.definitions.network.BridgeMasterSpec) |  |  |
| wireguard | [WireguardSpec](#talos.resource.definitions.network.WireguardSpec) |  |  |
| config_layer | [talos.resource.definitions.enums.NetworkConfigLayer](#talos.resource.definitions.enums.NetworkConfigLayer) |  |  |






<a name="talos.resource.definitions.network.LinkStatusSpec"></a>

### LinkStatusSpec
LinkStatusSpec describes status of rendered secrets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| index | [uint32](#uint32) |  |  |
| type | [talos.resource.definitions.enums.NethelpersLinkType](#talos.resource.definitions.enums.NethelpersLinkType) |  |  |
| link_index | [uint32](#uint32) |  |  |
| flags | [uint32](#uint32) |  |  |
| hardware_addr | [bytes](#bytes) |  |  |
| broadcast_addr | [bytes](#bytes) |  |  |
| mtu | [uint32](#uint32) |  |  |
| queue_disc | [string](#string) |  |  |
| master_index | [uint32](#uint32) |  |  |
| operational_state | [talos.resource.definitions.enums.NethelpersOperationalState](#talos.resource.definitions.enums.NethelpersOperationalState) |  |  |
| kind | [string](#string) |  |  |
| slave_kind | [string](#string) |  |  |
| bus_path | [string](#string) |  |  |
| pciid | [string](#string) |  |  |
| driver | [string](#string) |  |  |
| driver_version | [string](#string) |  |  |
| firmware_version | [string](#string) |  |  |
| product_id | [string](#string) |  |  |
| vendor_id | [string](#string) |  |  |
| product | [string](#string) |  |  |
| vendor | [string](#string) |  |  |
| link_state | [bool](#bool) |  |  |
| speed_megabits | [int64](#int64) |  |  |
| port | [talos.resource.definitions.enums.NethelpersPort](#talos.resource.definitions.enums.NethelpersPort) |  |  |
| duplex | [talos.resource.definitions.enums.NethelpersDuplex](#talos.resource.definitions.enums.NethelpersDuplex) |  |  |
| vlan | [VLANSpec](#talos.resource.definitions.network.VLANSpec) |  |  |
| bridge_master | [BridgeMasterSpec](#talos.resource.definitions.network.BridgeMasterSpec) |  |  |
| bond_master | [BondMasterSpec](#talos.resource.definitions.network.BondMasterSpec) |  |  |
| wireguard | [WireguardSpec](#talos.resource.definitions.network.WireguardSpec) |  |  |
| permanent_addr | [bytes](#bytes) |  |  |
| alias | [string](#string) |  |  |
| alt_names | [string](#string) | repeated |  |






<a name="talos.resource.definitions.network.NfTablesAddressMatch"></a>

### NfTablesAddressMatch
NfTablesAddressMatch describes the match on the IP address.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| include_subnets | [common.NetIPPrefix](#common.NetIPPrefix) | repeated |  |
| exclude_subnets | [common.NetIPPrefix](#common.NetIPPrefix) | repeated |  |
| invert | [bool](#bool) |  |  |






<a name="talos.resource.definitions.network.NfTablesChainSpec"></a>

### NfTablesChainSpec
NfTablesChainSpec describes status of rendered secrets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| type | [string](#string) |  |  |
| hook | [talos.resource.definitions.enums.NethelpersNfTablesChainHook](#talos.resource.definitions.enums.NethelpersNfTablesChainHook) |  |  |
| priority | [talos.resource.definitions.enums.NethelpersNfTablesChainPriority](#talos.resource.definitions.enums.NethelpersNfTablesChainPriority) |  |  |
| rules | [NfTablesRule](#talos.resource.definitions.network.NfTablesRule) | repeated |  |
| policy | [talos.resource.definitions.enums.NethelpersNfTablesVerdict](#talos.resource.definitions.enums.NethelpersNfTablesVerdict) |  |  |






<a name="talos.resource.definitions.network.NfTablesClampMSS"></a>

### NfTablesClampMSS
NfTablesClampMSS describes the TCP MSS clamping operation.

MSS is limited by the `MaxMTU` so that:
- IPv4: MSS = MaxMTU - 40
- IPv6: MSS = MaxMTU - 60.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| mtu | [fixed32](#fixed32) |  |  |






<a name="talos.resource.definitions.network.NfTablesConntrackStateMatch"></a>

### NfTablesConntrackStateMatch
NfTablesConntrackStateMatch describes the match on the connection tracking state.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| states | [talos.resource.definitions.enums.NethelpersConntrackState](#talos.resource.definitions.enums.NethelpersConntrackState) | repeated |  |






<a name="talos.resource.definitions.network.NfTablesIfNameMatch"></a>

### NfTablesIfNameMatch
NfTablesIfNameMatch describes the match on the interface name.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| operator | [talos.resource.definitions.enums.NethelpersMatchOperator](#talos.resource.definitions.enums.NethelpersMatchOperator) |  |  |
| interface_names | [string](#string) | repeated |  |






<a name="talos.resource.definitions.network.NfTablesLayer4Match"></a>

### NfTablesLayer4Match
NfTablesLayer4Match describes the match on the transport layer protocol.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| protocol | [talos.resource.definitions.enums.NethelpersProtocol](#talos.resource.definitions.enums.NethelpersProtocol) |  |  |
| match_source_port | [NfTablesPortMatch](#talos.resource.definitions.network.NfTablesPortMatch) |  |  |
| match_destination_port | [NfTablesPortMatch](#talos.resource.definitions.network.NfTablesPortMatch) |  |  |






<a name="talos.resource.definitions.network.NfTablesLimitMatch"></a>

### NfTablesLimitMatch
NfTablesLimitMatch describes the match on the packet rate.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| packet_rate_per_second | [uint64](#uint64) |  |  |






<a name="talos.resource.definitions.network.NfTablesMark"></a>

### NfTablesMark
NfTablesMark encodes packet mark match/update operation.

When used as a match computes the following condition:
(mark & mask) ^ xor == value

When used as an update computes the following operation:
mark = (mark & mask) ^ xor.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| mask | [uint32](#uint32) |  |  |
| xor | [uint32](#uint32) |  |  |
| value | [uint32](#uint32) |  |  |






<a name="talos.resource.definitions.network.NfTablesPortMatch"></a>

### NfTablesPortMatch
NfTablesPortMatch describes the match on the transport layer port.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ranges | [PortRange](#talos.resource.definitions.network.PortRange) | repeated |  |






<a name="talos.resource.definitions.network.NfTablesRule"></a>

### NfTablesRule
NfTablesRule describes a single rule in the nftables chain.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| match_o_if_name | [NfTablesIfNameMatch](#talos.resource.definitions.network.NfTablesIfNameMatch) |  |  |
| verdict | [talos.resource.definitions.enums.NethelpersNfTablesVerdict](#talos.resource.definitions.enums.NethelpersNfTablesVerdict) |  |  |
| match_mark | [NfTablesMark](#talos.resource.definitions.network.NfTablesMark) |  |  |
| set_mark | [NfTablesMark](#talos.resource.definitions.network.NfTablesMark) |  |  |
| match_source_address | [NfTablesAddressMatch](#talos.resource.definitions.network.NfTablesAddressMatch) |  |  |
| match_destination_address | [NfTablesAddressMatch](#talos.resource.definitions.network.NfTablesAddressMatch) |  |  |
| match_layer4 | [NfTablesLayer4Match](#talos.resource.definitions.network.NfTablesLayer4Match) |  |  |
| match_i_if_name | [NfTablesIfNameMatch](#talos.resource.definitions.network.NfTablesIfNameMatch) |  |  |
| clamp_mss | [NfTablesClampMSS](#talos.resource.definitions.network.NfTablesClampMSS) |  |  |
| match_limit | [NfTablesLimitMatch](#talos.resource.definitions.network.NfTablesLimitMatch) |  |  |
| match_conntrack_state | [NfTablesConntrackStateMatch](#talos.resource.definitions.network.NfTablesConntrackStateMatch) |  |  |
| anon_counter | [bool](#bool) |  |  |






<a name="talos.resource.definitions.network.NodeAddressFilterSpec"></a>

### NodeAddressFilterSpec
NodeAddressFilterSpec describes a filter for NodeAddresses.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| include_subnets | [common.NetIPPrefix](#common.NetIPPrefix) | repeated |  |
| exclude_subnets | [common.NetIPPrefix](#common.NetIPPrefix) | repeated |  |






<a name="talos.resource.definitions.network.NodeAddressSortAlgorithmSpec"></a>

### NodeAddressSortAlgorithmSpec
NodeAddressSortAlgorithmSpec describes a filter for NodeAddresses.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| algorithm | [talos.resource.definitions.enums.NethelpersAddressSortAlgorithm](#talos.resource.definitions.enums.NethelpersAddressSortAlgorithm) |  |  |






<a name="talos.resource.definitions.network.NodeAddressSpec"></a>

### NodeAddressSpec
NodeAddressSpec describes a set of node addresses.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| addresses | [common.NetIPPrefix](#common.NetIPPrefix) | repeated |  |
| sort_algorithm | [talos.resource.definitions.enums.NethelpersAddressSortAlgorithm](#talos.resource.definitions.enums.NethelpersAddressSortAlgorithm) |  |  |






<a name="talos.resource.definitions.network.OperatorSpecSpec"></a>

### OperatorSpecSpec
OperatorSpecSpec describes DNS resolvers.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| operator | [talos.resource.definitions.enums.NetworkOperator](#talos.resource.definitions.enums.NetworkOperator) |  |  |
| link_name | [string](#string) |  |  |
| require_up | [bool](#bool) |  |  |
| dhcp4 | [DHCP4OperatorSpec](#talos.resource.definitions.network.DHCP4OperatorSpec) |  |  |
| dhcp6 | [DHCP6OperatorSpec](#talos.resource.definitions.network.DHCP6OperatorSpec) |  |  |
| vip | [VIPOperatorSpec](#talos.resource.definitions.network.VIPOperatorSpec) |  |  |
| config_layer | [talos.resource.definitions.enums.NetworkConfigLayer](#talos.resource.definitions.enums.NetworkConfigLayer) |  |  |






<a name="talos.resource.definitions.network.PortRange"></a>

### PortRange
PortRange describes a range of ports.

Range is [lo, hi].


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| lo | [fixed32](#fixed32) |  |  |
| hi | [fixed32](#fixed32) |  |  |






<a name="talos.resource.definitions.network.ProbeSpecSpec"></a>

### ProbeSpecSpec
ProbeSpecSpec describes the Probe.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| interval | [google.protobuf.Duration](#google.protobuf.Duration) |  |  |
| failure_threshold | [int64](#int64) |  |  |
| tcp | [TCPProbeSpec](#talos.resource.definitions.network.TCPProbeSpec) |  |  |
| config_layer | [talos.resource.definitions.enums.NetworkConfigLayer](#talos.resource.definitions.enums.NetworkConfigLayer) |  |  |






<a name="talos.resource.definitions.network.ProbeStatusSpec"></a>

### ProbeStatusSpec
ProbeStatusSpec describes the Probe.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| success | [bool](#bool) |  |  |
| last_error | [string](#string) |  |  |






<a name="talos.resource.definitions.network.ResolverSpecSpec"></a>

### ResolverSpecSpec
ResolverSpecSpec describes DNS resolvers.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| dns_servers | [common.NetIP](#common.NetIP) | repeated |  |
| config_layer | [talos.resource.definitions.enums.NetworkConfigLayer](#talos.resource.definitions.enums.NetworkConfigLayer) |  |  |
| search_domains | [string](#string) | repeated |  |






<a name="talos.resource.definitions.network.ResolverStatusSpec"></a>

### ResolverStatusSpec
ResolverStatusSpec describes DNS resolvers.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| dns_servers | [common.NetIP](#common.NetIP) | repeated |  |
| search_domains | [string](#string) | repeated |  |






<a name="talos.resource.definitions.network.RouteSpecSpec"></a>

### RouteSpecSpec
RouteSpecSpec describes the route.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| family | [talos.resource.definitions.enums.NethelpersFamily](#talos.resource.definitions.enums.NethelpersFamily) |  |  |
| destination | [common.NetIPPrefix](#common.NetIPPrefix) |  |  |
| source | [common.NetIP](#common.NetIP) |  |  |
| gateway | [common.NetIP](#common.NetIP) |  |  |
| out_link_name | [string](#string) |  |  |
| table | [talos.resource.definitions.enums.NethelpersRoutingTable](#talos.resource.definitions.enums.NethelpersRoutingTable) |  |  |
| priority | [uint32](#uint32) |  |  |
| scope | [talos.resource.definitions.enums.NethelpersScope](#talos.resource.definitions.enums.NethelpersScope) |  |  |
| type | [talos.resource.definitions.enums.NethelpersRouteType](#talos.resource.definitions.enums.NethelpersRouteType) |  |  |
| flags | [uint32](#uint32) |  |  |
| protocol | [talos.resource.definitions.enums.NethelpersRouteProtocol](#talos.resource.definitions.enums.NethelpersRouteProtocol) |  |  |
| config_layer | [talos.resource.definitions.enums.NetworkConfigLayer](#talos.resource.definitions.enums.NetworkConfigLayer) |  |  |
| mtu | [uint32](#uint32) |  |  |






<a name="talos.resource.definitions.network.RouteStatusSpec"></a>

### RouteStatusSpec
RouteStatusSpec describes status of rendered secrets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| family | [talos.resource.definitions.enums.NethelpersFamily](#talos.resource.definitions.enums.NethelpersFamily) |  |  |
| destination | [common.NetIPPrefix](#common.NetIPPrefix) |  |  |
| source | [common.NetIP](#common.NetIP) |  |  |
| gateway | [common.NetIP](#common.NetIP) |  |  |
| out_link_index | [uint32](#uint32) |  |  |
| out_link_name | [string](#string) |  |  |
| table | [talos.resource.definitions.enums.NethelpersRoutingTable](#talos.resource.definitions.enums.NethelpersRoutingTable) |  |  |
| priority | [uint32](#uint32) |  |  |
| scope | [talos.resource.definitions.enums.NethelpersScope](#talos.resource.definitions.enums.NethelpersScope) |  |  |
| type | [talos.resource.definitions.enums.NethelpersRouteType](#talos.resource.definitions.enums.NethelpersRouteType) |  |  |
| flags | [uint32](#uint32) |  |  |
| protocol | [talos.resource.definitions.enums.NethelpersRouteProtocol](#talos.resource.definitions.enums.NethelpersRouteProtocol) |  |  |
| mtu | [uint32](#uint32) |  |  |






<a name="talos.resource.definitions.network.STPSpec"></a>

### STPSpec
STPSpec describes Spanning Tree Protocol (STP) settings of a bridge.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| enabled | [bool](#bool) |  |  |






<a name="talos.resource.definitions.network.StatusSpec"></a>

### StatusSpec
StatusSpec describes network state.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| address_ready | [bool](#bool) |  |  |
| connectivity_ready | [bool](#bool) |  |  |
| hostname_ready | [bool](#bool) |  |  |
| etc_files_ready | [bool](#bool) |  |  |






<a name="talos.resource.definitions.network.TCPProbeSpec"></a>

### TCPProbeSpec
TCPProbeSpec describes the TCP Probe.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| endpoint | [string](#string) |  |  |
| timeout | [google.protobuf.Duration](#google.protobuf.Duration) |  |  |






<a name="talos.resource.definitions.network.TimeServerSpecSpec"></a>

### TimeServerSpecSpec
TimeServerSpecSpec describes NTP servers.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ntp_servers | [string](#string) | repeated |  |
| config_layer | [talos.resource.definitions.enums.NetworkConfigLayer](#talos.resource.definitions.enums.NetworkConfigLayer) |  |  |






<a name="talos.resource.definitions.network.TimeServerStatusSpec"></a>

### TimeServerStatusSpec
TimeServerStatusSpec describes NTP servers.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ntp_servers | [string](#string) | repeated |  |






<a name="talos.resource.definitions.network.VIPEquinixMetalSpec"></a>

### VIPEquinixMetalSpec
VIPEquinixMetalSpec describes virtual (elastic) IP settings for Equinix Metal.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| project_id | [string](#string) |  |  |
| device_id | [string](#string) |  |  |
| api_token | [string](#string) |  |  |






<a name="talos.resource.definitions.network.VIPHCloudSpec"></a>

### VIPHCloudSpec
VIPHCloudSpec describes virtual (elastic) IP settings for Hetzner Cloud.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| device_id | [int64](#int64) |  |  |
| network_id | [int64](#int64) |  |  |
| api_token | [string](#string) |  |  |






<a name="talos.resource.definitions.network.VIPOperatorSpec"></a>

### VIPOperatorSpec
VIPOperatorSpec describes virtual IP operator options.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ip | [common.NetIP](#common.NetIP) |  |  |
| gratuitous_arp | [bool](#bool) |  |  |
| equinix_metal | [VIPEquinixMetalSpec](#talos.resource.definitions.network.VIPEquinixMetalSpec) |  |  |
| h_cloud | [VIPHCloudSpec](#talos.resource.definitions.network.VIPHCloudSpec) |  |  |






<a name="talos.resource.definitions.network.VLANSpec"></a>

### VLANSpec
VLANSpec describes VLAN settings if Kind == "vlan".


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| vid | [fixed32](#fixed32) |  |  |
| protocol | [talos.resource.definitions.enums.NethelpersVLANProtocol](#talos.resource.definitions.enums.NethelpersVLANProtocol) |  |  |






<a name="talos.resource.definitions.network.WireguardPeer"></a>

### WireguardPeer
WireguardPeer describes a single peer.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| public_key | [string](#string) |  |  |
| preshared_key | [string](#string) |  |  |
| endpoint | [string](#string) |  |  |
| persistent_keepalive_interval | [google.protobuf.Duration](#google.protobuf.Duration) |  |  |
| allowed_ips | [common.NetIPPrefix](#common.NetIPPrefix) | repeated |  |






<a name="talos.resource.definitions.network.WireguardSpec"></a>

### WireguardSpec
WireguardSpec describes Wireguard settings if Kind == "wireguard".


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| private_key | [string](#string) |  |  |
| public_key | [string](#string) |  |  |
| listen_port | [int64](#int64) |  |  |
| firewall_mark | [int64](#int64) |  |  |
| peers | [WireguardPeer](#talos.resource.definitions.network.WireguardPeer) | repeated |  |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="resource/definitions/perf/perf.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## resource/definitions/perf/perf.proto



<a name="talos.resource.definitions.perf.CPUSpec"></a>

### CPUSpec
CPUSpec represents the last CPU stats snapshot.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| cpu | [CPUStat](#talos.resource.definitions.perf.CPUStat) | repeated |  |
| cpu_total | [CPUStat](#talos.resource.definitions.perf.CPUStat) |  |  |
| irq_total | [uint64](#uint64) |  |  |
| context_switches | [uint64](#uint64) |  |  |
| process_created | [uint64](#uint64) |  |  |
| process_running | [uint64](#uint64) |  |  |
| process_blocked | [uint64](#uint64) |  |  |
| soft_irq_total | [uint64](#uint64) |  |  |






<a name="talos.resource.definitions.perf.CPUStat"></a>

### CPUStat
CPUStat represents a single cpu stat.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| user | [double](#double) |  |  |
| nice | [double](#double) |  |  |
| system | [double](#double) |  |  |
| idle | [double](#double) |  |  |
| iowait | [double](#double) |  |  |
| irq | [double](#double) |  |  |
| soft_irq | [double](#double) |  |  |
| steal | [double](#double) |  |  |
| guest | [double](#double) |  |  |
| guest_nice | [double](#double) |  |  |






<a name="talos.resource.definitions.perf.MemorySpec"></a>

### MemorySpec
MemorySpec represents the last Memory stats snapshot.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| mem_total | [uint64](#uint64) |  |  |
| mem_used | [uint64](#uint64) |  |  |
| mem_available | [uint64](#uint64) |  |  |
| buffers | [uint64](#uint64) |  |  |
| cached | [uint64](#uint64) |  |  |
| swap_cached | [uint64](#uint64) |  |  |
| active | [uint64](#uint64) |  |  |
| inactive | [uint64](#uint64) |  |  |
| active_anon | [uint64](#uint64) |  |  |
| inactive_anon | [uint64](#uint64) |  |  |
| active_file | [uint64](#uint64) |  |  |
| inactive_file | [uint64](#uint64) |  |  |
| unevictable | [uint64](#uint64) |  |  |
| mlocked | [uint64](#uint64) |  |  |
| swap_total | [uint64](#uint64) |  |  |
| swap_free | [uint64](#uint64) |  |  |
| dirty | [uint64](#uint64) |  |  |
| writeback | [uint64](#uint64) |  |  |
| anon_pages | [uint64](#uint64) |  |  |
| mapped | [uint64](#uint64) |  |  |
| shmem | [uint64](#uint64) |  |  |
| slab | [uint64](#uint64) |  |  |
| s_reclaimable | [uint64](#uint64) |  |  |
| s_unreclaim | [uint64](#uint64) |  |  |
| kernel_stack | [uint64](#uint64) |  |  |
| page_tables | [uint64](#uint64) |  |  |
| nf_sunstable | [uint64](#uint64) |  |  |
| bounce | [uint64](#uint64) |  |  |
| writeback_tmp | [uint64](#uint64) |  |  |
| commit_limit | [uint64](#uint64) |  |  |
| committed_as | [uint64](#uint64) |  |  |
| vmalloc_total | [uint64](#uint64) |  |  |
| vmalloc_used | [uint64](#uint64) |  |  |
| vmalloc_chunk | [uint64](#uint64) |  |  |
| hardware_corrupted | [uint64](#uint64) |  |  |
| anon_huge_pages | [uint64](#uint64) |  |  |
| shmem_huge_pages | [uint64](#uint64) |  |  |
| shmem_pmd_mapped | [uint64](#uint64) |  |  |
| cma_total | [uint64](#uint64) |  |  |
| cma_free | [uint64](#uint64) |  |  |
| huge_pages_total | [uint64](#uint64) |  |  |
| huge_pages_free | [uint64](#uint64) |  |  |
| huge_pages_rsvd | [uint64](#uint64) |  |  |
| huge_pages_surp | [uint64](#uint64) |  |  |
| hugepagesize | [uint64](#uint64) |  |  |
| direct_map4k | [uint64](#uint64) |  |  |
| direct_map2m | [uint64](#uint64) |  |  |
| direct_map1g | [uint64](#uint64) |  |  |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="resource/definitions/proto/proto.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## resource/definitions/proto/proto.proto



<a name="talos.resource.definitions.proto.LinuxIDMapping"></a>

### LinuxIDMapping
LinuxIDMapping specifies UID/GID mappings.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| container_id | [uint32](#uint32) |  |  |
| host_id | [uint32](#uint32) |  |  |
| size | [uint32](#uint32) |  |  |






<a name="talos.resource.definitions.proto.Mount"></a>

### Mount
Mount specifies a mount for a container.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| destination | [string](#string) |  |  |
| type | [string](#string) |  |  |
| source | [string](#string) |  |  |
| options | [string](#string) | repeated |  |
| uid_mappings | [LinuxIDMapping](#talos.resource.definitions.proto.LinuxIDMapping) | repeated |  |
| gid_mappings | [LinuxIDMapping](#talos.resource.definitions.proto.LinuxIDMapping) | repeated |  |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="resource/definitions/runtime/runtime.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## resource/definitions/runtime/runtime.proto



<a name="talos.resource.definitions.runtime.DevicesStatusSpec"></a>

### DevicesStatusSpec
DevicesStatusSpec is the spec for devices status.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ready | [bool](#bool) |  |  |






<a name="talos.resource.definitions.runtime.DiagnosticSpec"></a>

### DiagnosticSpec
DiagnosticSpec is the spec for devices status.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| message | [string](#string) |  |  |
| details | [string](#string) | repeated |  |






<a name="talos.resource.definitions.runtime.EventSinkConfigSpec"></a>

### EventSinkConfigSpec
EventSinkConfigSpec describes configuration of Talos event log streaming.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| endpoint | [string](#string) |  |  |






<a name="talos.resource.definitions.runtime.ExtensionServiceConfigFile"></a>

### ExtensionServiceConfigFile
ExtensionServiceConfigFile describes extensions service config files.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| content | [string](#string) |  |  |
| mount_path | [string](#string) |  |  |






<a name="talos.resource.definitions.runtime.ExtensionServiceConfigSpec"></a>

### ExtensionServiceConfigSpec
ExtensionServiceConfigSpec describes status of rendered extensions service config files.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| files | [ExtensionServiceConfigFile](#talos.resource.definitions.runtime.ExtensionServiceConfigFile) | repeated |  |
| environment | [string](#string) | repeated |  |






<a name="talos.resource.definitions.runtime.ExtensionServiceConfigStatusSpec"></a>

### ExtensionServiceConfigStatusSpec
ExtensionServiceConfigStatusSpec describes status of rendered extensions service config files.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| spec_version | [string](#string) |  |  |






<a name="talos.resource.definitions.runtime.KernelModuleSpecSpec"></a>

### KernelModuleSpecSpec
KernelModuleSpecSpec describes Linux kernel module to load.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| parameters | [string](#string) | repeated |  |






<a name="talos.resource.definitions.runtime.KernelParamSpecSpec"></a>

### KernelParamSpecSpec
KernelParamSpecSpec describes status of the defined sysctls.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| value | [string](#string) |  |  |
| ignore_errors | [bool](#bool) |  |  |






<a name="talos.resource.definitions.runtime.KernelParamStatusSpec"></a>

### KernelParamStatusSpec
KernelParamStatusSpec describes status of the defined sysctls.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| current | [string](#string) |  |  |
| default | [string](#string) |  |  |
| unsupported | [bool](#bool) |  |  |






<a name="talos.resource.definitions.runtime.KmsgLogConfigSpec"></a>

### KmsgLogConfigSpec
KmsgLogConfigSpec describes configuration for kmsg log streaming.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| destinations | [common.URL](#common.URL) | repeated |  |






<a name="talos.resource.definitions.runtime.MachineStatusSpec"></a>

### MachineStatusSpec
MachineStatusSpec describes status of the defined sysctls.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| stage | [talos.resource.definitions.enums.RuntimeMachineStage](#talos.resource.definitions.enums.RuntimeMachineStage) |  |  |
| status | [MachineStatusStatus](#talos.resource.definitions.runtime.MachineStatusStatus) |  |  |






<a name="talos.resource.definitions.runtime.MachineStatusStatus"></a>

### MachineStatusStatus
MachineStatusStatus describes machine current status at the stage.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ready | [bool](#bool) |  |  |
| unmet_conditions | [UnmetCondition](#talos.resource.definitions.runtime.UnmetCondition) | repeated |  |






<a name="talos.resource.definitions.runtime.MaintenanceServiceConfigSpec"></a>

### MaintenanceServiceConfigSpec
MaintenanceServiceConfigSpec describes configuration for maintenance service API.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| listen_address | [string](#string) |  |  |
| reachable_addresses | [common.NetIP](#common.NetIP) | repeated |  |






<a name="talos.resource.definitions.runtime.MetaKeySpec"></a>

### MetaKeySpec
MetaKeySpec describes status of the defined sysctls.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| value | [string](#string) |  |  |






<a name="talos.resource.definitions.runtime.MetaLoadedSpec"></a>

### MetaLoadedSpec
MetaLoadedSpec is the spec for meta loaded. The Done field is always true when resource exists.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| done | [bool](#bool) |  |  |






<a name="talos.resource.definitions.runtime.MountStatusSpec"></a>

### MountStatusSpec
MountStatusSpec describes status of the defined sysctls.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| source | [string](#string) |  |  |
| target | [string](#string) |  |  |
| filesystem_type | [string](#string) |  |  |
| options | [string](#string) | repeated |  |
| encrypted | [bool](#bool) |  |  |
| encryption_providers | [string](#string) | repeated |  |






<a name="talos.resource.definitions.runtime.PlatformMetadataSpec"></a>

### PlatformMetadataSpec
PlatformMetadataSpec describes platform metadata properties.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| platform | [string](#string) |  |  |
| hostname | [string](#string) |  |  |
| region | [string](#string) |  |  |
| zone | [string](#string) |  |  |
| instance_type | [string](#string) |  |  |
| instance_id | [string](#string) |  |  |
| provider_id | [string](#string) |  |  |
| spot | [bool](#bool) |  |  |
| internal_dns | [string](#string) |  |  |
| external_dns | [string](#string) |  |  |
| tags | [PlatformMetadataSpec.TagsEntry](#talos.resource.definitions.runtime.PlatformMetadataSpec.TagsEntry) | repeated |  |






<a name="talos.resource.definitions.runtime.PlatformMetadataSpec.TagsEntry"></a>

### PlatformMetadataSpec.TagsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="talos.resource.definitions.runtime.SecurityStateSpec"></a>

### SecurityStateSpec
SecurityStateSpec describes the security state resource properties.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| secure_boot | [bool](#bool) |  |  |
| uki_signing_key_fingerprint | [string](#string) |  |  |
| pcr_signing_key_fingerprint | [string](#string) |  |  |
| se_linux_state | [talos.resource.definitions.enums.RuntimeSELinuxState](#talos.resource.definitions.enums.RuntimeSELinuxState) |  |  |
| booted_with_uki | [bool](#bool) |  |  |






<a name="talos.resource.definitions.runtime.UniqueMachineTokenSpec"></a>

### UniqueMachineTokenSpec
UniqueMachineTokenSpec is the spec for the machine unique token. Token can be empty if machine wasn't assigned any.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| token | [string](#string) |  |  |






<a name="talos.resource.definitions.runtime.UnmetCondition"></a>

### UnmetCondition
UnmetCondition is a failure which prevents machine from being ready at the stage.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| reason | [string](#string) |  |  |






<a name="talos.resource.definitions.runtime.WatchdogTimerConfigSpec"></a>

### WatchdogTimerConfigSpec
WatchdogTimerConfigSpec describes configuration of watchdog timer.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| device | [string](#string) |  |  |
| timeout | [google.protobuf.Duration](#google.protobuf.Duration) |  |  |






<a name="talos.resource.definitions.runtime.WatchdogTimerStatusSpec"></a>

### WatchdogTimerStatusSpec
WatchdogTimerStatusSpec describes configuration of watchdog timer.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| device | [string](#string) |  |  |
| timeout | [google.protobuf.Duration](#google.protobuf.Duration) |  |  |
| feed_interval | [google.protobuf.Duration](#google.protobuf.Duration) |  |  |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="resource/definitions/secrets/secrets.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## resource/definitions/secrets/secrets.proto



<a name="talos.resource.definitions.secrets.APICertsSpec"></a>

### APICertsSpec
APICertsSpec describes etcd certs secrets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| client | [common.PEMEncodedCertificateAndKey](#common.PEMEncodedCertificateAndKey) |  |  |
| server | [common.PEMEncodedCertificateAndKey](#common.PEMEncodedCertificateAndKey) |  |  |
| accepted_c_as | [common.PEMEncodedCertificate](#common.PEMEncodedCertificate) | repeated |  |






<a name="talos.resource.definitions.secrets.CertSANSpec"></a>

### CertSANSpec
CertSANSpec describes fields of the cert SANs.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| i_ps | [common.NetIP](#common.NetIP) | repeated |  |
| dns_names | [string](#string) | repeated |  |
| fqdn | [string](#string) |  |  |






<a name="talos.resource.definitions.secrets.EtcdCertsSpec"></a>

### EtcdCertsSpec
EtcdCertsSpec describes etcd certs secrets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| etcd | [common.PEMEncodedCertificateAndKey](#common.PEMEncodedCertificateAndKey) |  |  |
| etcd_peer | [common.PEMEncodedCertificateAndKey](#common.PEMEncodedCertificateAndKey) |  |  |
| etcd_admin | [common.PEMEncodedCertificateAndKey](#common.PEMEncodedCertificateAndKey) |  |  |
| etcd_api_server | [common.PEMEncodedCertificateAndKey](#common.PEMEncodedCertificateAndKey) |  |  |






<a name="talos.resource.definitions.secrets.EtcdRootSpec"></a>

### EtcdRootSpec
EtcdRootSpec describes etcd CA secrets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| etcd_ca | [common.PEMEncodedCertificateAndKey](#common.PEMEncodedCertificateAndKey) |  |  |






<a name="talos.resource.definitions.secrets.KubeletSpec"></a>

### KubeletSpec
KubeletSpec describes root Kubernetes secrets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| endpoint | [common.URL](#common.URL) |  |  |
| bootstrap_token_id | [string](#string) |  |  |
| bootstrap_token_secret | [string](#string) |  |  |
| accepted_c_as | [common.PEMEncodedCertificate](#common.PEMEncodedCertificate) | repeated |  |






<a name="talos.resource.definitions.secrets.KubernetesCertsSpec"></a>

### KubernetesCertsSpec
KubernetesCertsSpec describes generated Kubernetes certificates.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| scheduler_kubeconfig | [string](#string) |  |  |
| controller_manager_kubeconfig | [string](#string) |  |  |
| localhost_admin_kubeconfig | [string](#string) |  |  |
| admin_kubeconfig | [string](#string) |  |  |






<a name="talos.resource.definitions.secrets.KubernetesDynamicCertsSpec"></a>

### KubernetesDynamicCertsSpec
KubernetesDynamicCertsSpec describes generated KubernetesCerts certificates.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| api_server | [common.PEMEncodedCertificateAndKey](#common.PEMEncodedCertificateAndKey) |  |  |
| api_server_kubelet_client | [common.PEMEncodedCertificateAndKey](#common.PEMEncodedCertificateAndKey) |  |  |
| front_proxy | [common.PEMEncodedCertificateAndKey](#common.PEMEncodedCertificateAndKey) |  |  |






<a name="talos.resource.definitions.secrets.KubernetesRootSpec"></a>

### KubernetesRootSpec
KubernetesRootSpec describes root Kubernetes secrets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| endpoint | [common.URL](#common.URL) |  |  |
| local_endpoint | [common.URL](#common.URL) |  |  |
| cert_sa_ns | [string](#string) | repeated |  |
| dns_domain | [string](#string) |  |  |
| issuing_ca | [common.PEMEncodedCertificateAndKey](#common.PEMEncodedCertificateAndKey) |  |  |
| service_account | [common.PEMEncodedKey](#common.PEMEncodedKey) |  |  |
| aggregator_ca | [common.PEMEncodedCertificateAndKey](#common.PEMEncodedCertificateAndKey) |  |  |
| aescbc_encryption_secret | [string](#string) |  |  |
| bootstrap_token_id | [string](#string) |  |  |
| bootstrap_token_secret | [string](#string) |  |  |
| secretbox_encryption_secret | [string](#string) |  |  |
| api_server_ips | [common.NetIP](#common.NetIP) | repeated |  |
| accepted_c_as | [common.PEMEncodedCertificate](#common.PEMEncodedCertificate) | repeated |  |






<a name="talos.resource.definitions.secrets.MaintenanceRootSpec"></a>

### MaintenanceRootSpec
MaintenanceRootSpec describes maintenance service CA.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ca | [common.PEMEncodedCertificateAndKey](#common.PEMEncodedCertificateAndKey) |  |  |






<a name="talos.resource.definitions.secrets.MaintenanceServiceCertsSpec"></a>

### MaintenanceServiceCertsSpec
MaintenanceServiceCertsSpec describes maintenance service certs secrets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ca | [common.PEMEncodedCertificateAndKey](#common.PEMEncodedCertificateAndKey) |  |  |
| server | [common.PEMEncodedCertificateAndKey](#common.PEMEncodedCertificateAndKey) |  |  |






<a name="talos.resource.definitions.secrets.OSRootSpec"></a>

### OSRootSpec
OSRootSpec describes operating system CA.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| issuing_ca | [common.PEMEncodedCertificateAndKey](#common.PEMEncodedCertificateAndKey) |  |  |
| cert_sani_ps | [common.NetIP](#common.NetIP) | repeated |  |
| cert_sandns_names | [string](#string) | repeated |  |
| token | [string](#string) |  |  |
| accepted_c_as | [common.PEMEncodedCertificate](#common.PEMEncodedCertificate) | repeated |  |






<a name="talos.resource.definitions.secrets.TrustdCertsSpec"></a>

### TrustdCertsSpec
TrustdCertsSpec describes etcd certs secrets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| server | [common.PEMEncodedCertificateAndKey](#common.PEMEncodedCertificateAndKey) |  |  |
| accepted_c_as | [common.PEMEncodedCertificate](#common.PEMEncodedCertificate) | repeated |  |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="resource/definitions/siderolink/siderolink.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## resource/definitions/siderolink/siderolink.proto



<a name="talos.resource.definitions.siderolink.ConfigSpec"></a>

### ConfigSpec
ConfigSpec describes Siderolink configuration.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| api_endpoint | [string](#string) |  |  |
| host | [string](#string) |  |  |
| join_token | [string](#string) |  |  |
| insecure | [bool](#bool) |  |  |
| tunnel | [bool](#bool) |  |  |






<a name="talos.resource.definitions.siderolink.StatusSpec"></a>

### StatusSpec
StatusSpec describes Siderolink status.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| host | [string](#string) |  |  |
| connected | [bool](#bool) |  |  |
| link_name | [string](#string) |  |  |
| grpc_tunnel | [bool](#bool) |  |  |






<a name="talos.resource.definitions.siderolink.TunnelSpec"></a>

### TunnelSpec
TunnelSpec describes Siderolink GRPC Tunnel configuration.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| api_endpoint | [string](#string) |  |  |
| link_name | [string](#string) |  |  |
| mtu | [int64](#int64) |  |  |
| node_address | [common.NetIPPort](#common.NetIPPort) |  |  |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="resource/definitions/time/time.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## resource/definitions/time/time.proto



<a name="talos.resource.definitions.time.AdjtimeStatusSpec"></a>

### AdjtimeStatusSpec
AdjtimeStatusSpec describes Linux internal adjtime state.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| offset | [google.protobuf.Duration](#google.protobuf.Duration) |  |  |
| frequency_adjustment_ratio | [double](#double) |  |  |
| max_error | [google.protobuf.Duration](#google.protobuf.Duration) |  |  |
| est_error | [google.protobuf.Duration](#google.protobuf.Duration) |  |  |
| status | [string](#string) |  |  |
| constant | [int64](#int64) |  |  |
| sync_status | [bool](#bool) |  |  |
| state | [string](#string) |  |  |






<a name="talos.resource.definitions.time.StatusSpec"></a>

### StatusSpec
StatusSpec describes time sync state.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| synced | [bool](#bool) |  |  |
| epoch | [int64](#int64) |  |  |
| sync_disabled | [bool](#bool) |  |  |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="resource/definitions/v1alpha1/v1alpha1.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## resource/definitions/v1alpha1/v1alpha1.proto



<a name="talos.resource.definitions.v1alpha1.ServiceSpec"></a>

### ServiceSpec
ServiceSpec describe service state.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| running | [bool](#bool) |  |  |
| healthy | [bool](#bool) |  |  |
| unknown | [bool](#bool) |  |  |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="inspect/inspect.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## inspect/inspect.proto



<a name="inspect.ControllerDependencyEdge"></a>

### ControllerDependencyEdge



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| controller_name | [string](#string) |  |  |
| edge_type | [DependencyEdgeType](#inspect.DependencyEdgeType) |  |  |
| resource_namespace | [string](#string) |  |  |
| resource_type | [string](#string) |  |  |
| resource_id | [string](#string) |  |  |






<a name="inspect.ControllerRuntimeDependenciesResponse"></a>

### ControllerRuntimeDependenciesResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [ControllerRuntimeDependency](#inspect.ControllerRuntimeDependency) | repeated |  |






<a name="inspect.ControllerRuntimeDependency"></a>

### ControllerRuntimeDependency
The ControllerRuntimeDependency message contains the graph of controller-resource dependencies.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| edges | [ControllerDependencyEdge](#inspect.ControllerDependencyEdge) | repeated |  |





 <!-- end messages -->


<a name="inspect.DependencyEdgeType"></a>

### DependencyEdgeType


| Name | Number | Description |
| ---- | ------ | ----------- |
| OUTPUT_EXCLUSIVE | 0 |  |
| OUTPUT_SHARED | 3 |  |
| INPUT_STRONG | 1 |  |
| INPUT_WEAK | 2 |  |
| INPUT_DESTROY_READY | 4 |  |


 <!-- end enums -->

 <!-- end HasExtensions -->


<a name="inspect.InspectService"></a>

### InspectService
The inspect service definition.

InspectService provides auxiliary API to inspect OS internals.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| ControllerRuntimeDependencies | [.google.protobuf.Empty](#google.protobuf.Empty) | [ControllerRuntimeDependenciesResponse](#inspect.ControllerRuntimeDependenciesResponse) |  |

 <!-- end services -->



<a name="machine/machine.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## machine/machine.proto



<a name="machine.AddressEvent"></a>

### AddressEvent
AddressEvent reports node endpoints aggregated from k8s.Endpoints and network.Hostname.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| hostname | [string](#string) |  |  |
| addresses | [string](#string) | repeated |  |






<a name="machine.ApplyConfiguration"></a>

### ApplyConfiguration
ApplyConfigurationResponse describes the response to a configuration request.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| warnings | [string](#string) | repeated | Configuration validation warnings. |
| mode | [ApplyConfigurationRequest.Mode](#machine.ApplyConfigurationRequest.Mode) |  | States which mode was actually chosen. |
| mode_details | [string](#string) |  | Human-readable message explaining the result of the apply configuration call. |






<a name="machine.ApplyConfigurationRequest"></a>

### ApplyConfigurationRequest
rpc applyConfiguration
ApplyConfiguration describes a request to assert a new configuration upon a
node.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| data | [bytes](#bytes) |  |  |
| mode | [ApplyConfigurationRequest.Mode](#machine.ApplyConfigurationRequest.Mode) |  |  |
| dry_run | [bool](#bool) |  |  |
| try_mode_timeout | [google.protobuf.Duration](#google.protobuf.Duration) |  |  |






<a name="machine.ApplyConfigurationResponse"></a>

### ApplyConfigurationResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [ApplyConfiguration](#machine.ApplyConfiguration) | repeated |  |






<a name="machine.BPFInstruction"></a>

### BPFInstruction



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| op | [uint32](#uint32) |  |  |
| jt | [uint32](#uint32) |  |  |
| jf | [uint32](#uint32) |  |  |
| k | [uint32](#uint32) |  |  |






<a name="machine.Bootstrap"></a>

### Bootstrap
The bootstrap message containing the bootstrap status.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |






<a name="machine.BootstrapRequest"></a>

### BootstrapRequest
rpc Bootstrap


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| recover_etcd | [bool](#bool) |  | Enable etcd recovery from the snapshot. Snapshot should be uploaded before this call via EtcdRecover RPC. |
| recover_skip_hash_check | [bool](#bool) |  | Skip hash check on the snapshot (etcd). Enable this when recovering from data directory copy to skip integrity check. |






<a name="machine.BootstrapResponse"></a>

### BootstrapResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Bootstrap](#machine.Bootstrap) | repeated |  |






<a name="machine.CNIConfig"></a>

### CNIConfig



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| urls | [string](#string) | repeated |  |






<a name="machine.CPUFreqStats"></a>

### CPUFreqStats



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| current_frequency | [uint64](#uint64) |  |  |
| minimum_frequency | [uint64](#uint64) |  |  |
| maximum_frequency | [uint64](#uint64) |  |  |
| governor | [string](#string) |  |  |






<a name="machine.CPUFreqStatsResponse"></a>

### CPUFreqStatsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [CPUsFreqStats](#machine.CPUsFreqStats) | repeated |  |






<a name="machine.CPUInfo"></a>

### CPUInfo



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| processor | [uint32](#uint32) |  |  |
| vendor_id | [string](#string) |  |  |
| cpu_family | [string](#string) |  |  |
| model | [string](#string) |  |  |
| model_name | [string](#string) |  |  |
| stepping | [string](#string) |  |  |
| microcode | [string](#string) |  |  |
| cpu_mhz | [double](#double) |  |  |
| cache_size | [string](#string) |  |  |
| physical_id | [string](#string) |  |  |
| siblings | [uint32](#uint32) |  |  |
| core_id | [string](#string) |  |  |
| cpu_cores | [uint32](#uint32) |  |  |
| apic_id | [string](#string) |  |  |
| initial_apic_id | [string](#string) |  |  |
| fpu | [string](#string) |  |  |
| fpu_exception | [string](#string) |  |  |
| cpu_id_level | [uint32](#uint32) |  |  |
| wp | [string](#string) |  |  |
| flags | [string](#string) | repeated |  |
| bugs | [string](#string) | repeated |  |
| bogo_mips | [double](#double) |  |  |
| cl_flush_size | [uint32](#uint32) |  |  |
| cache_alignment | [uint32](#uint32) |  |  |
| address_sizes | [string](#string) |  |  |
| power_management | [string](#string) |  |  |






<a name="machine.CPUInfoResponse"></a>

### CPUInfoResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [CPUsInfo](#machine.CPUsInfo) | repeated |  |






<a name="machine.CPUStat"></a>

### CPUStat



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| user | [double](#double) |  |  |
| nice | [double](#double) |  |  |
| system | [double](#double) |  |  |
| idle | [double](#double) |  |  |
| iowait | [double](#double) |  |  |
| irq | [double](#double) |  |  |
| soft_irq | [double](#double) |  |  |
| steal | [double](#double) |  |  |
| guest | [double](#double) |  |  |
| guest_nice | [double](#double) |  |  |






<a name="machine.CPUsFreqStats"></a>

### CPUsFreqStats



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| cpu_freq_stats | [CPUFreqStats](#machine.CPUFreqStats) | repeated |  |






<a name="machine.CPUsInfo"></a>

### CPUsInfo



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| cpu_info | [CPUInfo](#machine.CPUInfo) | repeated |  |






<a name="machine.ClusterConfig"></a>

### ClusterConfig



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| control_plane | [ControlPlaneConfig](#machine.ControlPlaneConfig) |  |  |
| cluster_network | [ClusterNetworkConfig](#machine.ClusterNetworkConfig) |  |  |
| allow_scheduling_on_control_planes | [bool](#bool) |  |  |






<a name="machine.ClusterNetworkConfig"></a>

### ClusterNetworkConfig



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| dns_domain | [string](#string) |  |  |
| cni_config | [CNIConfig](#machine.CNIConfig) |  |  |






<a name="machine.ConfigLoadErrorEvent"></a>

### ConfigLoadErrorEvent
ConfigLoadErrorEvent is reported when the config loading has failed.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| error | [string](#string) |  |  |






<a name="machine.ConfigValidationErrorEvent"></a>

### ConfigValidationErrorEvent
ConfigValidationErrorEvent is reported when config validation has failed.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| error | [string](#string) |  |  |






<a name="machine.ConnectRecord"></a>

### ConnectRecord



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| l4proto | [string](#string) |  |  |
| localip | [string](#string) |  |  |
| localport | [uint32](#uint32) |  |  |
| remoteip | [string](#string) |  |  |
| remoteport | [uint32](#uint32) |  |  |
| state | [ConnectRecord.State](#machine.ConnectRecord.State) |  |  |
| txqueue | [uint64](#uint64) |  |  |
| rxqueue | [uint64](#uint64) |  |  |
| tr | [ConnectRecord.TimerActive](#machine.ConnectRecord.TimerActive) |  |  |
| timerwhen | [uint64](#uint64) |  |  |
| retrnsmt | [uint64](#uint64) |  |  |
| uid | [uint32](#uint32) |  |  |
| timeout | [uint64](#uint64) |  |  |
| inode | [uint64](#uint64) |  |  |
| ref | [uint64](#uint64) |  |  |
| pointer | [uint64](#uint64) |  |  |
| process | [ConnectRecord.Process](#machine.ConnectRecord.Process) |  |  |
| netns | [string](#string) |  |  |






<a name="machine.ConnectRecord.Process"></a>

### ConnectRecord.Process



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| pid | [uint32](#uint32) |  |  |
| name | [string](#string) |  |  |






<a name="machine.Container"></a>

### Container
The messages message containing the requested containers.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| containers | [ContainerInfo](#machine.ContainerInfo) | repeated |  |






<a name="machine.ContainerInfo"></a>

### ContainerInfo
The messages message containing the requested containers.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| namespace | [string](#string) |  |  |
| id | [string](#string) |  |  |
| uid | [string](#string) |  |  |
| internal_id | [string](#string) |  |  |
| image | [string](#string) |  |  |
| pid | [uint32](#uint32) |  |  |
| status | [string](#string) |  |  |
| pod_id | [string](#string) |  |  |
| name | [string](#string) |  |  |
| network_namespace | [string](#string) |  |  |






<a name="machine.ContainersRequest"></a>

### ContainersRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| namespace | [string](#string) |  |  |
| driver | [common.ContainerDriver](#common.ContainerDriver) |  | driver might be default "containerd" or "cri" |






<a name="machine.ContainersResponse"></a>

### ContainersResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Container](#machine.Container) | repeated |  |






<a name="machine.ControlPlaneConfig"></a>

### ControlPlaneConfig



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| endpoint | [string](#string) |  |  |






<a name="machine.CopyRequest"></a>

### CopyRequest
CopyRequest describes a request to copy data out of Talos node

Copy produces .tar.gz archive which is streamed back to the caller


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| root_path | [string](#string) |  | Root path to start copying data out, it might be either a file or directory |






<a name="machine.DHCPOptionsConfig"></a>

### DHCPOptionsConfig



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| route_metric | [uint32](#uint32) |  |  |






<a name="machine.DiskStat"></a>

### DiskStat



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| read_completed | [uint64](#uint64) |  |  |
| read_merged | [uint64](#uint64) |  |  |
| read_sectors | [uint64](#uint64) |  |  |
| read_time_ms | [uint64](#uint64) |  |  |
| write_completed | [uint64](#uint64) |  |  |
| write_merged | [uint64](#uint64) |  |  |
| write_sectors | [uint64](#uint64) |  |  |
| write_time_ms | [uint64](#uint64) |  |  |
| io_in_progress | [uint64](#uint64) |  |  |
| io_time_ms | [uint64](#uint64) |  |  |
| io_time_weighted_ms | [uint64](#uint64) |  |  |
| discard_completed | [uint64](#uint64) |  |  |
| discard_merged | [uint64](#uint64) |  |  |
| discard_sectors | [uint64](#uint64) |  |  |
| discard_time_ms | [uint64](#uint64) |  |  |






<a name="machine.DiskStats"></a>

### DiskStats



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| total | [DiskStat](#machine.DiskStat) |  |  |
| devices | [DiskStat](#machine.DiskStat) | repeated |  |






<a name="machine.DiskStatsResponse"></a>

### DiskStatsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [DiskStats](#machine.DiskStats) | repeated |  |






<a name="machine.DiskUsageInfo"></a>

### DiskUsageInfo
DiskUsageInfo describes a file or directory's information for du command


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| name | [string](#string) |  | Name is the name (including prefixed path) of the file or directory |
| size | [int64](#int64) |  | Size indicates the number of bytes contained within the file |
| error | [string](#string) |  | Error describes any error encountered while trying to read the file information. |
| relative_name | [string](#string) |  | RelativeName is the name of the file or directory relative to the RootPath |






<a name="machine.DiskUsageRequest"></a>

### DiskUsageRequest
DiskUsageRequest describes a request to list disk usage of directories and regular files


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| recursion_depth | [int32](#int32) |  | RecursionDepth indicates how many levels of subdirectories should be recursed. The default (0) indicates that no limit should be enforced. |
| all | [bool](#bool) |  | All write sizes for all files, not just directories. |
| threshold | [int64](#int64) |  | Threshold exclude entries smaller than SIZE if positive, or entries greater than SIZE if negative. |
| paths | [string](#string) | repeated | DiskUsagePaths is the list of directories to calculate disk usage for. |






<a name="machine.DmesgRequest"></a>

### DmesgRequest
dmesg


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| follow | [bool](#bool) |  |  |
| tail | [bool](#bool) |  |  |






<a name="machine.EtcdAlarm"></a>

### EtcdAlarm



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| member_alarms | [EtcdMemberAlarm](#machine.EtcdMemberAlarm) | repeated |  |






<a name="machine.EtcdAlarmDisarm"></a>

### EtcdAlarmDisarm



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| member_alarms | [EtcdMemberAlarm](#machine.EtcdMemberAlarm) | repeated |  |






<a name="machine.EtcdAlarmDisarmResponse"></a>

### EtcdAlarmDisarmResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [EtcdAlarmDisarm](#machine.EtcdAlarmDisarm) | repeated |  |






<a name="machine.EtcdAlarmListResponse"></a>

### EtcdAlarmListResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [EtcdAlarm](#machine.EtcdAlarm) | repeated |  |






<a name="machine.EtcdDefragment"></a>

### EtcdDefragment



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |






<a name="machine.EtcdDefragmentResponse"></a>

### EtcdDefragmentResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [EtcdDefragment](#machine.EtcdDefragment) | repeated |  |






<a name="machine.EtcdForfeitLeadership"></a>

### EtcdForfeitLeadership



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| member | [string](#string) |  |  |






<a name="machine.EtcdForfeitLeadershipRequest"></a>

### EtcdForfeitLeadershipRequest







<a name="machine.EtcdForfeitLeadershipResponse"></a>

### EtcdForfeitLeadershipResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [EtcdForfeitLeadership](#machine.EtcdForfeitLeadership) | repeated |  |






<a name="machine.EtcdLeaveCluster"></a>

### EtcdLeaveCluster



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |






<a name="machine.EtcdLeaveClusterRequest"></a>

### EtcdLeaveClusterRequest







<a name="machine.EtcdLeaveClusterResponse"></a>

### EtcdLeaveClusterResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [EtcdLeaveCluster](#machine.EtcdLeaveCluster) | repeated |  |






<a name="machine.EtcdMember"></a>

### EtcdMember
EtcdMember describes a single etcd member.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [uint64](#uint64) |  | member ID. |
| hostname | [string](#string) |  | human-readable name of the member. |
| peer_urls | [string](#string) | repeated | the list of URLs the member exposes to clients for communication. |
| client_urls | [string](#string) | repeated | the list of URLs the member exposes to the cluster for communication. |
| is_learner | [bool](#bool) |  | learner flag |






<a name="machine.EtcdMemberAlarm"></a>

### EtcdMemberAlarm



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| member_id | [uint64](#uint64) |  |  |
| alarm | [EtcdMemberAlarm.AlarmType](#machine.EtcdMemberAlarm.AlarmType) |  |  |






<a name="machine.EtcdMemberListRequest"></a>

### EtcdMemberListRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| query_local | [bool](#bool) |  |  |






<a name="machine.EtcdMemberListResponse"></a>

### EtcdMemberListResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [EtcdMembers](#machine.EtcdMembers) | repeated |  |






<a name="machine.EtcdMemberStatus"></a>

### EtcdMemberStatus



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| member_id | [uint64](#uint64) |  |  |
| protocol_version | [string](#string) |  |  |
| db_size | [int64](#int64) |  |  |
| db_size_in_use | [int64](#int64) |  |  |
| leader | [uint64](#uint64) |  |  |
| raft_index | [uint64](#uint64) |  |  |
| raft_term | [uint64](#uint64) |  |  |
| raft_applied_index | [uint64](#uint64) |  |  |
| errors | [string](#string) | repeated |  |
| is_learner | [bool](#bool) |  |  |






<a name="machine.EtcdMembers"></a>

### EtcdMembers
EtcdMembers contains the list of members registered on the host.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| legacy_members | [string](#string) | repeated | list of member hostnames. |
| members | [EtcdMember](#machine.EtcdMember) | repeated | the list of etcd members registered on the node. |






<a name="machine.EtcdRecover"></a>

### EtcdRecover



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |






<a name="machine.EtcdRecoverResponse"></a>

### EtcdRecoverResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [EtcdRecover](#machine.EtcdRecover) | repeated |  |






<a name="machine.EtcdRemoveMember"></a>

### EtcdRemoveMember



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |






<a name="machine.EtcdRemoveMemberByID"></a>

### EtcdRemoveMemberByID



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |






<a name="machine.EtcdRemoveMemberByIDRequest"></a>

### EtcdRemoveMemberByIDRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| member_id | [uint64](#uint64) |  |  |






<a name="machine.EtcdRemoveMemberByIDResponse"></a>

### EtcdRemoveMemberByIDResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [EtcdRemoveMemberByID](#machine.EtcdRemoveMemberByID) | repeated |  |






<a name="machine.EtcdRemoveMemberRequest"></a>

### EtcdRemoveMemberRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| member | [string](#string) |  |  |






<a name="machine.EtcdRemoveMemberResponse"></a>

### EtcdRemoveMemberResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [EtcdRemoveMember](#machine.EtcdRemoveMember) | repeated |  |






<a name="machine.EtcdSnapshotRequest"></a>

### EtcdSnapshotRequest







<a name="machine.EtcdStatus"></a>

### EtcdStatus



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| member_status | [EtcdMemberStatus](#machine.EtcdMemberStatus) |  |  |






<a name="machine.EtcdStatusResponse"></a>

### EtcdStatusResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [EtcdStatus](#machine.EtcdStatus) | repeated |  |






<a name="machine.Event"></a>

### Event



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| data | [google.protobuf.Any](#google.protobuf.Any) |  |  |
| id | [string](#string) |  |  |
| actor_id | [string](#string) |  |  |






<a name="machine.EventsRequest"></a>

### EventsRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tail_events | [int32](#int32) |  |  |
| tail_id | [string](#string) |  |  |
| tail_seconds | [int32](#int32) |  |  |
| with_actor_id | [string](#string) |  |  |






<a name="machine.FeaturesInfo"></a>

### FeaturesInfo
FeaturesInfo describes individual Talos features that can be switched on or off.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| rbac | [bool](#bool) |  | RBAC is true if role-based access control is enabled. |






<a name="machine.FileInfo"></a>

### FileInfo
FileInfo describes a file or directory's information


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| name | [string](#string) |  | Name is the name (including prefixed path) of the file or directory |
| size | [int64](#int64) |  | Size indicates the number of bytes contained within the file |
| mode | [uint32](#uint32) |  | Mode is the bitmap of UNIX mode/permission flags of the file |
| modified | [int64](#int64) |  | Modified indicates the UNIX timestamp at which the file was last modified |
| is_dir | [bool](#bool) |  | IsDir indicates that the file is a directory |
| error | [string](#string) |  | Error describes any error encountered while trying to read the file information. |
| link | [string](#string) |  | Link is filled with symlink target |
| relative_name | [string](#string) |  | RelativeName is the name of the file or directory relative to the RootPath |
| uid | [uint32](#uint32) |  | Owner uid |
| gid | [uint32](#uint32) |  | Owner gid |
| xattrs | [Xattr](#machine.Xattr) | repeated | Extended attributes (if present and requested) |






<a name="machine.GenerateClientConfiguration"></a>

### GenerateClientConfiguration



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| ca | [bytes](#bytes) |  | PEM-encoded CA certificate. |
| crt | [bytes](#bytes) |  | PEM-encoded generated client certificate. |
| key | [bytes](#bytes) |  | PEM-encoded generated client key. |
| talosconfig | [bytes](#bytes) |  | Client configuration (talosconfig) file content. |






<a name="machine.GenerateClientConfigurationRequest"></a>

### GenerateClientConfigurationRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| roles | [string](#string) | repeated | Roles in the generated client certificate. |
| crt_ttl | [google.protobuf.Duration](#google.protobuf.Duration) |  | Client certificate TTL. |






<a name="machine.GenerateClientConfigurationResponse"></a>

### GenerateClientConfigurationResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [GenerateClientConfiguration](#machine.GenerateClientConfiguration) | repeated |  |






<a name="machine.GenerateConfiguration"></a>

### GenerateConfiguration
GenerateConfiguration describes the response to a generate configuration request.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| data | [bytes](#bytes) | repeated |  |
| talosconfig | [bytes](#bytes) |  |  |






<a name="machine.GenerateConfigurationRequest"></a>

### GenerateConfigurationRequest
GenerateConfigurationRequest describes a request to generate a new configuration
on a node.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| config_version | [string](#string) |  |  |
| cluster_config | [ClusterConfig](#machine.ClusterConfig) |  |  |
| machine_config | [MachineConfig](#machine.MachineConfig) |  |  |
| override_time | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  |  |






<a name="machine.GenerateConfigurationResponse"></a>

### GenerateConfigurationResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [GenerateConfiguration](#machine.GenerateConfiguration) | repeated |  |






<a name="machine.Hostname"></a>

### Hostname



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| hostname | [string](#string) |  |  |






<a name="machine.HostnameResponse"></a>

### HostnameResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Hostname](#machine.Hostname) | repeated |  |






<a name="machine.ImageListRequest"></a>

### ImageListRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| namespace | [common.ContainerdNamespace](#common.ContainerdNamespace) |  | Containerd namespace to use. |






<a name="machine.ImageListResponse"></a>

### ImageListResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| name | [string](#string) |  |  |
| digest | [string](#string) |  |  |
| size | [int64](#int64) |  |  |
| created_at | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  |  |






<a name="machine.ImagePull"></a>

### ImagePull



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |






<a name="machine.ImagePullRequest"></a>

### ImagePullRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| namespace | [common.ContainerdNamespace](#common.ContainerdNamespace) |  | Containerd namespace to use. |
| reference | [string](#string) |  | Image reference to pull. |






<a name="machine.ImagePullResponse"></a>

### ImagePullResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [ImagePull](#machine.ImagePull) | repeated |  |






<a name="machine.InstallConfig"></a>

### InstallConfig



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| install_disk | [string](#string) |  |  |
| install_image | [string](#string) |  |  |






<a name="machine.ListRequest"></a>

### ListRequest
ListRequest describes a request to list the contents of a directory.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| root | [string](#string) |  | Root indicates the root directory for the list. If not indicated, '/' is presumed. |
| recurse | [bool](#bool) |  | Recurse indicates that subdirectories should be recursed. |
| recursion_depth | [int32](#int32) |  | RecursionDepth indicates how many levels of subdirectories should be recursed. The default (0) indicates that no limit should be enforced. |
| types | [ListRequest.Type](#machine.ListRequest.Type) | repeated | Types indicates what file type should be returned. If not indicated, all files will be returned. |
| report_xattrs | [bool](#bool) |  | Report xattrs |






<a name="machine.LoadAvg"></a>

### LoadAvg



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| load1 | [double](#double) |  |  |
| load5 | [double](#double) |  |  |
| load15 | [double](#double) |  |  |






<a name="machine.LoadAvgResponse"></a>

### LoadAvgResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [LoadAvg](#machine.LoadAvg) | repeated |  |






<a name="machine.LogsContainer"></a>

### LogsContainer
LogsContainer desribes all avalaible registered log containers.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| ids | [string](#string) | repeated |  |






<a name="machine.LogsContainersResponse"></a>

### LogsContainersResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [LogsContainer](#machine.LogsContainer) | repeated |  |






<a name="machine.LogsRequest"></a>

### LogsRequest
rpc logs
The request message containing the process name.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| namespace | [string](#string) |  |  |
| id | [string](#string) |  |  |
| driver | [common.ContainerDriver](#common.ContainerDriver) |  | driver might be default "containerd" or "cri" |
| follow | [bool](#bool) |  |  |
| tail_lines | [int32](#int32) |  |  |






<a name="machine.MachineConfig"></a>

### MachineConfig



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| type | [MachineConfig.MachineType](#machine.MachineConfig.MachineType) |  |  |
| install_config | [InstallConfig](#machine.InstallConfig) |  |  |
| network_config | [NetworkConfig](#machine.NetworkConfig) |  |  |
| kubernetes_version | [string](#string) |  |  |






<a name="machine.MachineStatusEvent"></a>

### MachineStatusEvent
MachineStatusEvent reports changes to the MachineStatus resource.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| stage | [MachineStatusEvent.MachineStage](#machine.MachineStatusEvent.MachineStage) |  |  |
| status | [MachineStatusEvent.MachineStatus](#machine.MachineStatusEvent.MachineStatus) |  |  |






<a name="machine.MachineStatusEvent.MachineStatus"></a>

### MachineStatusEvent.MachineStatus



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ready | [bool](#bool) |  |  |
| unmet_conditions | [MachineStatusEvent.MachineStatus.UnmetCondition](#machine.MachineStatusEvent.MachineStatus.UnmetCondition) | repeated |  |






<a name="machine.MachineStatusEvent.MachineStatus.UnmetCondition"></a>

### MachineStatusEvent.MachineStatus.UnmetCondition



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| reason | [string](#string) |  |  |






<a name="machine.MemInfo"></a>

### MemInfo



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| memtotal | [uint64](#uint64) |  |  |
| memfree | [uint64](#uint64) |  |  |
| memavailable | [uint64](#uint64) |  |  |
| buffers | [uint64](#uint64) |  |  |
| cached | [uint64](#uint64) |  |  |
| swapcached | [uint64](#uint64) |  |  |
| active | [uint64](#uint64) |  |  |
| inactive | [uint64](#uint64) |  |  |
| activeanon | [uint64](#uint64) |  |  |
| inactiveanon | [uint64](#uint64) |  |  |
| activefile | [uint64](#uint64) |  |  |
| inactivefile | [uint64](#uint64) |  |  |
| unevictable | [uint64](#uint64) |  |  |
| mlocked | [uint64](#uint64) |  |  |
| swaptotal | [uint64](#uint64) |  |  |
| swapfree | [uint64](#uint64) |  |  |
| dirty | [uint64](#uint64) |  |  |
| writeback | [uint64](#uint64) |  |  |
| anonpages | [uint64](#uint64) |  |  |
| mapped | [uint64](#uint64) |  |  |
| shmem | [uint64](#uint64) |  |  |
| slab | [uint64](#uint64) |  |  |
| sreclaimable | [uint64](#uint64) |  |  |
| sunreclaim | [uint64](#uint64) |  |  |
| kernelstack | [uint64](#uint64) |  |  |
| pagetables | [uint64](#uint64) |  |  |
| nfsunstable | [uint64](#uint64) |  |  |
| bounce | [uint64](#uint64) |  |  |
| writebacktmp | [uint64](#uint64) |  |  |
| commitlimit | [uint64](#uint64) |  |  |
| committedas | [uint64](#uint64) |  |  |
| vmalloctotal | [uint64](#uint64) |  |  |
| vmallocused | [uint64](#uint64) |  |  |
| vmallocchunk | [uint64](#uint64) |  |  |
| hardwarecorrupted | [uint64](#uint64) |  |  |
| anonhugepages | [uint64](#uint64) |  |  |
| shmemhugepages | [uint64](#uint64) |  |  |
| shmempmdmapped | [uint64](#uint64) |  |  |
| cmatotal | [uint64](#uint64) |  |  |
| cmafree | [uint64](#uint64) |  |  |
| hugepagestotal | [uint64](#uint64) |  |  |
| hugepagesfree | [uint64](#uint64) |  |  |
| hugepagesrsvd | [uint64](#uint64) |  |  |
| hugepagessurp | [uint64](#uint64) |  |  |
| hugepagesize | [uint64](#uint64) |  |  |
| directmap4k | [uint64](#uint64) |  |  |
| directmap2m | [uint64](#uint64) |  |  |
| directmap1g | [uint64](#uint64) |  |  |






<a name="machine.Memory"></a>

### Memory



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| meminfo | [MemInfo](#machine.MemInfo) |  |  |






<a name="machine.MemoryResponse"></a>

### MemoryResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Memory](#machine.Memory) | repeated |  |






<a name="machine.MetaDelete"></a>

### MetaDelete



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |






<a name="machine.MetaDeleteRequest"></a>

### MetaDeleteRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [uint32](#uint32) |  |  |






<a name="machine.MetaDeleteResponse"></a>

### MetaDeleteResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [MetaDelete](#machine.MetaDelete) | repeated |  |






<a name="machine.MetaWrite"></a>

### MetaWrite



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |






<a name="machine.MetaWriteRequest"></a>

### MetaWriteRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [uint32](#uint32) |  |  |
| value | [bytes](#bytes) |  |  |






<a name="machine.MetaWriteResponse"></a>

### MetaWriteResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [MetaWrite](#machine.MetaWrite) | repeated |  |






<a name="machine.MountStat"></a>

### MountStat
The messages message containing the requested processes.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| filesystem | [string](#string) |  |  |
| size | [uint64](#uint64) |  |  |
| available | [uint64](#uint64) |  |  |
| mounted_on | [string](#string) |  |  |






<a name="machine.Mounts"></a>

### Mounts
The messages message containing the requested df stats.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| stats | [MountStat](#machine.MountStat) | repeated |  |






<a name="machine.MountsResponse"></a>

### MountsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Mounts](#machine.Mounts) | repeated |  |






<a name="machine.NetDev"></a>

### NetDev



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| rx_bytes | [uint64](#uint64) |  |  |
| rx_packets | [uint64](#uint64) |  |  |
| rx_errors | [uint64](#uint64) |  |  |
| rx_dropped | [uint64](#uint64) |  |  |
| rx_fifo | [uint64](#uint64) |  |  |
| rx_frame | [uint64](#uint64) |  |  |
| rx_compressed | [uint64](#uint64) |  |  |
| rx_multicast | [uint64](#uint64) |  |  |
| tx_bytes | [uint64](#uint64) |  |  |
| tx_packets | [uint64](#uint64) |  |  |
| tx_errors | [uint64](#uint64) |  |  |
| tx_dropped | [uint64](#uint64) |  |  |
| tx_fifo | [uint64](#uint64) |  |  |
| tx_collisions | [uint64](#uint64) |  |  |
| tx_carrier | [uint64](#uint64) |  |  |
| tx_compressed | [uint64](#uint64) |  |  |






<a name="machine.Netstat"></a>

### Netstat



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| connectrecord | [ConnectRecord](#machine.ConnectRecord) | repeated |  |






<a name="machine.NetstatRequest"></a>

### NetstatRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| filter | [NetstatRequest.Filter](#machine.NetstatRequest.Filter) |  |  |
| feature | [NetstatRequest.Feature](#machine.NetstatRequest.Feature) |  |  |
| l4proto | [NetstatRequest.L4proto](#machine.NetstatRequest.L4proto) |  |  |
| netns | [NetstatRequest.NetNS](#machine.NetstatRequest.NetNS) |  |  |






<a name="machine.NetstatRequest.Feature"></a>

### NetstatRequest.Feature



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| pid | [bool](#bool) |  |  |






<a name="machine.NetstatRequest.L4proto"></a>

### NetstatRequest.L4proto



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tcp | [bool](#bool) |  |  |
| tcp6 | [bool](#bool) |  |  |
| udp | [bool](#bool) |  |  |
| udp6 | [bool](#bool) |  |  |
| udplite | [bool](#bool) |  |  |
| udplite6 | [bool](#bool) |  |  |
| raw | [bool](#bool) |  |  |
| raw6 | [bool](#bool) |  |  |






<a name="machine.NetstatRequest.NetNS"></a>

### NetstatRequest.NetNS



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| hostnetwork | [bool](#bool) |  |  |
| netns | [string](#string) | repeated |  |
| allnetns | [bool](#bool) |  |  |






<a name="machine.NetstatResponse"></a>

### NetstatResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Netstat](#machine.Netstat) | repeated |  |






<a name="machine.NetworkConfig"></a>

### NetworkConfig



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| hostname | [string](#string) |  |  |
| interfaces | [NetworkDeviceConfig](#machine.NetworkDeviceConfig) | repeated |  |






<a name="machine.NetworkDeviceConfig"></a>

### NetworkDeviceConfig



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| interface | [string](#string) |  |  |
| cidr | [string](#string) |  |  |
| mtu | [int32](#int32) |  |  |
| dhcp | [bool](#bool) |  |  |
| ignore | [bool](#bool) |  |  |
| dhcp_options | [DHCPOptionsConfig](#machine.DHCPOptionsConfig) |  |  |
| routes | [RouteConfig](#machine.RouteConfig) | repeated |  |






<a name="machine.NetworkDeviceStats"></a>

### NetworkDeviceStats



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| total | [NetDev](#machine.NetDev) |  |  |
| devices | [NetDev](#machine.NetDev) | repeated |  |






<a name="machine.NetworkDeviceStatsResponse"></a>

### NetworkDeviceStatsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [NetworkDeviceStats](#machine.NetworkDeviceStats) | repeated |  |






<a name="machine.PacketCaptureRequest"></a>

### PacketCaptureRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| interface | [string](#string) |  | Interface name to perform packet capture on. |
| promiscuous | [bool](#bool) |  | Enable promiscuous mode. |
| snap_len | [uint32](#uint32) |  | Snap length in bytes. |
| bpf_filter | [BPFInstruction](#machine.BPFInstruction) | repeated | BPF filter. |






<a name="machine.PhaseEvent"></a>

### PhaseEvent



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| phase | [string](#string) |  |  |
| action | [PhaseEvent.Action](#machine.PhaseEvent.Action) |  |  |






<a name="machine.PlatformInfo"></a>

### PlatformInfo



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| mode | [string](#string) |  |  |






<a name="machine.Process"></a>

### Process



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| processes | [ProcessInfo](#machine.ProcessInfo) | repeated |  |






<a name="machine.ProcessInfo"></a>

### ProcessInfo



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| pid | [int32](#int32) |  |  |
| ppid | [int32](#int32) |  |  |
| state | [string](#string) |  |  |
| threads | [int32](#int32) |  |  |
| cpu_time | [double](#double) |  |  |
| virtual_memory | [uint64](#uint64) |  |  |
| resident_memory | [uint64](#uint64) |  |  |
| command | [string](#string) |  |  |
| executable | [string](#string) |  |  |
| args | [string](#string) |  |  |
| label | [string](#string) |  |  |






<a name="machine.ProcessesResponse"></a>

### ProcessesResponse
rpc processes


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Process](#machine.Process) | repeated |  |






<a name="machine.ReadRequest"></a>

### ReadRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| path | [string](#string) |  |  |






<a name="machine.Reboot"></a>

### Reboot
The reboot message containing the reboot status.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| actor_id | [string](#string) |  |  |






<a name="machine.RebootRequest"></a>

### RebootRequest
rpc reboot


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| mode | [RebootRequest.Mode](#machine.RebootRequest.Mode) |  |  |






<a name="machine.RebootResponse"></a>

### RebootResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Reboot](#machine.Reboot) | repeated |  |






<a name="machine.Reset"></a>

### Reset
The reset message containing the restart status.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| actor_id | [string](#string) |  |  |






<a name="machine.ResetPartitionSpec"></a>

### ResetPartitionSpec
rpc reset


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| label | [string](#string) |  |  |
| wipe | [bool](#bool) |  |  |






<a name="machine.ResetRequest"></a>

### ResetRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| graceful | [bool](#bool) |  | Graceful indicates whether node should leave etcd before the upgrade, it also enforces etcd checks before leaving. |
| reboot | [bool](#bool) |  | Reboot indicates whether node should reboot or halt after resetting. |
| system_partitions_to_wipe | [ResetPartitionSpec](#machine.ResetPartitionSpec) | repeated | System_partitions_to_wipe lists specific system disk partitions to be reset (wiped). If system_partitions_to_wipe is empty, all the partitions are erased. |
| user_disks_to_wipe | [string](#string) | repeated | UserDisksToWipe lists specific connected block devices to be reset (wiped). |
| mode | [ResetRequest.WipeMode](#machine.ResetRequest.WipeMode) |  | WipeMode defines which devices should be wiped. |






<a name="machine.ResetResponse"></a>

### ResetResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Reset](#machine.Reset) | repeated |  |






<a name="machine.Restart"></a>

### Restart



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |






<a name="machine.RestartEvent"></a>

### RestartEvent



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| cmd | [int64](#int64) |  |  |






<a name="machine.RestartRequest"></a>

### RestartRequest
rpc restart
The request message containing the process to restart.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| namespace | [string](#string) |  |  |
| id | [string](#string) |  |  |
| driver | [common.ContainerDriver](#common.ContainerDriver) |  | driver might be default "containerd" or "cri" |






<a name="machine.RestartResponse"></a>

### RestartResponse
The messages message containing the restart status.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Restart](#machine.Restart) | repeated |  |






<a name="machine.Rollback"></a>

### Rollback



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |






<a name="machine.RollbackRequest"></a>

### RollbackRequest
rpc rollback






<a name="machine.RollbackResponse"></a>

### RollbackResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Rollback](#machine.Rollback) | repeated |  |






<a name="machine.RouteConfig"></a>

### RouteConfig



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| network | [string](#string) |  |  |
| gateway | [string](#string) |  |  |
| metric | [uint32](#uint32) |  |  |






<a name="machine.SequenceEvent"></a>

### SequenceEvent
rpc events


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| sequence | [string](#string) |  |  |
| action | [SequenceEvent.Action](#machine.SequenceEvent.Action) |  |  |
| error | [common.Error](#common.Error) |  |  |






<a name="machine.ServiceEvent"></a>

### ServiceEvent



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| msg | [string](#string) |  |  |
| state | [string](#string) |  |  |
| ts | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  |  |






<a name="machine.ServiceEvents"></a>

### ServiceEvents



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| events | [ServiceEvent](#machine.ServiceEvent) | repeated |  |






<a name="machine.ServiceHealth"></a>

### ServiceHealth



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| unknown | [bool](#bool) |  |  |
| healthy | [bool](#bool) |  |  |
| last_message | [string](#string) |  |  |
| last_change | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  |  |






<a name="machine.ServiceInfo"></a>

### ServiceInfo



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| state | [string](#string) |  |  |
| events | [ServiceEvents](#machine.ServiceEvents) |  |  |
| health | [ServiceHealth](#machine.ServiceHealth) |  |  |






<a name="machine.ServiceList"></a>

### ServiceList
rpc servicelist


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| services | [ServiceInfo](#machine.ServiceInfo) | repeated |  |






<a name="machine.ServiceListResponse"></a>

### ServiceListResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [ServiceList](#machine.ServiceList) | repeated |  |






<a name="machine.ServiceRestart"></a>

### ServiceRestart



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| resp | [string](#string) |  |  |






<a name="machine.ServiceRestartRequest"></a>

### ServiceRestartRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |






<a name="machine.ServiceRestartResponse"></a>

### ServiceRestartResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [ServiceRestart](#machine.ServiceRestart) | repeated |  |






<a name="machine.ServiceStart"></a>

### ServiceStart



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| resp | [string](#string) |  |  |






<a name="machine.ServiceStartRequest"></a>

### ServiceStartRequest
rpc servicestart


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |






<a name="machine.ServiceStartResponse"></a>

### ServiceStartResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [ServiceStart](#machine.ServiceStart) | repeated |  |






<a name="machine.ServiceStateEvent"></a>

### ServiceStateEvent



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| service | [string](#string) |  |  |
| action | [ServiceStateEvent.Action](#machine.ServiceStateEvent.Action) |  |  |
| message | [string](#string) |  |  |
| health | [ServiceHealth](#machine.ServiceHealth) |  |  |






<a name="machine.ServiceStop"></a>

### ServiceStop



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| resp | [string](#string) |  |  |






<a name="machine.ServiceStopRequest"></a>

### ServiceStopRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |






<a name="machine.ServiceStopResponse"></a>

### ServiceStopResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [ServiceStop](#machine.ServiceStop) | repeated |  |






<a name="machine.Shutdown"></a>

### Shutdown
rpc shutdown
The messages message containing the shutdown status.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| actor_id | [string](#string) |  |  |






<a name="machine.ShutdownRequest"></a>

### ShutdownRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| force | [bool](#bool) |  | Force indicates whether node should shutdown without first cordening and draining |






<a name="machine.ShutdownResponse"></a>

### ShutdownResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Shutdown](#machine.Shutdown) | repeated |  |






<a name="machine.SoftIRQStat"></a>

### SoftIRQStat



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| hi | [uint64](#uint64) |  |  |
| timer | [uint64](#uint64) |  |  |
| net_tx | [uint64](#uint64) |  |  |
| net_rx | [uint64](#uint64) |  |  |
| block | [uint64](#uint64) |  |  |
| block_io_poll | [uint64](#uint64) |  |  |
| tasklet | [uint64](#uint64) |  |  |
| sched | [uint64](#uint64) |  |  |
| hrtimer | [uint64](#uint64) |  |  |
| rcu | [uint64](#uint64) |  |  |






<a name="machine.Stat"></a>

### Stat
The messages message containing the requested stat.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| namespace | [string](#string) |  |  |
| id | [string](#string) |  |  |
| memory_usage | [uint64](#uint64) |  |  |
| cpu_usage | [uint64](#uint64) |  |  |
| pod_id | [string](#string) |  |  |
| name | [string](#string) |  |  |






<a name="machine.Stats"></a>

### Stats
The messages message containing the requested stats.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| stats | [Stat](#machine.Stat) | repeated |  |






<a name="machine.StatsRequest"></a>

### StatsRequest
The request message containing the containerd namespace.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| namespace | [string](#string) |  |  |
| driver | [common.ContainerDriver](#common.ContainerDriver) |  | driver might be default "containerd" or "cri" |






<a name="machine.StatsResponse"></a>

### StatsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Stats](#machine.Stats) | repeated |  |






<a name="machine.SystemStat"></a>

### SystemStat



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| boot_time | [uint64](#uint64) |  |  |
| cpu_total | [CPUStat](#machine.CPUStat) |  |  |
| cpu | [CPUStat](#machine.CPUStat) | repeated |  |
| irq_total | [uint64](#uint64) |  |  |
| irq | [uint64](#uint64) | repeated |  |
| context_switches | [uint64](#uint64) |  |  |
| process_created | [uint64](#uint64) |  |  |
| process_running | [uint64](#uint64) |  |  |
| process_blocked | [uint64](#uint64) |  |  |
| soft_irq_total | [uint64](#uint64) |  |  |
| soft_irq | [SoftIRQStat](#machine.SoftIRQStat) |  |  |






<a name="machine.SystemStatResponse"></a>

### SystemStatResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [SystemStat](#machine.SystemStat) | repeated |  |






<a name="machine.TaskEvent"></a>

### TaskEvent



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| task | [string](#string) |  |  |
| action | [TaskEvent.Action](#machine.TaskEvent.Action) |  |  |






<a name="machine.Upgrade"></a>

### Upgrade



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| ack | [string](#string) |  |  |
| actor_id | [string](#string) |  |  |






<a name="machine.UpgradeRequest"></a>

### UpgradeRequest
rpc upgrade


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| image | [string](#string) |  |  |
| preserve | [bool](#bool) |  |  |
| stage | [bool](#bool) |  |  |
| force | [bool](#bool) |  |  |
| reboot_mode | [UpgradeRequest.RebootMode](#machine.UpgradeRequest.RebootMode) |  |  |






<a name="machine.UpgradeResponse"></a>

### UpgradeResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Upgrade](#machine.Upgrade) | repeated |  |






<a name="machine.Version"></a>

### Version



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| version | [VersionInfo](#machine.VersionInfo) |  |  |
| platform | [PlatformInfo](#machine.PlatformInfo) |  |  |
| features | [FeaturesInfo](#machine.FeaturesInfo) |  | Features describe individual Talos features that can be switched on or off. |






<a name="machine.VersionInfo"></a>

### VersionInfo



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tag | [string](#string) |  |  |
| sha | [string](#string) |  |  |
| built | [string](#string) |  |  |
| go_version | [string](#string) |  |  |
| os | [string](#string) |  |  |
| arch | [string](#string) |  |  |






<a name="machine.VersionResponse"></a>

### VersionResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Version](#machine.Version) | repeated |  |






<a name="machine.Xattr"></a>

### Xattr



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| data | [bytes](#bytes) |  |  |





 <!-- end messages -->


<a name="machine.ApplyConfigurationRequest.Mode"></a>

### ApplyConfigurationRequest.Mode


| Name | Number | Description |
| ---- | ------ | ----------- |
| REBOOT | 0 |  |
| AUTO | 1 |  |
| NO_REBOOT | 2 |  |
| STAGED | 3 |  |
| TRY | 4 |  |



<a name="machine.ConnectRecord.State"></a>

### ConnectRecord.State


| Name | Number | Description |
| ---- | ------ | ----------- |
| RESERVED | 0 |  |
| ESTABLISHED | 1 |  |
| SYN_SENT | 2 |  |
| SYN_RECV | 3 |  |
| FIN_WAIT1 | 4 |  |
| FIN_WAIT2 | 5 |  |
| TIME_WAIT | 6 |  |
| CLOSE | 7 |  |
| CLOSEWAIT | 8 |  |
| LASTACK | 9 |  |
| LISTEN | 10 |  |
| CLOSING | 11 |  |



<a name="machine.ConnectRecord.TimerActive"></a>

### ConnectRecord.TimerActive


| Name | Number | Description |
| ---- | ------ | ----------- |
| OFF | 0 |  |
| ON | 1 |  |
| KEEPALIVE | 2 |  |
| TIMEWAIT | 3 |  |
| PROBE | 4 |  |



<a name="machine.EtcdMemberAlarm.AlarmType"></a>

### EtcdMemberAlarm.AlarmType


| Name | Number | Description |
| ---- | ------ | ----------- |
| NONE | 0 |  |
| NOSPACE | 1 |  |
| CORRUPT | 2 |  |



<a name="machine.ListRequest.Type"></a>

### ListRequest.Type
File type.

| Name | Number | Description |
| ---- | ------ | ----------- |
| REGULAR | 0 | Regular file (not directory, symlink, etc). |
| DIRECTORY | 1 | Directory. |
| SYMLINK | 2 | Symbolic link. |



<a name="machine.MachineConfig.MachineType"></a>

### MachineConfig.MachineType


| Name | Number | Description |
| ---- | ------ | ----------- |
| TYPE_UNKNOWN | 0 |  |
| TYPE_INIT | 1 |  |
| TYPE_CONTROL_PLANE | 2 |  |
| TYPE_WORKER | 3 |  |



<a name="machine.MachineStatusEvent.MachineStage"></a>

### MachineStatusEvent.MachineStage


| Name | Number | Description |
| ---- | ------ | ----------- |
| UNKNOWN | 0 |  |
| BOOTING | 1 |  |
| INSTALLING | 2 |  |
| MAINTENANCE | 3 |  |
| RUNNING | 4 |  |
| REBOOTING | 5 |  |
| SHUTTING_DOWN | 6 |  |
| RESETTING | 7 |  |
| UPGRADING | 8 |  |



<a name="machine.NetstatRequest.Filter"></a>

### NetstatRequest.Filter


| Name | Number | Description |
| ---- | ------ | ----------- |
| ALL | 0 |  |
| CONNECTED | 1 |  |
| LISTENING | 2 |  |



<a name="machine.PhaseEvent.Action"></a>

### PhaseEvent.Action


| Name | Number | Description |
| ---- | ------ | ----------- |
| START | 0 |  |
| STOP | 1 |  |



<a name="machine.RebootRequest.Mode"></a>

### RebootRequest.Mode


| Name | Number | Description |
| ---- | ------ | ----------- |
| DEFAULT | 0 |  |
| POWERCYCLE | 1 |  |



<a name="machine.ResetRequest.WipeMode"></a>

### ResetRequest.WipeMode


| Name | Number | Description |
| ---- | ------ | ----------- |
| ALL | 0 |  |
| SYSTEM_DISK | 1 |  |
| USER_DISKS | 2 |  |



<a name="machine.SequenceEvent.Action"></a>

### SequenceEvent.Action


| Name | Number | Description |
| ---- | ------ | ----------- |
| NOOP | 0 |  |
| START | 1 |  |
| STOP | 2 |  |



<a name="machine.ServiceStateEvent.Action"></a>

### ServiceStateEvent.Action


| Name | Number | Description |
| ---- | ------ | ----------- |
| INITIALIZED | 0 |  |
| PREPARING | 1 |  |
| WAITING | 2 |  |
| RUNNING | 3 |  |
| STOPPING | 4 |  |
| FINISHED | 5 |  |
| FAILED | 6 |  |
| SKIPPED | 7 |  |
| STARTING | 8 |  |



<a name="machine.TaskEvent.Action"></a>

### TaskEvent.Action


| Name | Number | Description |
| ---- | ------ | ----------- |
| START | 0 |  |
| STOP | 1 |  |



<a name="machine.UpgradeRequest.RebootMode"></a>

### UpgradeRequest.RebootMode


| Name | Number | Description |
| ---- | ------ | ----------- |
| DEFAULT | 0 |  |
| POWERCYCLE | 1 |  |


 <!-- end enums -->

 <!-- end HasExtensions -->


<a name="machine.MachineService"></a>

### MachineService
The machine service definition.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| ApplyConfiguration | [ApplyConfigurationRequest](#machine.ApplyConfigurationRequest) | [ApplyConfigurationResponse](#machine.ApplyConfigurationResponse) |  |
| Bootstrap | [BootstrapRequest](#machine.BootstrapRequest) | [BootstrapResponse](#machine.BootstrapResponse) | Bootstrap method makes control plane node enter etcd bootstrap mode. Node aborts etcd join sequence and creates single-node etcd cluster. If recover_etcd argument is specified, etcd is recovered from a snapshot uploaded with EtcdRecover. |
| Containers | [ContainersRequest](#machine.ContainersRequest) | [ContainersResponse](#machine.ContainersResponse) |  |
| Copy | [CopyRequest](#machine.CopyRequest) | [.common.Data](#common.Data) stream |  |
| CPUFreqStats | [.google.protobuf.Empty](#google.protobuf.Empty) | [CPUFreqStatsResponse](#machine.CPUFreqStatsResponse) |  |
| CPUInfo | [.google.protobuf.Empty](#google.protobuf.Empty) | [CPUInfoResponse](#machine.CPUInfoResponse) |  |
| DiskStats | [.google.protobuf.Empty](#google.protobuf.Empty) | [DiskStatsResponse](#machine.DiskStatsResponse) |  |
| Dmesg | [DmesgRequest](#machine.DmesgRequest) | [.common.Data](#common.Data) stream |  |
| Events | [EventsRequest](#machine.EventsRequest) | [Event](#machine.Event) stream |  |
| EtcdMemberList | [EtcdMemberListRequest](#machine.EtcdMemberListRequest) | [EtcdMemberListResponse](#machine.EtcdMemberListResponse) |  |
| EtcdRemoveMemberByID | [EtcdRemoveMemberByIDRequest](#machine.EtcdRemoveMemberByIDRequest) | [EtcdRemoveMemberByIDResponse](#machine.EtcdRemoveMemberByIDResponse) | EtcdRemoveMemberByID removes a member from the etcd cluster identified by member ID. This API should be used to remove members which don't have an associated Talos node anymore. To remove a member with a running Talos node, use EtcdLeaveCluster API on the node to be removed. |
| EtcdLeaveCluster | [EtcdLeaveClusterRequest](#machine.EtcdLeaveClusterRequest) | [EtcdLeaveClusterResponse](#machine.EtcdLeaveClusterResponse) |  |
| EtcdForfeitLeadership | [EtcdForfeitLeadershipRequest](#machine.EtcdForfeitLeadershipRequest) | [EtcdForfeitLeadershipResponse](#machine.EtcdForfeitLeadershipResponse) |  |
| EtcdRecover | [.common.Data](#common.Data) stream | [EtcdRecoverResponse](#machine.EtcdRecoverResponse) | EtcdRecover method uploads etcd data snapshot created with EtcdSnapshot to the node. Snapshot can be later used to recover the cluster via Bootstrap method. |
| EtcdSnapshot | [EtcdSnapshotRequest](#machine.EtcdSnapshotRequest) | [.common.Data](#common.Data) stream | EtcdSnapshot method creates etcd data snapshot (backup) from the local etcd instance and streams it back to the client. This method is available only on control plane nodes (which run etcd). |
| EtcdAlarmList | [.google.protobuf.Empty](#google.protobuf.Empty) | [EtcdAlarmListResponse](#machine.EtcdAlarmListResponse) | EtcdAlarmList lists etcd alarms for the current node. This method is available only on control plane nodes (which run etcd). |
| EtcdAlarmDisarm | [.google.protobuf.Empty](#google.protobuf.Empty) | [EtcdAlarmDisarmResponse](#machine.EtcdAlarmDisarmResponse) | EtcdAlarmDisarm disarms etcd alarms for the current node. This method is available only on control plane nodes (which run etcd). |
| EtcdDefragment | [.google.protobuf.Empty](#google.protobuf.Empty) | [EtcdDefragmentResponse](#machine.EtcdDefragmentResponse) | EtcdDefragment defragments etcd data directory for the current node. Defragmentation is a resource-heavy operation, so it should only run on a specific node. This method is available only on control plane nodes (which run etcd). |
| EtcdStatus | [.google.protobuf.Empty](#google.protobuf.Empty) | [EtcdStatusResponse](#machine.EtcdStatusResponse) | EtcdStatus returns etcd status for the current member. This method is available only on control plane nodes (which run etcd). |
| GenerateConfiguration | [GenerateConfigurationRequest](#machine.GenerateConfigurationRequest) | [GenerateConfigurationResponse](#machine.GenerateConfigurationResponse) |  |
| Hostname | [.google.protobuf.Empty](#google.protobuf.Empty) | [HostnameResponse](#machine.HostnameResponse) |  |
| Kubeconfig | [.google.protobuf.Empty](#google.protobuf.Empty) | [.common.Data](#common.Data) stream |  |
| List | [ListRequest](#machine.ListRequest) | [FileInfo](#machine.FileInfo) stream |  |
| DiskUsage | [DiskUsageRequest](#machine.DiskUsageRequest) | [DiskUsageInfo](#machine.DiskUsageInfo) stream |  |
| LoadAvg | [.google.protobuf.Empty](#google.protobuf.Empty) | [LoadAvgResponse](#machine.LoadAvgResponse) |  |
| Logs | [LogsRequest](#machine.LogsRequest) | [.common.Data](#common.Data) stream |  |
| LogsContainers | [.google.protobuf.Empty](#google.protobuf.Empty) | [LogsContainersResponse](#machine.LogsContainersResponse) |  |
| Memory | [.google.protobuf.Empty](#google.protobuf.Empty) | [MemoryResponse](#machine.MemoryResponse) |  |
| Mounts | [.google.protobuf.Empty](#google.protobuf.Empty) | [MountsResponse](#machine.MountsResponse) |  |
| NetworkDeviceStats | [.google.protobuf.Empty](#google.protobuf.Empty) | [NetworkDeviceStatsResponse](#machine.NetworkDeviceStatsResponse) |  |
| Processes | [.google.protobuf.Empty](#google.protobuf.Empty) | [ProcessesResponse](#machine.ProcessesResponse) |  |
| Read | [ReadRequest](#machine.ReadRequest) | [.common.Data](#common.Data) stream |  |
| Reboot | [RebootRequest](#machine.RebootRequest) | [RebootResponse](#machine.RebootResponse) |  |
| Restart | [RestartRequest](#machine.RestartRequest) | [RestartResponse](#machine.RestartResponse) |  |
| Rollback | [RollbackRequest](#machine.RollbackRequest) | [RollbackResponse](#machine.RollbackResponse) |  |
| Reset | [ResetRequest](#machine.ResetRequest) | [ResetResponse](#machine.ResetResponse) |  |
| ServiceList | [.google.protobuf.Empty](#google.protobuf.Empty) | [ServiceListResponse](#machine.ServiceListResponse) |  |
| ServiceRestart | [ServiceRestartRequest](#machine.ServiceRestartRequest) | [ServiceRestartResponse](#machine.ServiceRestartResponse) |  |
| ServiceStart | [ServiceStartRequest](#machine.ServiceStartRequest) | [ServiceStartResponse](#machine.ServiceStartResponse) |  |
| ServiceStop | [ServiceStopRequest](#machine.ServiceStopRequest) | [ServiceStopResponse](#machine.ServiceStopResponse) |  |
| Shutdown | [ShutdownRequest](#machine.ShutdownRequest) | [ShutdownResponse](#machine.ShutdownResponse) |  |
| Stats | [StatsRequest](#machine.StatsRequest) | [StatsResponse](#machine.StatsResponse) |  |
| SystemStat | [.google.protobuf.Empty](#google.protobuf.Empty) | [SystemStatResponse](#machine.SystemStatResponse) |  |
| Upgrade | [UpgradeRequest](#machine.UpgradeRequest) | [UpgradeResponse](#machine.UpgradeResponse) |  |
| Version | [.google.protobuf.Empty](#google.protobuf.Empty) | [VersionResponse](#machine.VersionResponse) |  |
| GenerateClientConfiguration | [GenerateClientConfigurationRequest](#machine.GenerateClientConfigurationRequest) | [GenerateClientConfigurationResponse](#machine.GenerateClientConfigurationResponse) | GenerateClientConfiguration generates talosctl client configuration (talosconfig). |
| PacketCapture | [PacketCaptureRequest](#machine.PacketCaptureRequest) | [.common.Data](#common.Data) stream | PacketCapture performs packet capture and streams back pcap file. |
| Netstat | [NetstatRequest](#machine.NetstatRequest) | [NetstatResponse](#machine.NetstatResponse) | Netstat provides information about network connections. |
| MetaWrite | [MetaWriteRequest](#machine.MetaWriteRequest) | [MetaWriteResponse](#machine.MetaWriteResponse) | MetaWrite writes a META key-value pair. |
| MetaDelete | [MetaDeleteRequest](#machine.MetaDeleteRequest) | [MetaDeleteResponse](#machine.MetaDeleteResponse) | MetaDelete deletes a META key. |
| ImageList | [ImageListRequest](#machine.ImageListRequest) | [ImageListResponse](#machine.ImageListResponse) stream | ImageList lists images in the CRI. |
| ImagePull | [ImagePullRequest](#machine.ImagePullRequest) | [ImagePullResponse](#machine.ImagePullResponse) | ImagePull pulls an image into the CRI. |

 <!-- end services -->



<a name="security/security.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## security/security.proto



<a name="securityapi.CertificateRequest"></a>

### CertificateRequest
The request message containing the certificate signing request.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| csr | [bytes](#bytes) |  | Certificate Signing Request in PEM format. |






<a name="securityapi.CertificateResponse"></a>

### CertificateResponse
The response message containing signed certificate.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ca | [bytes](#bytes) |  | Certificate of the CA that signed the requested certificate in PEM format. |
| crt | [bytes](#bytes) |  | Signed X.509 requested certificate in PEM format. |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->


<a name="securityapi.SecurityService"></a>

### SecurityService
The security service definition.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| Certificate | [CertificateRequest](#securityapi.CertificateRequest) | [CertificateResponse](#securityapi.CertificateResponse) |  |

 <!-- end services -->



<a name="storage/storage.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## storage/storage.proto



<a name="storage.BlockDeviceWipe"></a>

### BlockDeviceWipe



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |






<a name="storage.BlockDeviceWipeDescriptor"></a>

### BlockDeviceWipeDescriptor
BlockDeviceWipeDescriptor represents a single block device to be wiped.

The device can be either a full disk (e.g. vda) or a partition (vda5).
The device should not be used in any of active volumes.
The device should not be used as a secondary (e.g. part of LVM).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| device | [string](#string) |  | Device name to wipe (e.g. sda or sda5).

The name should be submitted without `/dev/` prefix. |
| method | [BlockDeviceWipeDescriptor.Method](#storage.BlockDeviceWipeDescriptor.Method) |  | Wipe method to use. |
| skip_volume_check | [bool](#bool) |  | Skip the volume in use check. |
| drop_partition | [bool](#bool) |  | Drop the partition (only applies if the device is a partition). |






<a name="storage.BlockDeviceWipeRequest"></a>

### BlockDeviceWipeRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| devices | [BlockDeviceWipeDescriptor](#storage.BlockDeviceWipeDescriptor) | repeated |  |






<a name="storage.BlockDeviceWipeResponse"></a>

### BlockDeviceWipeResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [BlockDeviceWipe](#storage.BlockDeviceWipe) | repeated |  |






<a name="storage.Disk"></a>

### Disk
Disk represents a disk.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| size | [uint64](#uint64) |  | Size indicates the disk size in bytes. |
| model | [string](#string) |  | Model idicates the disk model. |
| device_name | [string](#string) |  | DeviceName indicates the disk name (e.g. `sda`). |
| name | [string](#string) |  | Name as in `/sys/block/<dev>/device/name`. |
| serial | [string](#string) |  | Serial as in `/sys/block/<dev>/device/serial`. |
| modalias | [string](#string) |  | Modalias as in `/sys/block/<dev>/device/modalias`. |
| uuid | [string](#string) |  | Uuid as in `/sys/block/<dev>/device/uuid`. |
| wwid | [string](#string) |  | Wwid as in `/sys/block/<dev>/device/wwid`. |
| type | [Disk.DiskType](#storage.Disk.DiskType) |  | Type is a type of the disk: nvme, ssd, hdd, sd card. |
| bus_path | [string](#string) |  | BusPath is the bus path of the disk. |
| system_disk | [bool](#bool) |  | SystemDisk indicates that the disk is used as Talos system disk. |
| subsystem | [string](#string) |  | Subsystem is the symlink path in the `/sys/block/<dev>/subsystem`. |
| readonly | [bool](#bool) |  | Readonly specifies if the disk is read only. |






<a name="storage.Disks"></a>

### Disks
DisksResponse represents the response of the `Disks` RPC.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| disks | [Disk](#storage.Disk) | repeated |  |






<a name="storage.DisksResponse"></a>

### DisksResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Disks](#storage.Disks) | repeated |  |





 <!-- end messages -->


<a name="storage.BlockDeviceWipeDescriptor.Method"></a>

### BlockDeviceWipeDescriptor.Method


| Name | Number | Description |
| ---- | ------ | ----------- |
| FAST | 0 | Fast wipe - wipe only filesystem signatures. |
| ZEROES | 1 | Zeroes wipe - wipe by overwriting with zeroes (might be slow depending on the disk size and available hardware features). |



<a name="storage.Disk.DiskType"></a>

### Disk.DiskType


| Name | Number | Description |
| ---- | ------ | ----------- |
| UNKNOWN | 0 |  |
| SSD | 1 |  |
| HDD | 2 |  |
| NVME | 3 |  |
| SD | 4 |  |
| CD | 5 |  |


 <!-- end enums -->

 <!-- end HasExtensions -->


<a name="storage.StorageService"></a>

### StorageService
StorageService represents the storage service.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| Disks | [.google.protobuf.Empty](#google.protobuf.Empty) | [DisksResponse](#storage.DisksResponse) |  |
| BlockDeviceWipe | [BlockDeviceWipeRequest](#storage.BlockDeviceWipeRequest) | [BlockDeviceWipeResponse](#storage.BlockDeviceWipeResponse) | BlockDeviceWipe performs a wipe of the blockdevice (partition or disk).

The method doesn't require a reboot, and it can only wipe blockdevices which are not being used as volumes at the moment. Wiping of volumes requires a different API. |

 <!-- end services -->



<a name="time/time.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## time/time.proto



<a name="time.Time"></a>

### Time



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| server | [string](#string) |  |  |
| localtime | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  |  |
| remotetime | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  |  |






<a name="time.TimeRequest"></a>

### TimeRequest
The response message containing the ntp server


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| server | [string](#string) |  |  |






<a name="time.TimeResponse"></a>

### TimeResponse
The response message containing the ntp server, time, and offset


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Time](#time.Time) | repeated |  |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->


<a name="time.TimeService"></a>

### TimeService
The time service definition.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| Time | [.google.protobuf.Empty](#google.protobuf.Empty) | [TimeResponse](#time.TimeResponse) |  |
| TimeCheck | [TimeRequest](#time.TimeRequest) | [TimeResponse](#time.TimeResponse) |  |

 <!-- end services -->



## Scalar Value Types

| .proto Type | Notes | C++ | Java | Python | Go | C# | PHP | Ruby |
| ----------- | ----- | --- | ---- | ------ | -- | -- | --- | ---- |
| <a name="double" /> double |  | double | double | float | float64 | double | float | Float |
| <a name="float" /> float |  | float | float | float | float32 | float | float | Float |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="uint32" /> uint32 | Uses variable-length encoding. | uint32 | int | int/long | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="uint64" /> uint64 | Uses variable-length encoding. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum or Fixnum (as required) |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | uint32 | int | int | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum |
| <a name="sfixed32" /> sfixed32 | Always four bytes. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="bool" /> bool |  | bool | boolean | boolean | bool | bool | boolean | TrueClass/FalseClass |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. | string | String | str/unicode | string | string | string | String (UTF-8) |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. | string | ByteString | str | []byte | ByteString | string | String (ASCII-8BIT) |

