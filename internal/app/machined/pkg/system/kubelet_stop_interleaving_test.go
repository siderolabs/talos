// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/pkg/conditions"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
)

type terminalRaceRuntime struct {
	runtime.Runtime
	stream runtime.EventStream
}

func (r terminalRaceRuntime) Events() runtime.EventStream { return r.stream }

type terminalRaceEvents struct {
	runtime.EventStream
	terminalEntered chan struct{}
	releaseTerminal chan struct{}
}

func (e terminalRaceEvents) Publish(_ context.Context, message proto.Message) {
	event, ok := message.(*machineapi.ServiceStateEvent)
	if ok && event.Action == machineapi.ServiceStateEvent_Action(events.StateFailed) {
		close(e.terminalEntered)
		<-e.releaseTerminal
	}
}

type terminalRaceService struct {
	TestService
	calls           atomic.Int32
	currentEntered  chan struct{}
	currentCanceled chan struct{}
	releaseCurrent  chan struct{}
}

func (*terminalRaceService) APIStartAllowed(runtime.Runtime) bool           { return true }
func (*terminalRaceService) APIRestartAllowed(runtime.Runtime) bool         { return true }
func (*terminalRaceService) Condition(runtime.Runtime) conditions.Condition { return nil }
func (s *terminalRaceService) PreFunc(context.Context, runtime.Runtime) error {
	if s.calls.Add(1) == 1 {
		return errors.New("old launch finishes")
	}

	return nil
}

func (s *terminalRaceService) Runner(runtime.Runtime) (runner.Runner, error) {
	return terminalRaceRunner{service: s}, nil
}

type terminalRaceRunner struct{ service *terminalRaceService }

func (terminalRaceRunner) String() string { return "controlled live kubelet" }
func (terminalRaceRunner) Open() error    { return nil }
func (terminalRaceRunner) Close() error   { return nil }
func (r terminalRaceRunner) Run(ctx context.Context, _ events.Recorder, onStart runner.OnStart) (runner.Status, error) {
	onStart(123)
	close(r.service.currentEntered)
	<-ctx.Done()
	close(r.service.currentCanceled)
	// A real container may still be reading shared files while graceful stop is pending.
	<-r.service.releaseCurrent

	return runner.Status{Started: true}, nil
}

type restartableTestService struct{ MockService }

func (*restartableTestService) APIStartAllowed(runtime.Runtime) bool   { return true }
func (*restartableTestService) APIRestartAllowed(runtime.Runtime) bool { return true }

func TestUnloadWaitsForTerminalPublication(t *testing.T) {
	baseRuntime := newRuntime(t)
	synctest.Test(t, func(t *testing.T) {
		stream := terminalRaceEvents{terminalEntered: make(chan struct{}), releaseTerminal: make(chan struct{})}
		manager := system.NewServices(terminalRaceRuntime{Runtime: baseRuntime, stream: stream})
		manager.Load(&MockService{name: "unload", preError: errors.New("failed")})
		require.NoError(t, manager.Start("unload"))
		<-stream.terminalEntered

		done := make(chan error, 1)
		go func() { done <- manager.Unload(t.Context(), "unload") }()

		synctest.Wait()

		select {
		case <-done:
			t.Error("Unload returned before the captured launch completed")
			close(stream.releaseTerminal)

			return
		default:
		}

		close(stream.releaseTerminal)
		require.NoError(t, <-done)

		_, _, err := manager.IsRunning("unload")
		require.Error(t, err)
		manager.Load(&MockService{name: "unload"})
		require.NoError(t, manager.Start("unload"))
		synctest.Wait()

		_, running, err := manager.IsRunning("unload")
		require.NoError(t, err)
		require.True(t, running)
		require.NoError(t, manager.Stop(t.Context(), "unload"))
	})
}

