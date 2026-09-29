// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build sidero.debug

package ctrltrace_test

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	osruntime "github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/pkg/ctrltrace"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

const testNamespace = "default"

func TestTraceRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.jsonl")

	tracer := ctrltrace.New(ctrltrace.Options{
		Path:         path,
		TalosVersion: "v0.0.0-test",
		WithSpecs:    true,
	})

	require.True(t, ctrltrace.Enabled())

	st := state.WrapCore(ctrltrace.WrapState(namespaced.NewState(inmem.Build)))

	ctx := ctrltrace.WithCaller(t.Context(), "test.Controller")

	res := conformance.NewPathResource(testNamespace, "/etc/hosts")

	require.NoError(t, st.Create(ctx, res))

	_, err := safe.StateGetResource(ctx, st, res)
	require.NoError(t, err)

	require.NoError(t, st.Destroy(ctx, res.Metadata()))

	require.NoError(t, tracer.Close())

	events := readTrace(t, path)

	require.NotEmpty(t, events)

	assert.Equal(t, "meta", events[0].K)
	assert.Equal(t, "start", events[0].Op)

	byOp := map[string]ctrltrace.Event{}

	for _, ev := range events {
		if ev.K == "op" {
			byOp[ev.Op] = ev
		}
	}

	create, ok := byOp["create"]
	require.True(t, ok, "create event not recorded")
	assert.Equal(t, "test.Controller", create.Ctrl)
	assert.Equal(t, testNamespace, create.NS)
	assert.Equal(t, conformance.PathResourceType, create.RT)
	assert.Equal(t, "/etc/hosts", create.ID)
	assert.NotEmpty(t, create.Spec)

	_, ok = byOp["get"]
	assert.True(t, ok, "get event not recorded")

	_, ok = byOp["destroy"]
	assert.True(t, ok, "destroy event not recorded")

	assert.Equal(t, "stop", events[len(events)-1].Op)
}

func TestControllerWrapperAttributesReads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.jsonl")

	tracer := ctrltrace.New(ctrltrace.Options{Path: path})

	st := state.WrapCore(ctrltrace.WrapState(namespaced.NewState(inmem.Build)))

	rt, err := osruntime.NewRuntime(st, zaptest.NewLogger(t))
	require.NoError(t, err)

	ctrl := &listController{listed: make(chan struct{}, 1)}

	require.NoError(t, rt.RegisterController(ctrltrace.WrapController(ctrl)))

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)

	go func() { errCh <- rt.Run(ctx) }()

	// trigger a reconcile
	require.NoError(t, st.Create(ctx, conformance.NewPathResource(testNamespace, "/tmp")))

	// wait for the controller to actually reconcile, not merely for the trace header
	select {
	case <-ctrl.listed:
	case <-ctx.Done():
		t.Fatal("controller never reconciled")
	}

	cancel()
	<-errCh

	require.NoError(t, tracer.Close())

	events := readTrace(t, path)

	var sawStart, sawAttributedList bool

	for _, ev := range events {
		if ev.K == "ctrl" && ev.Op == "start" && ev.Ctrl == "test.ListController" {
			sawStart = true
		}

		if ev.K == "op" && ev.Op == "list" && ev.Ctrl == "test.ListController" {
			sawAttributedList = true
		}
	}

	assert.True(t, sawStart, "controller start not recorded")
	assert.True(t, sawAttributedList, "controller list not attributed")
}

// listController lists its inputs on every reconcile event.
type listController struct {
	listed chan struct{}
}

func (listController) Name() string { return "test.ListController" }

func (listController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: testNamespace,
			Type:      conformance.PathResourceType,
			Kind:      controller.InputWeak,
		},
	}
}

func (listController) Outputs() []controller.Output { return nil }

func (c *listController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if _, err := safe.ReaderListAll[*conformance.PathResource](ctx, r); err != nil {
			return err
		}

		select {
		case c.listed <- struct{}{}:
		default:
		}
	}
}

func readTrace(t *testing.T, path string) []ctrltrace.Event {
	t.Helper()

	f, err := os.Open(path)
	require.NoError(t, err)

	defer f.Close() //nolint:errcheck

	var events []ctrltrace.Event

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		var ev ctrltrace.Event

		require.NoError(t, json.Unmarshal(scanner.Bytes(), &ev))

		events = append(events, ev)
	}

	require.NoError(t, scanner.Err())

	return events
}

