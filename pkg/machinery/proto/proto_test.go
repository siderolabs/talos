// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package proto_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/siderolabs/protoenc"
	"github.com/stretchr/testify/require"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/cluster"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

func TestCheckMemberSpecDecodingResult(t *testing.T) {
	// Input:
	// 00000000  0a 2c 37 78 31 53 75 43  38 45 67 65 35 42 47 58  |.,7x1SuC8Ege5BGX|
	// 00000010  64 41 66 54 45 66 66 35  69 51 6e 6c 57 5a 4c 66  |dAfTEff5iQnlWZLf|
	// 00000020  76 39 68 31 4c 47 4d 78  41 32 70 59 6b 43 12 06  |v9h1LGMxA2pYkC..|
	// 00000030  0a 04 ac 14 00 02 12 12  0a 10 fd 50 8d 60 42 38  |...........P.`B8|
	// 00000040  63 02 f8 57 23 ff fe 21  d1 e0 1a 1c 74 61 6c 6f  |c..W#..!....talo|
	// 00000050  73 2d 64 65 66 61 75 6c  74 2d 63 6f 6e 74 72 6f  |s-default-contro|
	// 00000060  6c 70 6c 61 6e 65 2d 31  20 02 2a 0e 54 61 6c 6f  |lplane-1 .*.Talo|
	// 00000070  73 20 28 76 31 2e 30 2e  30 29                    |s (v1.0.0)|
	//
	// This test ensures that things we encode in 1.3 is compatible with 1.2.
	//
	//nolint:lll
	const encodedString = "0a2c3778315375433845676535424758644166544566663569516e6c575a4c66763968314c474d78413270596b4312060a04ac14000212120a10fd508d6042386302f85723fffe21d1e01a1c74616c6f732d64656661756c742d636f6e74726f6c706c616e652d3120022a0e54616c6f73202876312e302e3029"

	type T = cluster.MemberSpec

	encoded := must(hex.DecodeString(encodedString))(t)
	addresses := []netaddr.IP{netaddr.MustParseIP("172.20.0.2"), netaddr.MustParseIP("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0")}
	expected := T{
		NodeID:          "7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC",
		Addresses:       addresses,
		Hostname:        "talos-default-controlplane-1",
		MachineType:     machine.TypeControlPlane,
		OperatingSystem: "Talos (v1.0.0)",
	}

	var decoded T

	require.NoError(t, protoenc.Unmarshal(encoded, &decoded))

	require.Equal(t, expected, decoded)
}

func TestCheckVIPOperatorSpecDecodingResult(t *testing.T) {
	// Input:
	// 00000000  0a 06 0a 04 c0 a8 01 01  10 01 1a 09 0a 01 61 12  |..............a.|
	// 00000010  01 62 1a 01 63 22 07 08  03 10 04 1a 01 64        |.b..c".......d|
	//
	// This test ensures that things we encode in 1.3 is compatible with 1.2.
	const encodedString = "0a060a04c0a8010110011a090a01611201621a01632207080310041a0164"

	type T = network.VIPOperatorSpec

	encoded := must(hex.DecodeString(encodedString))(t)
	expected := T{
		IP:            netaddr.MustParseIP("192.168.1.1"),
		GratuitousARP: true,
		EquinixMetal: network.VIPEquinixMetalSpec{
			ProjectID: "a",
			DeviceID:  "b",
			APIToken:  "c",
		},
		HCloud: network.VIPHCloudSpec{
			DeviceID:  3,
			NetworkID: 4,
			APIToken:  "d",
		},
	}

	var decoded T

	require.NoError(t, protoenc.Unmarshal(encoded, &decoded))

	require.Equal(t, expected, decoded)
}

//nolint:lll
func ExampleMemberSpec_output() {
	addresses := []netaddr.IP{netaddr.MustParseIP("172.20.0.2"), netaddr.MustParseIP("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0")}
	spec := cluster.MemberSpec{
		NodeID:          "7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC",
		Addresses:       addresses,
		Hostname:        "talos-default-controlplane-1",
		MachineType:     machine.TypeControlPlane,
		OperatingSystem: "Talos (v1.0.0)",
	}

	result, err := protoenc.Marshal(&spec)
	if err != nil {
		panic(err)
	}

	fmt.Println(hex.Dump(result))
	fmt.Println(hex.EncodeToString(result))

	// Output:
	// 00000000  0a 2c 37 78 31 53 75 43  38 45 67 65 35 42 47 58  |.,7x1SuC8Ege5BGX|
	// 00000010  64 41 66 54 45 66 66 35  69 51 6e 6c 57 5a 4c 66  |dAfTEff5iQnlWZLf|
	// 00000020  76 39 68 31 4c 47 4d 78  41 32 70 59 6b 43 12 04  |v9h1LGMxA2pYkC..|
	// 00000030  ac 14 00 02 12 10 fd 50  8d 60 42 38 63 02 f8 57  |.......P.`B8c..W|
	// 00000040  23 ff fe 21 d1 e0 1a 1c  74 61 6c 6f 73 2d 64 65  |#..!....talos-de|
	// 00000050  66 61 75 6c 74 2d 63 6f  6e 74 72 6f 6c 70 6c 61  |fault-controlpla|
	// 00000060  6e 65 2d 31 20 02 2a 0e  54 61 6c 6f 73 20 28 76  |ne-1 .*.Talos (v|
	// 00000070  31 2e 30 2e 30 29                                 |1.0.0)|
	//
	// 0a2c3778315375433845676535424758644166544566663569516e6c575a4c66763968314c474d78413270596b431204ac1400021210fd508d6042386302f85723fffe21d1e01a1c74616c6f732d64656661756c742d636f6e74726f6c706c616e652d3120022a0e54616c6f73202876312e302e3029
}

func ExampleVIPOperatorSpec_output() {
	spec := network.VIPOperatorSpec{
		IP:            netaddr.MustParseIP("192.168.1.1"),
		GratuitousARP: true,
		EquinixMetal: network.VIPEquinixMetalSpec{
			ProjectID: "a",
			DeviceID:  "b",
			APIToken:  "c",
		},
		HCloud: network.VIPHCloudSpec{
			DeviceID:  3,
			NetworkID: 4,
			APIToken:  "d",
		},
	}

	result, err := protoenc.Marshal(&spec)
	if err != nil {
		panic(err)
	}

	fmt.Println(hex.Dump(result))
	fmt.Println(hex.EncodeToString(result))

	// Output:
	// 00000000  0a 04 c0 a8 01 01 10 01  1a 09 0a 01 61 12 01 62  |............a..b|
	// 00000010  1a 01 63 22 07 08 03 10  04 1a 01 64              |..c".......d|
	//
	// 0a04c0a8010110011a090a01611201621a01632207080310041a0164
}

func ExampleVLANSpec_output() {
	spec := network.VLANSpec{
		VID:      25,
		Protocol: nethelpers.VLANProtocol8021AD,
	}

	result, err := protoenc.Marshal(&spec)
	if err != nil {
		panic(err)
	}

	fmt.Println(hex.Dump(result))
	fmt.Println(hex.EncodeToString(result))

	// Output:
	// 00000000  0d 19 00 00 00 15 a8 88  00 00                    |..........|
	//
	// 0d1900000015a8880000
}

func must[T any](v T, err error) func(t *testing.T) T {
	return func(t *testing.T) T {
		if err != nil {
			t.Fatal(err)
		}

		return v
	}
}
