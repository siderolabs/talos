// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package metal_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/metal"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

//nolint:gocyclo
func TestNetworkConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	p := &metal.Metal{}

	ch := make(chan *runtime.PlatformNetworkConfig, 1)

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	uuid := hardware.NewSystemInformation(hardware.SystemInformationID)
	uuid.TypedSpec().UUID = "0123-4567-89ab-cdef"
	require.NoError(t, st.Create(ctx, uuid))

	errCh := make(chan error)

	go func() {
		errCh <- p.NetworkConfiguration(ctx, st, ch)
	}()

	// platform might see updates coming in different order, so we need to wait a bit for the final state
outerLoop:
	for {
		select {
		case <-ctx.Done():
			require.FailNow(t, "timed out waiting for network config")
		case cfg := <-ch:
			assert.Equal(t, constants.PlatformMetal, cfg.Metadata.Platform)

			if cfg.Metadata.InstanceID == "" {
				continue
			}

			assert.Equal(t, uuid.TypedSpec().UUID, cfg.Metadata.InstanceID)

			break outerLoop
		}
	}

	metaKey := runtimeres.NewMetaKey(runtimeres.NamespaceName, runtimeres.MetaKeyTagToID(meta.MetalNetworkPlatformConfig))
	metaKey.TypedSpec().Value = `{"externalIPs": ["1.2.3.4"]}`
	require.NoError(t, st.Create(ctx, metaKey))

	// platform might see updates coming in different order, so we need to wait a bit for the final state
outerLoop2:
	for {
		select {
		case <-ctx.Done():
			require.FailNow(t, "timed out waiting for network config")
		case cfg := <-ch:
			assert.Equal(t, constants.PlatformMetal, cfg.Metadata.Platform)
			assert.Equal(t, uuid.TypedSpec().UUID, cfg.Metadata.InstanceID)

			if len(cfg.ExternalIPs) == 0 {
				continue
			}

			assert.Equal(t, "[1.2.3.4]", fmt.Sprintf("%v", cfg.ExternalIPs))

			break outerLoop2
		}
	}

	metaKey.TypedSpec().Value = `{"hostnames": [{"hostname": "talos", "domainname": "fqdn", "layer": "platform"}]}`
	require.NoError(t, st.Update(ctx, metaKey))

	select {
	case <-ctx.Done():
		require.FailNow(t, "timed out waiting for network config")
	case cfg := <-ch:
		assert.Equal(t, constants.PlatformMetal, cfg.Metadata.Platform)
		assert.Equal(t, uuid.TypedSpec().UUID, cfg.Metadata.InstanceID)

		assert.Equal(t, "[]", fmt.Sprintf("%v", cfg.ExternalIPs))
		assert.Equal(t, "[{talos fqdn platform}]", fmt.Sprintf("%v", cfg.Hostnames))
	}

	require.NoError(t, st.Destroy(ctx, metaKey.Metadata()))

	select {
	case <-ctx.Done():
		require.FailNow(t, "timed out waiting for network config")
	case cfg := <-ch:
		assert.Equal(t, constants.PlatformMetal, cfg.Metadata.Platform)
		assert.Equal(t, uuid.TypedSpec().UUID, cfg.Metadata.InstanceID)

		assert.Equal(t, "[]", fmt.Sprintf("%v", cfg.ExternalIPs))
		assert.Equal(t, "[]", fmt.Sprintf("%v", cfg.Hostnames))
	}

	cancel()
	require.ErrorIs(t, <-errCh, context.Canceled)
}
