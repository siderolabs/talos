// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logind_test

import (
	"bufio"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
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

	broker, err := logind.NewBroker(t.Context(), socketPathService, socketPathClient, selfCgroup(t))
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

func TestIntegrationNoKubelet(t *testing.T) {
	// test same flow as TestIntegration, but this time without kubelet connection
	// to ensure there is no wait for the lock that was never required
	dir := t.TempDir()

	socketPathService := filepath.Join(dir, "system_bus_service")
	socketPathClient := filepath.Join(dir, "system_bus_client")

	broker, err := logind.NewBroker(t.Context(), socketPathService, socketPathClient, selfCgroup(t))
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

	t.Log("emitting shutdown signal")

	require.NoError(t, serviceConn.EmitShutdown())

	t.Log("waiting for inhibit lock release")

	assert.NoError(t, serviceConn.WaitLockRelease(ctx))

	assert.NoError(t, serviceConn.Close())
	assert.NoError(t, broker.Close())

	cancel()

	assert.NoError(t, <-errCh)
}

func TestIntegrationStuckClients(t *testing.T) {
	dir := t.TempDir()

	socketPathService := filepath.Join(dir, "system_bus_service")
	socketPathClient := filepath.Join(dir, "system_bus_client")

	broker, err := logind.NewBroker(t.Context(), socketPathService, socketPathClient, selfCgroup(t))
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

	t.Log("connecting a client which never finishes auth")

	stuckConn, err := (&net.Dialer{}).DialContext(t.Context(), "unix", socketPathClient)
	require.NoError(t, err)

	defer stuckConn.Close() //nolint:errcheck

	_, err = stuckConn.Write([]byte("\x00AUTH\r\n"))
	require.NoError(t, err)

	reply, err := bufio.NewReader(stuckConn).ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, "REJECTED EXTERNAL\r\n", reply)

	t.Log("connecting a client which never disconnects")

	staleConn, err := NewDBusCon("unix:path=" + socketPathClient)
	require.NoError(t, err)

	defer staleConn.Close() //nolint:errcheck

	t.Log("connecting kubelet")

	kubeletConn, err := NewDBusCon("unix:path=" + socketPathClient)
	require.NoError(t, err)

	defer kubeletConn.Close() //nolint:errcheck

	d, err := kubeletConn.CurrentInhibitDelay()
	require.NoError(t, err)

	assert.Equal(t, 40*constants.KubeletShutdownGracePeriod, d)

	_, err = staleConn.CurrentInhibitDelay()
	assert.Error(t, err)

	assert.NoError(t, stuckConn.Close())
	assert.NoError(t, serviceConn.Close())
	assert.NoError(t, kubeletConn.Close())
	assert.NoError(t, broker.Close())

	cancel()

	assert.NoError(t, <-errCh)
}

func TestIntegrationOtherCgroup(t *testing.T) {
	dir := t.TempDir()

	socketPathService := filepath.Join(dir, "system_bus_service")
	socketPathClient := filepath.Join(dir, "system_bus_client")

	broker, err := logind.NewBroker(t.Context(), socketPathService, socketPathClient, selfCgroup(t)+"/kubelet")
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

	t.Log("connecting a client outside of the kubelet cgroup")

	_, err = NewDBusCon("unix:path=" + socketPathClient)
	require.Error(t, err)

	assert.NoError(t, serviceConn.Close())
	assert.NoError(t, broker.Close())

	cancel()

	assert.NoError(t, <-errCh)
}

// selfCgroup returns the cgroup of the test process on the unified hierarchy.
func selfCgroup(t *testing.T) string {
	t.Helper()

	contents, err := os.ReadFile("/proc/self/cgroup")
	require.NoError(t, err)

	path, ok := strings.CutPrefix(strings.TrimSpace(string(contents)), "0::")
	require.True(t, ok, "not on the unified hierarchy: %q", contents)

	return path
}
