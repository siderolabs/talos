# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [health/health.proto](#health/health.proto)
    - [HealthCheck](#health.HealthCheck)
    - [HealthCheckResponse](#health.HealthCheckResponse)
    - [HealthWatchRequest](#health.HealthWatchRequest)
    - [ReadyCheck](#health.ReadyCheck)
    - [ReadyCheckResponse](#health.ReadyCheckResponse)
  
    - [HealthCheck.ServingStatus](#health.HealthCheck.ServingStatus)
    - [ReadyCheck.ReadyStatus](#health.ReadyCheck.ReadyStatus)
  
    - [Health](#health.Health)
  
- [Scalar Value Types](#scalar-value-types)



<a name="health/health.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## health/health.proto



<a name="health.HealthCheck"></a>

### HealthCheck



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| status | [HealthCheck.ServingStatus](#health.HealthCheck.ServingStatus) |  |  |






<a name="health.HealthCheckResponse"></a>

### HealthCheckResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [HealthCheck](#health.HealthCheck) | repeated |  |






<a name="health.HealthWatchRequest"></a>

### HealthWatchRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| interval_seconds | [int64](#int64) |  |  |






<a name="health.ReadyCheck"></a>

### ReadyCheck



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| status | [ReadyCheck.ReadyStatus](#health.ReadyCheck.ReadyStatus) |  |  |






<a name="health.ReadyCheckResponse"></a>

### ReadyCheckResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [ReadyCheck](#health.ReadyCheck) | repeated |  |





 


<a name="health.HealthCheck.ServingStatus"></a>

### HealthCheck.ServingStatus


| Name | Number | Description |
| ---- | ------ | ----------- |
| UNKNOWN | 0 |  |
| SERVING | 1 |  |
| NOT_SERVING | 2 |  |



<a name="health.ReadyCheck.ReadyStatus"></a>

### ReadyCheck.ReadyStatus


| Name | Number | Description |
| ---- | ------ | ----------- |
| UNKNOWN | 0 |  |
| READY | 1 |  |
| NOT_READY | 2 |  |


 

 


<a name="health.Health"></a>

### Health


| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| Check | [.google.protobuf.Empty](#google.protobuf.Empty) | [HealthCheckResponse](#health.HealthCheckResponse) |  |
| Watch | [HealthWatchRequest](#health.HealthWatchRequest) | [HealthCheckResponse](#health.HealthCheckResponse) stream |  |
| Ready | [.google.protobuf.Empty](#google.protobuf.Empty) | [ReadyCheckResponse](#health.ReadyCheckResponse) |  |

 



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

