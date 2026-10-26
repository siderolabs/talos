// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos //nolint:testpackage // to test patchFn

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
)

type patchMachineClient struct {
	machine.MachineServiceClient
	calls int
	err   error
}

func (c *patchMachineClient) ApplyConfiguration(context.Context, *machine.ApplyConfigurationRequest, ...grpc.CallOption) (*machine.ApplyConfigurationResponse, error) {
	c.calls++

	if c.err != nil {
		return nil, c.err
	}

	return &machine.ApplyConfigurationResponse{}, nil
}

func TestPatchSkipsUnchangedConfig(t *testing.T) {
	for _, test := range []struct {
		name  string
		patch string
		calls int
		err   error
	}{
		{
			name:  "unchanged",
			patch: `{"machine":{"type":"worker"}}`,
		},
		{
			name:  "changed",
			patch: `{"machine":{"type":"controlplane"}}`,
			calls: 1,
		},
		{
			name:  "apply error",
			patch: `{"machine":{"type":"controlplane"}}`,
			calls: 1,
			err:   status.Error(codes.FailedPrecondition, "another apply configuration is already in progress"),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := configloader.NewFromBytes([]byte("version: v1alpha1\nmachine:\n    type: worker\n"))
			require.NoError(t, err)

			data, err := cfg.EncodeBytes()
			require.NoError(t, err)

			cfg, err = configloader.NewFromBytes(data)
			require.NoError(t, err)

			patches, err := configpatcher.LoadPatches([]string{test.patch})
			require.NoError(t, err)

			machineClient := &patchMachineClient{err: test.err}
			c := &client.Client{MachineClient: machineClient}

			err = patchFn(patches)(t.Context(), c, "127.0.0.1", configres.NewMachineConfig(cfg))
			require.Equal(t, test.err, err)
			require.Equal(t, test.calls, machineClient.calls)
		})
	}
}
