// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

func TestMachineTypeProtobufMarshal(t *testing.T) {
	r := config.NewMachineType()
	r.SetMachineType(machine.TypeWorker)

	protoR, err := protobuf.FromResource(r)
	require.NoError(t, err)

	marshaled, err := protoR.Marshal()
	require.NoError(t, err)

	protoR, err = protobuf.Unmarshal(marshaled)
	require.NoError(t, err)

	r2, err := protobuf.UnmarshalResource(protoR)
	require.NoError(t, err)

	require.True(t, resource.Equal(r, r2))
}
