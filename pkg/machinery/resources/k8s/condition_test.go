// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

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

	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

func TestCondition(t *testing.T) {
	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(ctxCancel)

	t.Parallel()

	for _, tt := range []struct {
		Name           string
		NodenameExists bool
		VersionMatches bool
		Succeeds       bool
	}{
		{
			Name:     "no nodename",
			Succeeds: false,
		},
		{
			Name:           "version mismatch",
			NodenameExists: true,
			VersionMatches: false,
			Succeeds:       false,
		},
		{
			Name:           "success",
			NodenameExists: true,
			VersionMatches: true,
			Succeeds:       true,
		},
	} {
		tt := tt

		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			state := state.WrapCore(namespaced.NewState(inmem.Build))

			hostnameStatus := network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
			hostnameStatus.TypedSpec().Hostname = "foo"

			require.NoError(t, state.Create(ctx, hostnameStatus))

			if tt.NodenameExists {
				nodename := k8s.NewNodename(k8s.NamespaceName, k8s.NodenameID)
				nodename.TypedSpec().Nodename = "foo"

				md := hostnameStatus.Metadata()

				if !tt.VersionMatches {
					md.BumpVersion()
				}

				nodename.TypedSpec().HostnameVersion = md.Version().String()

				require.NoError(t, state.Create(ctx, nodename))
			}

			err := k8s.NewNodenameReadyCondition(state).Wait(ctx)

			if tt.Succeeds {
				assert.NoError(t, err)
			} else {
				assert.True(t, errors.Is(err, context.DeadlineExceeded), "error is %v", err)
			}
		})
	}
}
