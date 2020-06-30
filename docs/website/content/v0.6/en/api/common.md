# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [common.proto](#common.proto)
    - [Data](#common.Data)
    - [DataResponse](#common.DataResponse)
    - [Empty](#common.Empty)
    - [EmptyResponse](#common.EmptyResponse)
    - [Error](#common.Error)
    - [Metadata](#common.Metadata)
  
    - [Code](#common.Code)
    - [ContainerDriver](#common.ContainerDriver)
  
- [Scalar Value Types](#scalar-value-types)



<a name="common.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## common.proto



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





 


<a name="common.Code"></a>

### Code


| Name | Number | Description |
| ---- | ------ | ----------- |
| FATAL | 0 |  |



<a name="common.ContainerDriver"></a>

### ContainerDriver


| Name | Number | Description |
| ---- | ------ | ----------- |
| CONTAINERD | 0 |  |
| CRI | 1 |  |


 

 

 



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

