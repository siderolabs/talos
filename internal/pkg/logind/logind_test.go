// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logind_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/logind"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestIntegration(t *testing.T) {
	dir := t.TempDir()

	socketPathService := filepath.Join(dir, "system_bus_service")
	socketPathClient := filepath.Join(dir, "system_bus_client")

	broker, err := logind.NewBroker(socketPathService, socketPathClient)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	errCh := make(chan error, 1)

	go func() {
		errCh <- broker.Run(ctx)
	}()

	serviceConn, err := logind.NewServiceMock(socketPathService)
	require.NoError(t, err)

	defer serviceConn.Close() //nolint:errcheck

	kubeletConn, err := NewDBusCon("unix:path=" + socketPathClient)
	require.NoError(t, err)

	defer kubeletConn.Close() //nolint:errcheck

	t.Log("ready to go")

	d, err := kubeletConn.CurrentInhibitDelay()
	require.NoError(t, err)

	assert.Equal(t, 40*constants.KubeletShutdownGracePeriod, d)

	t.Log("acquiring inhibit lock")

	l, err := kubeletConn.InhibitShutdown()
	require.NoError(t, err)

	t.Log("monitoring shutdown signal")

	ch, err := kubeletConn.MonitorShutdown()
	require.NoError(t, err)

	t.Log("emitting shutdown signal")

	require.NoError(t, serviceConn.EmitShutdown())

	assert.True(t, <-ch)

	t.Log("releasing inhibit lock")

	require.NoError(t, kubeletConn.ReleaseInhibitLock(l))

	t.Log("waiting for inhibit lock release")

	assert.NoError(t, serviceConn.WaitLockRelease(ctx))

	assert.NoError(t, serviceConn.Close())
	assert.NoError(t, kubeletConn.Close())
	assert.NoError(t, broker.Close())

	cancel()

	assert.NoError(t, <-errCh)
}
