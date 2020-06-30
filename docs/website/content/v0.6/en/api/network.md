# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [network/network.proto](#network/network.proto)
    - [Interface](#network.Interface)
    - [Interfaces](#network.Interfaces)
    - [InterfacesResponse](#network.InterfacesResponse)
    - [Route](#network.Route)
    - [Routes](#network.Routes)
    - [RoutesResponse](#network.RoutesResponse)
  
    - [AddressFamily](#network.AddressFamily)
    - [InterfaceFlags](#network.InterfaceFlags)
    - [RouteProtocol](#network.RouteProtocol)
  
    - [NetworkService](#network.NetworkService)
  
- [Scalar Value Types](#scalar-value-types)



<a name="network/network.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## network/network.proto



<a name="network.Interface"></a>

### Interface
Interface represents a net.Interface


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| index | [uint32](#uint32) |  |  |
| mtu | [uint32](#uint32) |  |  |
| name | [string](#string) |  |  |
| hardwareaddr | [string](#string) |  |  |
| flags | [InterfaceFlags](#network.InterfaceFlags) |  |  |
| ipaddress | [string](#string) | repeated |  |






<a name="network.Interfaces"></a>

### Interfaces



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| interfaces | [Interface](#network.Interface) | repeated |  |






<a name="network.InterfacesResponse"></a>

### InterfacesResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Interfaces](#network.Interfaces) | repeated |  |






<a name="network.Route"></a>

### Route
The messages message containing a route.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| interface | [string](#string) |  | Interface is the interface over which traffic to this destination should be sent |
| destination | [string](#string) |  | Destination is the network prefix CIDR which this route provides |
| gateway | [string](#string) |  | Gateway is the gateway address to which traffic to this destination should be sent |
| metric | [uint32](#uint32) |  | Metric is the priority of the route, where lower metrics have higher priorities |
| scope | [uint32](#uint32) |  | Scope desribes the scope of this route |
| source | [string](#string) |  | Source is the source prefix CIDR for the route, if one is defined |
| family | [AddressFamily](#network.AddressFamily) |  | Family is the address family of the route. Currently, the only options are AF_INET (IPV4) and AF_INET6 (IPV6). |
| protocol | [RouteProtocol](#network.RouteProtocol) |  | Protocol is the protocol by which this route came to be in place |
| flags | [uint32](#uint32) |  | Flags indicate any special flags on the route |






<a name="network.Routes"></a>

### Routes



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [common.Metadata](#common.Metadata) |  |  |
| routes | [Route](#network.Route) | repeated |  |






<a name="network.RoutesResponse"></a>

### RoutesResponse
The messages message containing the routes.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Routes](#network.Routes) | repeated |  |





 


<a name="network.AddressFamily"></a>

### AddressFamily


| Name | Number | Description |
| ---- | ------ | ----------- |
| AF_UNSPEC | 0 |  |
| AF_INET | 2 |  |
| IPV4 | 2 |  |
| AF_INET6 | 10 |  |
| IPV6 | 10 |  |



<a name="network.InterfaceFlags"></a>

### InterfaceFlags


| Name | Number | Description |
| ---- | ------ | ----------- |
| FLAG_UNKNOWN | 0 |  |
| FLAG_UP | 1 |  |
| FLAG_BROADCAST | 2 |  |
| FLAG_LOOPBACK | 3 |  |
| FLAG_POINT_TO_POINT | 4 |  |
| FLAG_MULTICAST | 5 |  |



<a name="network.RouteProtocol"></a>

### RouteProtocol


| Name | Number | Description |
| ---- | ------ | ----------- |
| RTPROT_UNSPEC | 0 |  |
| RTPROT_REDIRECT | 1 | Route installed by ICMP redirects |
| RTPROT_KERNEL | 2 | Route installed by kernel |
| RTPROT_BOOT | 3 | Route installed during boot |
| RTPROT_STATIC | 4 | Route installed by administrator |
| RTPROT_GATED | 8 | Route installed by gated |
| RTPROT_RA | 9 | Route installed by router advertisement |
| RTPROT_MRT | 10 | Route installed by Merit MRT |
| RTPROT_ZEBRA | 11 | Route installed by Zebra/Quagga |
| RTPROT_BIRD | 12 | Route installed by Bird |
| RTPROT_DNROUTED | 13 | Route installed by DECnet routing daemon |
| RTPROT_XORP | 14 | Route installed by XORP |
| RTPROT_NTK | 15 | Route installed by Netsukuku |
| RTPROT_DHCP | 16 | Route installed by DHCP |
| RTPROT_MROUTED | 17 | Route installed by Multicast daemon |
| RTPROT_BABEL | 42 | Route installed by Babel daemon |


 

 


<a name="network.NetworkService"></a>

### NetworkService
The network service definition.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| Routes | [.google.protobuf.Empty](#google.protobuf.Empty) | [RoutesResponse](#network.RoutesResponse) |  |
| Interfaces | [.google.protobuf.Empty](#google.protobuf.Empty) | [InterfacesResponse](#network.InterfacesResponse) |  |

 



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

