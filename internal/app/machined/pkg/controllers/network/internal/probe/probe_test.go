// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package probe_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/probe"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestProbeHTTP(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	p := probe.Runner{
		ID: "test",
		Spec: network.ProbeSpecSpec{
			Interval: 10 * time.Millisecond,
			TCP: network.TCPProbeSpec{
				Endpoint: u.Host,
				Timeout:  time.Second,
			},
		},
	}

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)

	notifyCh := make(chan probe.Notification)

	p.Start(ctx, notifyCh, zaptest.NewLogger(t))
	t.Cleanup(p.Stop)

	// probe should always succeed
	for range 3 {
		assert.Equal(t, probe.Notification{
			ID: "test",
			Status: network.ProbeStatusSpec{
				Success: true,
			},
		}, <-notifyCh)
	}

	// stop the test server, probe should fail
	server.Close()

	for {
		notification := <-notifyCh

		if notification.Status.Success {
			continue
		}

		assert.Equal(t, "test", notification.ID)
		assert.False(t, notification.Status.Success)
		assert.Contains(t, notification.Status.LastError, "connection refused")

		break
	}
}

func TestProbeConsecutiveFailures(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	mockClock := clock.NewMock()

	p := probe.Runner{
		ID: "consecutive-failures",
		Spec: network.ProbeSpecSpec{
			Interval:         10 * time.Millisecond,
			FailureThreshold: 3,
			TCP: network.TCPProbeSpec{
				Endpoint: u.Host,
				Timeout:  time.Second,
			},
		},
		Clock: mockClock,
	}

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)

	notifyCh := make(chan probe.Notification)

	p.Start(ctx, notifyCh, zaptest.NewLogger(t))
	t.Cleanup(p.Stop)

	// first iteration should succeed
	assert.Equal(t, probe.Notification{
		ID: "consecutive-failures",
		Status: network.ProbeStatusSpec{
			Success: true,
		},
	}, <-notifyCh)

	// stop the test server, probe should fail
	server.Close()

	for range p.Spec.FailureThreshold - 1 {
		// probe should fail, but no notification should be sent yet (failure threshold not reached)
		mockClock.Add(p.Spec.Interval)

		select {
		case ev := <-notifyCh:
			require.Fail(t, "unexpected notification", "got: %v", ev)
		case <-time.After(100 * time.Millisecond):
		}
	}

	// advance clock to trigger another failure(s)
	mockClock.Add(p.Spec.Interval)

	notify := <-notifyCh
	assert.Equal(t, "consecutive-failures", notify.ID)
	assert.False(t, notify.Status.Success)
	assert.Contains(t, notify.Status.LastError, "connection refused")

	// advance clock to trigger another failure(s)
	mockClock.Add(p.Spec.Interval)

	notify = <-notifyCh
	assert.Equal(t, "consecutive-failures", notify.ID)
	assert.False(t, notify.Status.Success)
}
