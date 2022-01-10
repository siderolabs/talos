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
  
    - [Code](#common.Code)
    - [ContainerDriver](#common.ContainerDriver)
  
    - [File-level Extensions](#common/common.proto-extensions)
  
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
    - [Bootstrap](#machine.Bootstrap)
    - [BootstrapRequest](#machine.BootstrapRequest)
    - [BootstrapResponse](#machine.BootstrapResponse)
    - [CNIConfig](#machine.CNIConfig)
    - [CPUInfo](#machine.CPUInfo)
    - [CPUInfoResponse](#machine.CPUInfoResponse)
    - [CPUStat](#machine.CPUStat)
    - [CPUsInfo](#machine.CPUsInfo)
    - [ClusterConfig](#machine.ClusterConfig)
    - [ClusterNetworkConfig](#machine.ClusterNetworkConfig)
    - [ConfigLoadErrorEvent](#machine.ConfigLoadErrorEvent)
    - [ConfigValidationErrorEvent](#machine.ConfigValidationErrorEvent)
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
    - [EtcdForfeitLeadership](#machine.EtcdForfeitLeadership)
    - [EtcdForfeitLeadershipRequest](#machine.EtcdForfeitLeadershipRequest)
    - [EtcdForfeitLeadershipResponse](#machine.EtcdForfeitLeadershipResponse)
    - [EtcdLeaveCluster](#machine.EtcdLeaveCluster)
    - [EtcdLeaveClusterRequest](#machine.EtcdLeaveClusterRequest)
    - [EtcdLeaveClusterResponse](#machine.EtcdLeaveClusterResponse)
    - [EtcdMember](#machine.EtcdMember)
    - [EtcdMemberListRequest](#machine.EtcdMemberListRequest)
    - [EtcdMemberListResponse](#machine.EtcdMemberListResponse)
    - [EtcdMembers](#machine.EtcdMembers)
    - [EtcdRecover](#machine.EtcdRecover)
    - [EtcdRecoverResponse](#machine.EtcdRecoverResponse)
    - [EtcdRemoveMember](#machine.EtcdRemoveMember)
    - [EtcdRemoveMemberRequest](#machine.EtcdRemoveMemberRequest)
    - [EtcdRemoveMemberResponse](#machine.EtcdRemoveMemberResponse)
    - [EtcdSnapshotRequest](#machine.EtcdSnapshotRequest)
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
    - [InstallConfig](#machine.InstallConfig)
    - [ListRequest](#machine.ListRequest)
    - [LoadAvg](#machine.LoadAvg)
    - [LoadAvgResponse](#machine.LoadAvgResponse)
    - [LogsRequest](#machine.LogsRequest)
    - [MachineConfig](#machine.MachineConfig)
    - [MemInfo](#machine.MemInfo)
    - [Memory](#machine.Memory)
    - [MemoryResponse](#machine.MemoryResponse)
    - [MountStat](#machine.MountStat)
    - [Mounts](#machine.Mounts)
    - [MountsResponse](#machine.MountsResponse)
    - [NetDev](#machine.NetDev)
    - [NetworkConfig](#machine.NetworkConfig)
    - [NetworkDeviceConfig](#machine.NetworkDeviceConfig)
    - [NetworkDeviceStats](#machine.NetworkDeviceStats)
    - [NetworkDeviceStatsResponse](#machine.NetworkDeviceStatsResponse)
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
  
    - [ApplyConfigurationRequest.Mode](#machine.ApplyConfigurationRequest.Mode)
    - [ListRequest.Type](#machine.ListRequest.Type)
    - [MachineConfig.MachineType](#machine.MachineConfig.MachineType)
    - [PhaseEvent.Action](#machine.PhaseEvent.Action)
    - [RebootRequest.Mode](#machine.RebootRequest.Mode)
    - [SequenceEvent.Action](#machine.SequenceEvent.Action)
    - [ServiceStateEvent.Action](#machine.ServiceStateEvent.Action)
    - [TaskEvent.Action](#machine.TaskEvent.Action)
  
    - [MachineService](#machine.MachineService)
  
- [resource/resource.proto](#resource/resource.proto)
    - [Get](#resource.Get)
    - [GetRequest](#resource.GetRequest)
    - [GetResponse](#resource.GetResponse)
    - [ListRequest](#resource.ListRequest)
    - [ListResponse](#resource.ListResponse)
    - [Metadata](#resource.Metadata)
    - [Resource](#resource.Resource)
    - [Spec](#resource.Spec)
    - [WatchRequest](#resource.WatchRequest)
    - [WatchResponse](#resource.WatchResponse)
  
    - [EventType](#resource.EventType)
  
    - [ResourceService](#resource.ResourceService)
  
- [security/security.proto](#security/security.proto)
    - [CertificateRequest](#securityapi.CertificateRequest)
    - [CertificateResponse](#securityapi.CertificateResponse)
  
    - [SecurityService](#securityapi.SecurityService)
  
- [storage/storage.proto](#storage/storage.proto)
    - [Disk](#storage.Disk)
    - [Disks](#storage.Disks)
    - [DisksResponse](#storage.DisksResponse)
  
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





 <!-- end messages -->


<a name="common.Code"></a>

### Code


| Name | Number | Description |
| ---- | ------ | ----------- |
| FATAL | 0 |  |
| LOCKED | 1 |  |



<a name="common.ContainerDriver"></a>

### ContainerDriver


| Name | Number | Description |
| ---- | ------ | ----------- |
| CONTAINERD | 0 |  |
| CRI | 1 |  |


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

InspectService provides auxilary API to inspect OS internals.

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
| on_reboot | [bool](#bool) |  | **Deprecated.** replaced by mode |
| immediate | [bool](#bool) |  | **Deprecated.** replaced by mode |
| mode | [ApplyConfigurationRequest.Mode](#machine.ApplyConfigurationRequest.Mode) |  |  |






<a name="machine.ApplyConfigurationResponse"></a>

### ApplyConfigurationResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [ApplyConfiguration](#machine.ApplyConfiguration) | repeated |  |






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
| recover_etcd | [bool](#bool) |  | Enable etcd recovery from the snapshot.

Snapshot should be uploaded before this call via EtcdRecover RPC. |
| recover_skip_hash_check | [bool](#bool) |  | Skip hash check on the snapshot (etcd).

Enable this when recovering from data directory copy to skip integrity check. |






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
| allow_scheduling_on_masters | [bool](#bool) |  |  |






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
| image | [string](#string) |  |  |
| pid | [uint32](#uint32) |  |  |
| status | [string](#string) |  |  |
| pod_id | [string](#string) |  |  |
| name | [string](#string) |  |  |






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







<a name="machine.Event"></a>

### Event



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| data | [google.protobuf.Any](#google.protobuf.Any) |  |  |
| id | [string](#string) |  |  |






<a name="machine.EventsRequest"></a>

### EventsRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tail_events | [int32](#int32) |  |  |
| tail_id | [string](#string) |  |  |
| tail_seconds | [int32](#int32) |  |  |






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






<a name="machine.UpgradeRequest"></a>

### UpgradeRequest
rpc upgrade


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| image | [string](#string) |  |  |
| preserve | [bool](#bool) |  |  |
| stage | [bool](#bool) |  |  |
| force | [bool](#bool) |  |  |






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





 <!-- end messages -->


<a name="machine.ApplyConfigurationRequest.Mode"></a>

### ApplyConfigurationRequest.Mode


| Name | Number | Description |
| ---- | ------ | ----------- |
| REBOOT | 0 |  |
| AUTO | 1 |  |
| NO_REBOOT | 2 |  |
| STAGED | 3 |  |



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



<a name="machine.TaskEvent.Action"></a>

### TaskEvent.Action


| Name | Number | Description |
| ---- | ------ | ----------- |
| START | 0 |  |
| STOP | 1 |  |


 <!-- end enums -->

 <!-- end HasExtensions -->


<a name="machine.MachineService"></a>

### MachineService
The machine service definition.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| ApplyConfiguration | [ApplyConfigurationRequest](#machine.ApplyConfigurationRequest) | [ApplyConfigurationResponse](#machine.ApplyConfigurationResponse) |  |
| Bootstrap | [BootstrapRequest](#machine.BootstrapRequest) | [BootstrapResponse](#machine.BootstrapResponse) | Bootstrap method makes control plane node enter etcd bootstrap mode.

Node aborts etcd join sequence and creates single-node etcd cluster.

If recover_etcd argument is specified, etcd is recovered from a snapshot uploaded with EtcdRecover. |
| Containers | [ContainersRequest](#machine.ContainersRequest) | [ContainersResponse](#machine.ContainersResponse) |  |
| Copy | [CopyRequest](#machine.CopyRequest) | [.common.Data](#common.Data) stream |  |
| CPUInfo | [.google.protobuf.Empty](#google.protobuf.Empty) | [CPUInfoResponse](#machine.CPUInfoResponse) |  |
| DiskStats | [.google.protobuf.Empty](#google.protobuf.Empty) | [DiskStatsResponse](#machine.DiskStatsResponse) |  |
| Dmesg | [DmesgRequest](#machine.DmesgRequest) | [.common.Data](#common.Data) stream |  |
| Events | [EventsRequest](#machine.EventsRequest) | [Event](#machine.Event) stream |  |
| EtcdMemberList | [EtcdMemberListRequest](#machine.EtcdMemberListRequest) | [EtcdMemberListResponse](#machine.EtcdMemberListResponse) |  |
| EtcdRemoveMember | [EtcdRemoveMemberRequest](#machine.EtcdRemoveMemberRequest) | [EtcdRemoveMemberResponse](#machine.EtcdRemoveMemberResponse) |  |
| EtcdLeaveCluster | [EtcdLeaveClusterRequest](#machine.EtcdLeaveClusterRequest) | [EtcdLeaveClusterResponse](#machine.EtcdLeaveClusterResponse) |  |
| EtcdForfeitLeadership | [EtcdForfeitLeadershipRequest](#machine.EtcdForfeitLeadershipRequest) | [EtcdForfeitLeadershipResponse](#machine.EtcdForfeitLeadershipResponse) |  |
| EtcdRecover | [.common.Data](#common.Data) stream | [EtcdRecoverResponse](#machine.EtcdRecoverResponse) | EtcdRecover method uploads etcd data snapshot created with EtcdSnapshot to the node.

Snapshot can be later used to recover the cluster via Bootstrap method. |
| EtcdSnapshot | [EtcdSnapshotRequest](#machine.EtcdSnapshotRequest) | [.common.Data](#common.Data) stream | EtcdSnapshot method creates etcd data snapshot (backup) from the local etcd instance and streams it back to the client.

This method is available only on control plane nodes (which run etcd). |
| GenerateConfiguration | [GenerateConfigurationRequest](#machine.GenerateConfigurationRequest) | [GenerateConfigurationResponse](#machine.GenerateConfigurationResponse) |  |
| Hostname | [.google.protobuf.Empty](#google.protobuf.Empty) | [HostnameResponse](#machine.HostnameResponse) |  |
| Kubeconfig | [.google.protobuf.Empty](#google.protobuf.Empty) | [.common.Data](#common.Data) stream |  |
| List | [ListRequest](#machine.ListRequest) | [FileInfo](#machine.FileInfo) stream |  |
| DiskUsage | [DiskUsageRequest](#machine.DiskUsageRequest) | [DiskUsageInfo](#machine.DiskUsageInfo) stream |  |
| LoadAvg | [.google.protobuf.Empty](#google.protobuf.Empty) | [LoadAvgResponse](#machine.LoadAvgResponse) |  |
| Logs | [LogsRequest](#machine.LogsRequest) | [.common.Data](#common.Data) stream |  |
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
| Shutdown | [.google.protobuf.Empty](#google.protobuf.Empty) | [ShutdownResponse](#machine.ShutdownResponse) |  |
| Stats | [StatsRequest](#machine.StatsRequest) | [StatsResponse](#machine.StatsResponse) |  |
| SystemStat | [.google.protobuf.Empty](#google.protobuf.Empty) | [SystemStatResponse](#machine.SystemStatResponse) |  |
| Upgrade | [UpgradeRequest](#machine.UpgradeRequest) | [UpgradeResponse](#machine.UpgradeResponse) |  |
| Version | [.google.protobuf.Empty](#google.protobuf.Empty) | [VersionResponse](#machine.VersionResponse) |  |
| GenerateClientConfiguration | [GenerateClientConfigurationRequest](#machine.GenerateClientConfigurationRequest) | [GenerateClientConfigurationResponse](#machine.GenerateClientConfigurationResponse) | GenerateClientConfiguration generates talosctl client configuration (talosconfig). |

 <!-- end services -->



<a name="resource/resource.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## resource/resource.proto



<a name="resource.Get"></a>

### Get
The GetResponse message contains the Resource returned.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| definition | [Resource](#resource.Resource) |  |  |
| resource | [Resource](#resource.Resource) |  |  |






<a name="resource.GetRequest"></a>

### GetRequest
rpc Get


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| namespace | [string](#string) |  |  |
| type | [string](#string) |  |  |
| id | [string](#string) |  |  |






<a name="resource.GetResponse"></a>

### GetResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Get](#resource.Get) | repeated |  |






<a name="resource.ListRequest"></a>

### ListRequest
rpc List
The ListResponse message contains the Resource returned.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| namespace | [string](#string) |  |  |
| type | [string](#string) |  |  |






<a name="resource.ListResponse"></a>

### ListResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| definition | [Resource](#resource.Resource) |  |  |
| resource | [Resource](#resource.Resource) |  |  |






<a name="resource.Metadata"></a>

### Metadata



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| namespace | [string](#string) |  |  |
| type | [string](#string) |  |  |
| id | [string](#string) |  |  |
| version | [string](#string) |  |  |
| owner | [string](#string) |  |  |
| phase | [string](#string) |  |  |
| created | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  |  |
| updated | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  |  |
| finalizers | [string](#string) | repeated |  |






<a name="resource.Resource"></a>

### Resource



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [Metadata](#resource.Metadata) |  |  |
| spec | [Spec](#resource.Spec) |  |  |






<a name="resource.Spec"></a>

### Spec



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| yaml | [bytes](#bytes) |  |  |






<a name="resource.WatchRequest"></a>

### WatchRequest
rpc Watch
The WatchResponse message contains the Resource returned.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| namespace | [string](#string) |  |  |
| type | [string](#string) |  |  |
| id | [string](#string) |  |  |
| tail_events | [uint32](#uint32) |  |  |






<a name="resource.WatchResponse"></a>

### WatchResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| event_type | [EventType](#resource.EventType) |  |  |
| definition | [Resource](#resource.Resource) |  |  |
| resource | [Resource](#resource.Resource) |  |  |





 <!-- end messages -->


<a name="resource.EventType"></a>

### EventType


| Name | Number | Description |
| ---- | ------ | ----------- |
| CREATED | 0 |  |
| UPDATED | 1 |  |
| DESTROYED | 2 |  |


 <!-- end enums -->

 <!-- end HasExtensions -->


<a name="resource.ResourceService"></a>

### ResourceService
The resource service definition.

ResourceService provides user-facing API for the Talos resources.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| Get | [GetRequest](#resource.GetRequest) | [GetResponse](#resource.GetResponse) |  |
| List | [ListRequest](#resource.ListRequest) | [ListResponse](#resource.ListResponse) stream |  |
| Watch | [WatchRequest](#resource.WatchRequest) | [WatchResponse](#resource.WatchResponse) stream |  |

 <!-- end services -->



<a name="security/security.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## security/security.proto



<a name="securityapi.CertificateRequest"></a>

### CertificateRequest
The request message containing the process name.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| csr | [bytes](#bytes) |  |  |






<a name="securityapi.CertificateResponse"></a>

### CertificateResponse
The response message containing the requested logs.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ca | [bytes](#bytes) |  |  |
| crt | [bytes](#bytes) |  |  |





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


<a name="storage.Disk.DiskType"></a>

### Disk.DiskType


| Name | Number | Description |
| ---- | ------ | ----------- |
| UNKNOWN | 0 |  |
| SSD | 1 |  |
| HDD | 2 |  |
| NVME | 3 |  |
| SD | 4 |  |


 <!-- end enums -->

 <!-- end HasExtensions -->


<a name="storage.StorageService"></a>

### StorageService
StorageService represents the storage service.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| Disks | [.google.protobuf.Empty](#google.protobuf.Empty) | [DisksResponse](#storage.DisksResponse) |  |

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

