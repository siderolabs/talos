# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [machine/machine.proto](#machine/machine.proto)
    - [Bootstrap](#machine.Bootstrap)
    - [BootstrapRequest](#machine.BootstrapRequest)
    - [BootstrapResponse](#machine.BootstrapResponse)
    - [CopyRequest](#machine.CopyRequest)
    - [Event](#machine.Event)
    - [EventsRequest](#machine.EventsRequest)
    - [FileInfo](#machine.FileInfo)
    - [ListRequest](#machine.ListRequest)
    - [LogsRequest](#machine.LogsRequest)
    - [MountStat](#machine.MountStat)
    - [Mounts](#machine.Mounts)
    - [MountsResponse](#machine.MountsResponse)
    - [PhaseEvent](#machine.PhaseEvent)
    - [PlatformInfo](#machine.PlatformInfo)
    - [ReadRequest](#machine.ReadRequest)
    - [Reboot](#machine.Reboot)
    - [RebootResponse](#machine.RebootResponse)
    - [Recover](#machine.Recover)
    - [RecoverRequest](#machine.RecoverRequest)
    - [RecoverResponse](#machine.RecoverResponse)
    - [Reset](#machine.Reset)
    - [ResetRequest](#machine.ResetRequest)
    - [ResetResponse](#machine.ResetResponse)
    - [Rollback](#machine.Rollback)
    - [RollbackRequest](#machine.RollbackRequest)
    - [RollbackResponse](#machine.RollbackResponse)
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
    - [ServiceStop](#machine.ServiceStop)
    - [ServiceStopRequest](#machine.ServiceStopRequest)
    - [ServiceStopResponse](#machine.ServiceStopResponse)
    - [Shutdown](#machine.Shutdown)
    - [ShutdownResponse](#machine.ShutdownResponse)
    - [StartRequest](#machine.StartRequest)
    - [StartResponse](#machine.StartResponse)
    - [StopRequest](#machine.StopRequest)
    - [StopResponse](#machine.StopResponse)
    - [TaskEvent](#machine.TaskEvent)
    - [Upgrade](#machine.Upgrade)
    - [UpgradeRequest](#machine.UpgradeRequest)
    - [UpgradeResponse](#machine.UpgradeResponse)
    - [Version](#machine.Version)
    - [VersionInfo](#machine.VersionInfo)
    - [VersionResponse](#machine.VersionResponse)
  
    - [PhaseEvent.Action](#machine.PhaseEvent.Action)
    - [RecoverRequest.Source](#machine.RecoverRequest.Source)
    - [SequenceEvent.Action](#machine.SequenceEvent.Action)
    - [TaskEvent.Action](#machine.TaskEvent.Action)
  
    - [MachineService](#machine.MachineService)
  
- [Scalar Value Types](#scalar-value-types)



<a name="machine/machine.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## machine/machine.proto



<a name="machine.Bootstrap"></a>

### Bootstrap
The bootstrap message containing the bootstrap status.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |






<a name="machine.BootstrapRequest"></a>

### BootstrapRequest
rpc bootstrap






<a name="machine.BootstrapResponse"></a>

### BootstrapResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Bootstrap](#machine.Bootstrap) | repeated |  |






<a name="machine.CopyRequest"></a>

### CopyRequest
CopyRequest describes a request to copy data out of Talos node

Copy produces .tar.gz archive which is streamed back to the caller


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| root_path | [string](#string) |  | Root path to start copying data out, it might be either a file or directory |






<a name="machine.Event"></a>

### Event



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| data | [google.protobuf.Any](#google.protobuf.Any) |  |  |






<a name="machine.EventsRequest"></a>

### EventsRequest







<a name="machine.FileInfo"></a>

### FileInfo
FileInfo describes a file or directory&#39;s information


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| name | [string](#string) |  | Name is the name (including prefixed path) of the file or directory |
| size | [int64](#int64) |  | Size indicates the number of bytes contained within the file |
| mode | [uint32](#uint32) |  | Mode is the bitmap of UNIX mode/permission flags of the file |
| modified | [int64](#int64) |  | Modified indicates the UNIX timestamp at which the file was last modified

TODO: unix timestamp or include proto&#39;s Date type |
| is_dir | [bool](#bool) |  | IsDir indicates that the file is a directory |
| error | [string](#string) |  | Error describes any error encountered while trying to read the file information. |
| link | [string](#string) |  | Link is filled with symlink target |
| relative_name | [string](#string) |  | RelativeName is the name of the file or directory relative to the RootPath |






<a name="machine.ListRequest"></a>

### ListRequest
ListRequest describes a request to list the contents of a directory


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| root | [string](#string) |  | Root indicates the root directory for the list. If not indicated, &#39;/&#39; is presumed. |
| recurse | [bool](#bool) |  | Recurse indicates that subdirectories should be recursed. |
| recursion_depth | [int32](#int32) |  | RecursionDepth indicates how many levels of subdirectories should be recursed. The default (0) indicates that no limit should be enforced. |






<a name="machine.LogsRequest"></a>

### LogsRequest
rpc logs
The request message containing the process name.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| namespace | [string](#string) |  |  |
| id | [string](#string) |  |  |
| driver | [common.ContainerDriver](#common.ContainerDriver) |  | driver might be default &#34;containerd&#34; or &#34;cri&#34; |
| follow | [bool](#bool) |  |  |
| tail_lines | [int32](#int32) |  |  |






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






<a name="machine.ReadRequest"></a>

### ReadRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| path | [string](#string) |  |  |






<a name="machine.Reboot"></a>

### Reboot
rpc reboot
The reboot message containing the reboot status.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |






<a name="machine.RebootResponse"></a>

### RebootResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Reboot](#machine.Reboot) | repeated |  |






<a name="machine.Recover"></a>

### Recover
The recover message containing the recover status.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |






<a name="machine.RecoverRequest"></a>

### RecoverRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| source | [RecoverRequest.Source](#machine.RecoverRequest.Source) |  |  |






<a name="machine.RecoverResponse"></a>

### RecoverResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Recover](#machine.Recover) | repeated |  |






<a name="machine.Reset"></a>

### Reset
The reset message containing the restart status.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |






<a name="machine.ResetRequest"></a>

### ResetRequest
rpc reset


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| graceful | [bool](#bool) |  |  |
| reboot | [bool](#bool) |  |  |






<a name="machine.ResetResponse"></a>

### ResetResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Reset](#machine.Reset) | repeated |  |






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






<a name="machine.StartRequest"></a>

### StartRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |






<a name="machine.StartResponse"></a>

### StartResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| resp | [string](#string) |  |  |






<a name="machine.StopRequest"></a>

### StopRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |






<a name="machine.StopResponse"></a>

### StopResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| resp | [string](#string) |  |  |






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





 


<a name="machine.PhaseEvent.Action"></a>

### PhaseEvent.Action


| Name | Number | Description |
| ---- | ------ | ----------- |
| START | 0 |  |
| STOP | 1 |  |



<a name="machine.RecoverRequest.Source"></a>

### RecoverRequest.Source


| Name | Number | Description |
| ---- | ------ | ----------- |
| ETCD | 0 |  |
| APISERVER | 1 |  |



<a name="machine.SequenceEvent.Action"></a>

### SequenceEvent.Action


| Name | Number | Description |
| ---- | ------ | ----------- |
| NOOP | 0 |  |
| START | 1 |  |
| STOP | 2 |  |



<a name="machine.TaskEvent.Action"></a>

### TaskEvent.Action


| Name | Number | Description |
| ---- | ------ | ----------- |
| START | 0 |  |
| STOP | 1 |  |


 

 


<a name="machine.MachineService"></a>

### MachineService
The machine service definition.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| Bootstrap | [BootstrapRequest](#machine.BootstrapRequest) | [BootstrapResponse](#machine.BootstrapResponse) |  |
| Copy | [CopyRequest](#machine.CopyRequest) | [.common.Data](#common.Data) stream |  |
| Events | [EventsRequest](#machine.EventsRequest) | [Event](#machine.Event) stream |  |
| Kubeconfig | [.google.protobuf.Empty](#google.protobuf.Empty) | [.common.Data](#common.Data) stream |  |
| List | [ListRequest](#machine.ListRequest) | [FileInfo](#machine.FileInfo) stream |  |
| Logs | [LogsRequest](#machine.LogsRequest) | [.common.Data](#common.Data) stream |  |
| Mounts | [.google.protobuf.Empty](#google.protobuf.Empty) | [MountsResponse](#machine.MountsResponse) |  |
| Read | [ReadRequest](#machine.ReadRequest) | [.common.Data](#common.Data) stream |  |
| Reboot | [.google.protobuf.Empty](#google.protobuf.Empty) | [RebootResponse](#machine.RebootResponse) |  |
| Rollback | [RollbackRequest](#machine.RollbackRequest) | [RollbackResponse](#machine.RollbackResponse) |  |
| Reset | [ResetRequest](#machine.ResetRequest) | [ResetResponse](#machine.ResetResponse) |  |
| Recover | [RecoverRequest](#machine.RecoverRequest) | [RecoverResponse](#machine.RecoverResponse) |  |
| ServiceList | [.google.protobuf.Empty](#google.protobuf.Empty) | [ServiceListResponse](#machine.ServiceListResponse) |  |
| ServiceRestart | [ServiceRestartRequest](#machine.ServiceRestartRequest) | [ServiceRestartResponse](#machine.ServiceRestartResponse) |  |
| ServiceStart | [ServiceStartRequest](#machine.ServiceStartRequest) | [ServiceStartResponse](#machine.ServiceStartResponse) |  |
| ServiceStop | [ServiceStopRequest](#machine.ServiceStopRequest) | [ServiceStopResponse](#machine.ServiceStopResponse) |  |
| Shutdown | [.google.protobuf.Empty](#google.protobuf.Empty) | [ShutdownResponse](#machine.ShutdownResponse) |  |
| Upgrade | [UpgradeRequest](#machine.UpgradeRequest) | [UpgradeResponse](#machine.UpgradeResponse) |  |
| Version | [.google.protobuf.Empty](#google.protobuf.Empty) | [VersionResponse](#machine.VersionResponse) |  |

 



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

