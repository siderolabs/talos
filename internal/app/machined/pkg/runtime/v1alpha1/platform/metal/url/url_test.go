// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package url_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/metal/url"
	"github.com/siderolabs/talos/internal/pkg/meta"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type setupFunc func(context.Context, *testing.T, state.State)

func TestPopulate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		url  string

		preSetup      []setupFunc
		parallelSetup []setupFunc

		expected string
	}{
		{
			name:     "no variables",
			url:      "https://example.com?foo=bar",
			expected: "https://example.com?foo=bar",
		},
		{
			name:     "legacy UUID",
			url:      "https://example.com?uuid=",
			expected: "https://example.com?uuid=0000-0000",
			preSetup: []setupFunc{
				createSysInfo("0000-0000", ""),
			},
		},
		{
			name:     "sys info",
			url:      "https://example.com?uuid=${uuid}&no=${serial}",
			expected: "https://example.com?no=12345&uuid=0000-0000",
			preSetup: []setupFunc{
				createSysInfo("0000-0000", "12345"),
			},
		},
		{
			name:     "multiple variables",
			url:      "https://example.com?uuid=${uuid}&mac=${mac}&hostname=${hostname}&code=${code}",
			expected: "https://example.com?code=top-secret&hostname=example-node&mac=12%3A34%3A56%3A78%3A90%3Aab&uuid=0000-0000",
			preSetup: []setupFunc{
				createSysInfo("0000-0000", "12345"),
				createMac("12:34:56:78:90:ab"),
				createHostname("example-node"),
				createCode("top-secret"),
			},
		},
		{
			name:     "mixed wait variables",
			url:      "https://example.com?uuid=${uuid}&mac=${mac}&hostname=${hostname}&code=${code}",
			expected: "https://example.com?code=top-secret&hostname=another-node&mac=12%3A34%3A56%3A78%3A90%3Aab&uuid=0000-1234",
			preSetup: []setupFunc{
				createSysInfo("0000-1234", "12345"),
				createMac("12:34:56:78:90:ab"),
				createHostname("example-node"),
			},
			parallelSetup: []setupFunc{
				sleep(time.Second),
				updateHostname("another-node"),
				sleep(time.Second),
				createCode("top-secret"),
			},
		},
	} {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			st := state.WrapCore(namespaced.NewState(inmem.Build))

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			for _, f := range test.preSetup {
				f(ctx, t, st)
			}

			errCh := make(chan error)

			var result string

			go func() {
				var e error

				result, e = url.Populate(ctx, test.url, st)
				errCh <- e
			}()

			for _, f := range test.parallelSetup {
				f(ctx, t, st)
			}

			err := <-errCh
			require.NoError(t, err)

			assert.Equal(t, test.expected, result)
		})
	}
}

func createSysInfo(uuid, serial string) setupFunc {
	return func(ctx context.Context, t *testing.T, st state.State) {
		sysInfo := hardware.NewSystemInformation(hardware.SystemInformationID)
		sysInfo.TypedSpec().UUID = uuid
		sysInfo.TypedSpec().SerialNumber = serial
		require.NoError(t, st.Create(ctx, sysInfo))
	}
}

func createMac(mac string) setupFunc {
	return func(ctx context.Context, t *testing.T, st state.State) {
		addr, err := net.ParseMAC(mac)
		require.NoError(t, err)

		hwAddr := network.NewHardwareAddr(network.NamespaceName, network.FirstHardwareAddr)
		hwAddr.TypedSpec().HardwareAddr = nethelpers.HardwareAddr(addr)
		require.NoError(t, st.Create(ctx, hwAddr))
	}
}

func createHostname(hostname string) setupFunc {
	return func(ctx context.Context, t *testing.T, st state.State) {
		hn := network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
		hn.TypedSpec().Hostname = hostname
		require.NoError(t, st.Create(ctx, hn))
	}
}

func updateHostname(hostname string) setupFunc {
	return func(ctx context.Context, t *testing.T, st state.State) {
		hn, err := safe.StateGet[*network.HostnameStatus](ctx, st, network.NewHostnameStatus(network.NamespaceName, network.HostnameID).Metadata())
		require.NoError(t, err)

		hn.TypedSpec().Hostname = hostname
		require.NoError(t, st.Update(ctx, hn))
	}
}

func createCode(code string) setupFunc {
	return func(ctx context.Context, t *testing.T, st state.State) {
		mk := runtime.NewMetaKey(runtime.NamespaceName, runtime.MetaKeyTagToID(meta.DownloadURLCode))
		mk.TypedSpec().Value = code
		require.NoError(t, st.Create(ctx, mk))
	}
}

func sleep(d time.Duration) setupFunc {
	return func(ctx context.Context, t *testing.T, st state.State) {
		time.Sleep(d)
	}
}
