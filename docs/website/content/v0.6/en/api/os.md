# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [os/os.proto](#os/os.proto)
    - [Container](#os.Container)
    - [ContainerInfo](#os.ContainerInfo)
    - [ContainersRequest](#os.ContainersRequest)
    - [ContainersResponse](#os.ContainersResponse)
    - [DmesgRequest](#os.DmesgRequest)
    - [MemInfo](#os.MemInfo)
    - [Memory](#os.Memory)
    - [MemoryResponse](#os.MemoryResponse)
    - [Process](#os.Process)
    - [ProcessInfo](#os.ProcessInfo)
    - [ProcessesRequest](#os.ProcessesRequest)
    - [ProcessesResponse](#os.ProcessesResponse)
    - [Restart](#os.Restart)
    - [RestartRequest](#os.RestartRequest)
    - [RestartResponse](#os.RestartResponse)
    - [Stat](#os.Stat)
    - [Stats](#os.Stats)
    - [StatsRequest](#os.StatsRequest)
    - [StatsResponse](#os.StatsResponse)
  
    - [OSService](#os.OSService)
  
- [Scalar Value Types](#scalar-value-types)



<a name="os/os.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## os/os.proto



<a name="os.Container"></a>

### Container
The messages message containing the requested containers.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| containers | [ContainerInfo](#os.ContainerInfo) | repeated |  |






<a name="os.ContainerInfo"></a>

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






<a name="os.ContainersRequest"></a>

### ContainersRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| namespace | [string](#string) |  |  |
| driver | [common.ContainerDriver](#common.ContainerDriver) |  | driver might be default &#34;containerd&#34; or &#34;cri&#34; |






<a name="os.ContainersResponse"></a>

### ContainersResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Container](#os.Container) | repeated |  |






<a name="os.DmesgRequest"></a>

### DmesgRequest
dmesg


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| follow | [bool](#bool) |  |  |
| tail | [bool](#bool) |  |  |






<a name="os.MemInfo"></a>

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






<a name="os.Memory"></a>

### Memory



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| meminfo | [MemInfo](#os.MemInfo) |  |  |






<a name="os.MemoryResponse"></a>

### MemoryResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Memory](#os.Memory) | repeated |  |






<a name="os.Process"></a>

### Process



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| processes | [ProcessInfo](#os.ProcessInfo) | repeated |  |






<a name="os.ProcessInfo"></a>

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






<a name="os.ProcessesRequest"></a>

### ProcessesRequest
rpc processes






<a name="os.ProcessesResponse"></a>

### ProcessesResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Process](#os.Process) | repeated |  |






<a name="os.Restart"></a>

### Restart



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |






<a name="os.RestartRequest"></a>

### RestartRequest
rpc restart
The request message containing the process to restart.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| namespace | [string](#string) |  |  |
| id | [string](#string) |  |  |
| driver | [common.ContainerDriver](#common.ContainerDriver) |  | driver might be default &#34;containerd&#34; or &#34;cri&#34; |






<a name="os.RestartResponse"></a>

### RestartResponse
The messages message containing the restart status.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Restart](#os.Restart) | repeated |  |






<a name="os.Stat"></a>

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






<a name="os.Stats"></a>

### Stats
The messages message containing the requested stats.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| stats | [Stat](#os.Stat) | repeated |  |






<a name="os.StatsRequest"></a>

### StatsRequest
The request message containing the containerd namespace.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| namespace | [string](#string) |  |  |
| driver | [common.ContainerDriver](#common.ContainerDriver) |  | driver might be default &#34;containerd&#34; or &#34;cri&#34; |






<a name="os.StatsResponse"></a>

### StatsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Stats](#os.Stats) | repeated |  |





 

 

 


<a name="os.OSService"></a>

### OSService
The OS service definition.

OS Service also implements all the API of Init Service

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| Containers | [ContainersRequest](#os.ContainersRequest) | [ContainersResponse](#os.ContainersResponse) |  |
| Dmesg | [DmesgRequest](#os.DmesgRequest) | [.common.Data](#common.Data) stream |  |
| Memory | [.google.protobuf.Empty](#google.protobuf.Empty) | [MemoryResponse](#os.MemoryResponse) |  |
| Processes | [.google.protobuf.Empty](#google.protobuf.Empty) | [ProcessesResponse](#os.ProcessesResponse) |  |
| Restart | [RestartRequest](#os.RestartRequest) | [RestartResponse](#os.RestartResponse) |  |
| Stats | [StatsRequest](#os.StatsRequest) | [StatsResponse](#os.StatsResponse) |  |

 



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

