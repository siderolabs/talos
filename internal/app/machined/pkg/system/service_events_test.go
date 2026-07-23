// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system_test

import (
	"context"
	"testing"
	"time"

	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
)

func upOrFinished() []system.StateEvent {
	return []system.StateEvent{system.StateEventUp, system.StateEventFinished}
}

// waitForServiceState blocks until the service reports the expected state.
func waitForServiceState(t *testing.T, services *system.Singleton, id string, expected events.ServiceState) {
	t.Helper()

	require.NoError(t, retry.Constant(time.Minute, retry.WithUnits(10*time.Millisecond)).Retry(func() error {
		for _, svcrunner := range services.List() {
			svc := svcrunner.AsProto()

			if svc.Id != id {
				continue
			}

			if svc.State != expected.String() {
				return retry.ExpectedErrorf("service %q is %q, expected %q", id, svc.State, expected)
			}

			return nil
		}

		return retry.ExpectedErrorf("service %q is not registered", id)
	}))
}

// TestWaitForServiceAnyEventUp asserts that a multi-event wait is satisfied by the first
// of the events to happen - here, the service coming up.
func TestWaitForServiceAnyEventUp(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	services := system.NewServices(newRuntime(t))
	t.Cleanup(func() { services.Shutdown(context.Background()) })

	services.LoadAndStart(&MockService{name: "up-service"})

	require.NoError(t, system.WaitForServiceAnyEventWithInstance(services, upOrFinished(), "up-service").Wait(ctx))

	waitForServiceState(t, services, "up-service", events.StateRunning)
}

// TestWaitForServiceAnyEventFinished asserts that a multi-event wait is satisfied by a
// service which ran to completion, while a wait for 'up' alone is not: a finished
// service is not up.
func TestWaitForServiceAnyEventFinished(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	services := system.NewServices(newRuntime(t))
	t.Cleanup(func() { services.Shutdown(context.Background()) })

	services.LoadAndStart(&MockService{name: "oneshot-service", runner: MockFinishingRunner{}})

	waitForServiceState(t, services, "oneshot-service", events.StateFinished)

	// the runner never reports StateRunning, so 'finished' is the only event which can
	// satisfy this condition
	require.NoError(t, system.WaitForServiceAnyEventWithInstance(services, upOrFinished(), "oneshot-service").Wait(ctx))

	upCtx, upCancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer upCancel()

	assert.ErrorIs(t,
		system.WaitForServiceWithInstance(services, system.StateEventUp, "oneshot-service").Wait(upCtx),
		context.DeadlineExceeded)
}

// TestWaitForServiceAnyEventFinishedEdge is TestWaitForServiceAnyEventFinished with the
// wait established before the service finishes, so the condition is completed by the
// state transition notification rather than by the state check done on subscribe.
func TestWaitForServiceAnyEventFinishedEdge(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	cond := NewMockCondition("gate")

	services := system.NewServices(newRuntime(t))
	t.Cleanup(func() { services.Shutdown(context.Background()) })

	services.LoadAndStart(&MockService{name: "gated-service", condition: cond, runner: MockFinishingRunner{}})

	// hold the service in StateWaiting while the wait below subscribes
	waitForServiceState(t, services, "gated-service", events.StateWaiting)

	errCh := make(chan error, 1)

	go func() {
		errCh <- system.WaitForServiceAnyEventWithInstance(services, upOrFinished(), "gated-service").Wait(ctx)
	}()

	// give the goroutine above a chance to subscribe before the service finishes;
	// if it loses the race the condition is still satisfied, just via the state check
	// in Subscribe() instead of the notification
	time.Sleep(50 * time.Millisecond)

	select {
	case err := <-errCh:
		require.FailNowf(t, "condition completed early", "service is still waiting on its condition: %v", err)
	default:
	}

	close(cond.done)

	require.NoError(t, <-errCh)

	waitForServiceState(t, services, "gated-service", events.StateFinished)
}

func TestServiceConditionString(t *testing.T) {
	assert.Equal(t,
		`service "foo" to be "up"`,
		system.WaitForServiceWithInstance(nil, system.StateEventUp, "foo").String())

	assert.Equal(t,
		`service "foo" to be up/finished`,
		system.WaitForServiceAnyEventWithInstance(nil, upOrFinished(), "foo").String())
}
