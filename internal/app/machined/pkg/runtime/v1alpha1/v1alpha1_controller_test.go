// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage
package v1alpha1

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	v1alpha1server "github.com/siderolabs/talos/internal/app/machined/internal/server/v1alpha1"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/logging"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
)

type mockSequencer struct {
	callsMu sync.Mutex
	calls   map[runtime.Sequence]int

	phases map[runtime.Sequence]PhaseList
}

func (m *mockSequencer) Boot(r runtime.Runtime) []runtime.Phase {
	return m.phases[runtime.SequenceBoot]
}

func (m *mockSequencer) Initialize(r runtime.Runtime) []runtime.Phase {
	return m.phases[runtime.SequenceInitialize]
}

func (m *mockSequencer) Install(r runtime.Runtime) []runtime.Phase {
	return m.phases[runtime.SequenceInstall]
}

func (m *mockSequencer) Reboot(r runtime.Runtime, _ *machine.RebootRequest) []runtime.Phase {
	return m.phases[runtime.SequenceReboot]
}

func (m *mockSequencer) Reset(r runtime.Runtime, opts runtime.ResetOptions) []runtime.Phase {
	return m.phases[runtime.SequenceReset]
}

func (m *mockSequencer) Shutdown(r runtime.Runtime, req *machine.ShutdownRequest) []runtime.Phase {
	return m.phases[runtime.SequenceShutdown]
}

func (m *mockSequencer) StageUpgrade(r runtime.Runtime, req *machine.UpgradeRequest) []runtime.Phase {
	return m.phases[runtime.SequenceStageUpgrade]
}

func (m *mockSequencer) MaintenanceUpgrade(r runtime.Runtime, req *machine.UpgradeRequest) []runtime.Phase {
	return m.phases[runtime.SequenceMaintenanceUpgrade]
}

func (m *mockSequencer) Upgrade(r runtime.Runtime, req *machine.UpgradeRequest) []runtime.Phase {
	return m.phases[runtime.SequenceUpgrade]
}

func (m *mockSequencer) EmergencyVolumeCleanup(r runtime.Runtime) []runtime.Phase {
	return m.phases[runtime.SequenceEmergencyVolumeCleanup]
}

func (m *mockSequencer) trackCall(name string, doneCh chan struct{}) func(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(seq runtime.Sequence, data any) (runtime.TaskExecutionFunc, string) {
		return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
			if doneCh != nil {
				defer func() {
					select {
					case doneCh <- struct{}{}:
					case <-time.After(time.Second):
					}
				}()
			}

			m.callsMu.Lock()
			defer m.callsMu.Unlock()

			m.calls[seq]++

			return nil
		}, name
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name        string
		from        runtime.Sequence
		to          runtime.Sequence
		expectError error
		dataFrom    any
		dataTo      any
	}{
		{
			name:        "reboot should take over boot",
			from:        runtime.SequenceBoot,
			to:          runtime.SequenceReboot,
			expectError: context.Canceled,
		},
		{
			name:        "reset should take over boot",
			from:        runtime.SequenceBoot,
			to:          runtime.SequenceReset,
			expectError: context.Canceled,
			dataTo:      &v1alpha1server.ResetOptions{},
		},
		{
			name:        "upgrade should take over boot",
			from:        runtime.SequenceBoot,
			to:          runtime.SequenceUpgrade,
			expectError: context.Canceled,
			dataTo:      &machine.UpgradeRequest{},
		},
		{
			name:        "boot should not take over reboot",
			from:        runtime.SequenceReboot,
			to:          runtime.SequenceBoot,
			expectError: runtime.ErrLocked,
		},
		{
			name:        "reset should not take over upgrade",
			from:        runtime.SequenceUpgrade,
			to:          runtime.SequenceReset,
			expectError: runtime.ErrLocked,
			dataFrom:    &machine.UpgradeRequest{},
			dataTo:      &v1alpha1server.ResetOptions{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			assert := assert.New(t)

			e := NewEvents(1000, 10)

			t.Setenv("PLATFORM", "container")

			s, err := NewState()
			require.NoError(err)

			sequencer := &mockSequencer{
				calls:  map[runtime.Sequence]int{},
				phases: map[runtime.Sequence]PhaseList{},
			}

			var (
				eg     errgroup.Group
				doneCh = make(chan struct{})
			)

			sequencer.phases[tt.from] = sequencer.phases[tt.from].
				Append(tt.from.String(), sequencer.trackCall(tt.from.String(), doneCh)).
				Append("wait", wait)

			sequencer.phases[tt.to] = sequencer.phases[tt.to].Append(tt.to.String(), sequencer.trackCall(tt.to.String(), nil))

			l := logging.NewCircularBufferLoggingManager(log.New(os.Stdout, "machined fallback logger: ", log.Flags()))

			r := NewRuntime(s, e, l)

			controller := Controller{
				r:            r,
				s:            sequencer,
				priorityLock: NewPriorityLock[runtime.Sequence](),
			}

			ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond*200)
			defer cancel()

			eg.Go(func() error {
				t.Logf("starting %s sequence", tt.from.String())

				seqErr := controller.Run(ctx, tt.from, tt.dataFrom)

				t.Logf("sequence %s finished with error: %v", tt.from.String(), seqErr)

				return seqErr
			})

			eg.Go(func() error {
				select {
				case <-doneCh:
				case <-time.After(time.Second):
					return fmt.Errorf("timed out waiting for %s sequence to start", tt.from.String())
				}

				t.Logf("starting %s sequence", tt.to.String())

				seqErr := controller.Run(ctx, tt.to, tt.dataTo)

				t.Logf("sequence %s finished with error: %v", tt.to.String(), seqErr)

				return seqErr
			})

			require.ErrorIs(eg.Wait(), tt.expectError)

			if errors.Is(tt.expectError, runtime.ErrLocked) {
				return
			}

			sequencer.callsMu.Lock()
			defer sequencer.callsMu.Unlock()

			assert.Equal(1, sequencer.calls[tt.to])
		})
	}
}