func TestStopLaunchEdgeCases(t *testing.T) {
	r := newRuntime(t)
	synctest.Test(t, func(t *testing.T) {
		manager := system.NewServices(r)
		manager.Load(&MockService{name: "never-started"}, &MockService{name: "failed", preError: errors.New("failed")})
		require.NoError(t, manager.Stop(t.Context(), "unknown", "never-started"))
		require.Error(t, manager.Start("unknown"))
		require.NoError(t, manager.Start("failed"))
		synctest.Wait()
		require.NoError(t, manager.Stop(t.Context(), "failed"))
		require.NoError(t, manager.Start("never-started"))
		synctest.Wait()

		_, running, err := manager.IsRunning("never-started")
		require.NoError(t, err)
		require.True(t, running, "Stop before Start must not leave a stale cancellation")
		require.NoError(t, manager.Stop(t.Context(), "never-started"))
	})
}

func TestBackToBackAPIRestarts(t *testing.T) {
	r := newRuntime(t)
	synctest.Test(t, func(t *testing.T) {
		manager := system.NewServices(r)
		manager.Load(&restartableTestService{name: "restartable"})

		for range 4 {
			require.NoError(t, manager.APIRestart(t.Context(), "restartable"))
			synctest.Wait()

			_, running, err := manager.IsRunning("restartable")
			require.NoError(t, err)
			require.True(t, running)
		}

		manager.DenyNewServices()

		done := make(chan struct{})

		go func() { manager.Shutdown(t.Context()); close(done) }()

		require.NoError(t, manager.Start("restartable"))
		<-done

		_, running, err := manager.IsRunning("restartable")
		require.NoError(t, err)
		require.False(t, running)
	})
}

// A delayed terminal notification from the previous launch must not acknowledge
// Stop of a later launch which is still cleaning up. The proposed kubelet
// retire/Stop/prepare protocol relies on this property before rewriting files.
func TestKubeletStopWaitsForCurrentLaunch(t *testing.T) {
	baseRuntime := newRuntime(t)

	synctest.Test(t, func(t *testing.T) {
		stream := terminalRaceEvents{terminalEntered: make(chan struct{}), releaseTerminal: make(chan struct{})}
		service := &terminalRaceService{currentEntered: make(chan struct{}), currentCanceled: make(chan struct{}), releaseCurrent: make(chan struct{})}
		manager := system.NewServices(terminalRaceRuntime{Runtime: baseRuntime, stream: stream})
		manager.Load(service)
		require.NoError(t, manager.Start("test-service"))
		<-stream.terminalEntered

		// API Start cannot reuse an ID until the old terminal publication completes.
		require.NoError(t, manager.APIStart(t.Context(), "test-service"))
		synctest.Wait()

		if !assert.EqualValues(t, 1, service.calls.Load(), "new launch overlapped old terminal publication") {
			close(stream.releaseTerminal)

			stopDone := make(chan error, 1)
			go func() { stopDone <- manager.Stop(t.Context(), "test-service") }()

			close(service.releaseCurrent)
			<-stopDone

			return
		}

		stopDone := make(chan error, 1)
		go func() { stopDone <- manager.Stop(t.Context(), "test-service") }()

		synctest.Wait()

		select {
		case <-stopDone:
			t.Error("Stop returned before terminal publication completed")
			close(stream.releaseTerminal)

			return
		default:
		}

		close(stream.releaseTerminal)
		require.NoError(t, <-stopDone)

		// A completed Stop permits an immediate new Start, not a stale-map no-op.
		require.NoError(t, manager.APIStart(t.Context(), "test-service"))
		<-service.currentEntered

		go func() { stopDone <- manager.Stop(t.Context(), "test-service") }()

		<-service.currentCanceled
		synctest.Wait()

		select {
		case <-stopDone:
			t.Error("Stop returned while the current process was still alive")
			close(service.releaseCurrent)

			return
		default:
		}

		close(service.releaseCurrent)
		require.NoError(t, <-stopDone)
		synctest.Wait()
	})
}