// fakeRuntime implements just enough of controller.Runtime for the restart test.
type fakeRuntime struct {
	controller.Runtime

	ch chan controller.ReconcileEvent
}

func (f *fakeRuntime) EventCh() <-chan controller.ReconcileEvent { return f.ch }

// recordingController captures what it is handed on every Run.
type recordingController struct {
	ctxs []context.Context //nolint:containedctx
	rts  []controller.Runtime
}

func (recordingController) Name() string                 { return "test.RecordingController" }
func (recordingController) Inputs() []controller.Input   { return nil }
func (recordingController) Outputs() []controller.Output { return nil }

func (c *recordingController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	c.ctxs = append(c.ctxs, ctx)
	c.rts = append(c.rts, r)

	// consume the event channel once, so the relay is started
	r.EventCh()

	return nil
}

// TestRestartKeepsContextIdentity guards the invariant that broke internal/pkg/dns.Manager:
// the controller runtime restarts a controller with the very same context and runtime values,
// and some controllers compare them by identity.  Re-tagging per Run would also leave a second
// event relay behind on every restart.
func TestRestartKeepsContextIdentity(t *testing.T) {
	tracer := ctrltrace.New(ctrltrace.Options{Path: filepath.Join(t.TempDir(), "trace.jsonl")})
	defer tracer.Close() //nolint:errcheck

	inner := &recordingController{}
	wrapped := ctrltrace.WrapController(inner)

	ctx := t.Context()
	rt := &fakeRuntime{ch: make(chan controller.ReconcileEvent)}

	for range 3 {
		require.NoError(t, wrapped.Run(ctx, rt, zaptest.NewLogger(t)))
	}

	require.Len(t, inner.ctxs, 3)

	assert.Same(t, inner.ctxs[0], inner.ctxs[1], "context identity must survive a restart")
	assert.Same(t, inner.ctxs[1], inner.ctxs[2], "context identity must survive a restart")
	assert.Same(t, inner.rts[0], inner.rts[1], "runtime identity must survive a restart")
	assert.Same(t, inner.rts[1], inner.rts[2], "runtime identity must survive a restart")

	assert.Equal(t, "test.RecordingController", ctrltrace.CallerFrom(inner.ctxs[0]))
	assert.NotSame(t, ctx, inner.ctxs[0], "the controller context carries the caller tag")
}

// the file must not be world-readable
func TestTraceFileIsNotWorldReadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.jsonl")

	tracer := ctrltrace.New(ctrltrace.Options{Path: path})
	require.NoError(t, tracer.Close())

	st, err := os.Stat(path)
	require.NoError(t, err)

	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
}

// TestSpecIsRecordedAsYAML checks a real Talos resource round-trips through the tracer as
// YAML keyed by its `yaml:` tags, rather than by Go field names — which is what makes the
// recorded spec match what `talosctl get -o yaml` shows.
func TestSpecIsRecordedAsYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.jsonl")

	tracer := ctrltrace.New(ctrltrace.Options{Path: path, WithSpecs: true})

	st := state.WrapCore(ctrltrace.WrapState(namespaced.NewState(inmem.Build)))
	ctx := ctrltrace.WithCaller(t.Context(), "block.DisksController")

	disk := block.NewDisk(block.NamespaceName, "nvme0n1")
	disk.TypedSpec().DevPath = "/dev/nvme0n1"
	disk.TypedSpec().Size = 512110190592
	disk.TypedSpec().PrettySize = "512 GB"
	disk.TypedSpec().Model = "SAMSUNG MZVLB512HBJQ"

	require.NoError(t, st.Create(ctx, disk))
	require.NoError(t, tracer.Close())

	var spec string

	for _, ev := range readTrace(t, path) {
		if ev.Op == "create" && ev.ID == "nvme0n1" {
			spec = ev.Spec
		}
	}

	require.NotEmpty(t, spec)

	assert.Contains(t, spec, "dev_path: /dev/nvme0n1")
	assert.Contains(t, spec, "size: 512110190592")
	assert.Contains(t, spec, "pretty_size: 512 GB")
	assert.Contains(t, spec, "model: SAMSUNG MZVLB512HBJQ")
	assert.NotContains(t, spec, "DevPath", "spec must use the yaml tags, not Go field names")
}
