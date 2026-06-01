// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package diagnostics_test

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/diagnostics"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func TestAddressOverlapCheck(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)

	for _, test := range []struct {
		name string

		setup func(t *testing.T, ctx context.Context, st state.State)

		expectedWarning *runtime.DiagnosticSpec
	}{
		{
			name: "no addresses",

			setup: func(t *testing.T, ctx context.Context, st state.State) {},
		},
		{
			name: "no overlap",

			setup: func(t *testing.T, ctx context.Context, st state.State) {
				hostAddresses := network.NewNodeAddress(network.NamespaceName, network.NodeAddressRoutedID)
				hostAddresses.TypedSpec().Addresses = []netip.Prefix{netip.MustParsePrefix("10.0.0.1/8"), netip.MustParsePrefix("10.244.1.3/32")}
				require.NoError(t, st.Create(ctx, hostAddresses))

				hostMinusK8s := network.NewNodeAddress(network.NamespaceName, network.FilteredNodeAddressID(network.NodeAddressRoutedID, k8s.NodeAddressFilterNoK8s))
				hostMinusK8s.TypedSpec().Addresses = []netip.Prefix{netip.MustParsePrefix("10.0.0.1/8")}
				require.NoError(t, st.Create(ctx, hostMinusK8s))
			},
		},
		{
			name: "with overlap",

			setup: func(t *testing.T, ctx context.Context, st state.State) {
				hostAddresses := network.NewNodeAddress(network.NamespaceName, network.NodeAddressRoutedID)
				hostAddresses.TypedSpec().Addresses = []netip.Prefix{netip.MustParsePrefix("10.244.3.4/24"), netip.MustParsePrefix("10.244.1.3/32")}
				require.NoError(t, st.Create(ctx, hostAddresses))

				hostMinusK8s := network.NewNodeAddress(network.NamespaceName, network.FilteredNodeAddressID(network.NodeAddressRoutedID, k8s.NodeAddressFilterNoK8s))
				hostMinusK8s.TypedSpec().Addresses = []netip.Prefix{}
				require.NoError(t, st.Create(ctx, hostMinusK8s))
			},

			expectedWarning: &runtime.DiagnosticSpec{
				Message: "host and Kubernetes pod/service CIDR addresses overlap",
				Details: []string{
					"host routed addresses: [\"10.244.3.4/24\" \"10.244.1.3/32\"]",
					"Kubernetes pod CIDRs: [\"10.244.0.0/16\"]", "Kubernetes service CIDRs: [\"10.96.0.0/12\"]",
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			logger := zaptest.NewLogger(t)
			st := state.WrapCore(namespaced.NewState(inmem.Build))

			in, err := generate.NewInput("test-cluster", "https://localhost", constants.DefaultKubernetesVersion)
			require.NoError(t, err)

			cfg, err := in.Config(machine.TypeWorker)
			require.NoError(t, err)

			cfgResource := config.NewMachineConfig(cfg)
			require.NoError(t, st.Create(ctx, cfgResource))

			test.setup(t, ctx, st)

			spec, err := diagnostics.AddressOverlapCheck(ctx, st, logger)
			require.NoError(t, err)

			if test.expectedWarning == nil {
				require.Nil(t, spec)
			} else {
				require.Equal(t, test.expectedWarning, spec)
			}
		})
	}
}
