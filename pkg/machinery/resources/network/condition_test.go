// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestCondition(t *testing.T) {
	ctx, ctxCancel := context.WithTimeout(t.Context(), time.Second)
	t.Cleanup(ctxCancel)

	t.Parallel()

	for _, tt := range []struct {
		Name     string
		Status   network.StatusSpec
		Checks   []network.StatusCheck
		Succeeds bool
	}{
		{
			Name:     "no checks",
			Succeeds: true,
		},
		{
			Name:     "timeout",
			Checks:   []network.StatusCheck{network.AddressReady, network.ConnectivityReady},
			Succeeds: false,
		},
		{
			Name: "partial",
			Status: network.StatusSpec{
				AddressReady: true,
			},
			Checks:   []network.StatusCheck{network.AddressReady, network.ConnectivityReady},
			Succeeds: false,
		},
		{
			Name: "okay",
			Status: network.StatusSpec{
				AddressReady:      true,
				ConnectivityReady: true,
				EtcFilesReady:     true,
			},
			Checks:   []network.StatusCheck{network.AddressReady, network.ConnectivityReady},
			Succeeds: true,
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			state := state.WrapCore(namespaced.NewState(inmem.Build))

			status := network.NewStatus(network.NamespaceName, network.StatusID)
			*status.TypedSpec() = tt.Status

			require.NoError(t, state.Create(ctx, status))

			err := network.NewReadyCondition(state, tt.Checks...).Wait(ctx)

			if tt.Succeeds {
				assert.NoError(t, err)
			} else {
				assert.True(t, errors.Is(err, context.DeadlineExceeded), "error is %v", err)
			}
		})
	}
}