func TestListenForSignalsCtrlAltDelete(t *testing.T) {
	securityProfile := func(ignoreCtrlAltDelete *bool) []config.Document {
		doc := runtimecfg.NewSecurityProfileConfigV1Alpha1()
		doc.IgnoreCtrlAltDeleteEnabled = ignoreCtrlAltDelete

		return []config.Document{doc}
	}

	tests := []struct {
		name       string
		configured bool
		documents  []config.Document
		signals    []os.Signal

		expectedReboots   int
		expectedShutdowns int
		expectedIgnored   int
	}{
		{
			name:            "no config",
			signals:         []os.Signal{syscall.SIGINT},
			expectedReboots: 1,
		},
		{
			name:            "no security profile",
			configured:      true,
			signals:         []os.Signal{syscall.SIGINT},
			expectedReboots: 1,
		},
		{
			name:            "not set",
			configured:      true,
			documents:       securityProfile(nil),
			signals:         []os.Signal{syscall.SIGINT},
			expectedReboots: 1,
		},
		{
			name:            "disabled",
			configured:      true,
			documents:       securityProfile(new(false)),
			signals:         []os.Signal{syscall.SIGINT},
			expectedReboots: 1,
		},
		{
			name:              "enabled",
			configured:        true,
			documents:         securityProfile(new(true)),
			signals:           []os.Signal{syscall.SIGINT, syscall.SIGINT, syscall.SIGTERM},
			expectedShutdowns: 1,
			expectedIgnored:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PLATFORM", "container")

			prevLogOutput := log.Writer()

			t.Cleanup(func() { log.SetOutput(prevLogOutput) })

			logOutput := &bytes.Buffer{}
			log.SetOutput(logOutput)

			s, err := NewState()
			require.NoError(t, err)

			sequencer := &mockSequencer{
				calls:  map[runtime.Sequence]int{},
				phases: map[runtime.Sequence]PhaseList{},
			}

			for _, seq := range []runtime.Sequence{runtime.SequenceReboot, runtime.SequenceShutdown} {
				sequencer.phases[seq] = sequencer.phases[seq].Append(seq.String(), sequencer.trackCall(seq.String(), nil))
			}

			l := logging.NewCircularBufferLoggingManager(log.New(os.Stdout, "machined fallback logger: ", log.Flags()))

			controller := Controller{
				r:            NewRuntime(s, NewEvents(1000, 10), l),
				s:            sequencer,
				priorityLock: NewPriorityLock[runtime.Sequence](),
			}

			if tt.configured {
				cfg, err := container.New(tt.documents...)
				require.NoError(t, err)

				require.NoError(t, controller.r.SetConfig(cfg))
			}

			sigs := make(chan os.Signal, len(tt.signals))

			for _, sig := range tt.signals {
				sigs <- sig
			}

			// listenForSignals ignores the passed signals on return, so pass one the test doesn't care about
			t.Cleanup(func() { signal.Reset(syscall.SIGUSR2) })

			controller.listenForSignals(t.Context(), sigs, []os.Signal{syscall.SIGUSR2})

			sequencer.callsMu.Lock()
			defer sequencer.callsMu.Unlock()

			assert.Equal(t, tt.expectedReboots, sequencer.calls[runtime.SequenceReboot])
			assert.Equal(t, tt.expectedShutdowns, sequencer.calls[runtime.SequenceShutdown])
			assert.Equal(t, tt.expectedIgnored, strings.Count(logOutput.String(), "Ctrl-Alt-Delete ignored as per SecurityProfileConfig"))
			assert.Empty(t, sigs)
		})
	}
}

func wait(seq runtime.Sequence, data any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second * 1):
		}

		return nil
	}, "wait"
}
