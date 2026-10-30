// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package debug //nolint:testpackage // to test the unexported instance lookup

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

const execContainer = "app"

func newExecState(t *testing.T, declared bool, phase *containers.ContainerInstancePhase, withSpec, tearingDown bool) state.State {
	t.Helper()

	st := state.WrapCore(namespaced.NewState(inmem.Build))
	ctx := t.Context()

	if declared {
		require.NoError(t, st.Create(ctx, containers.NewContainerSpec(containers.NamespaceName, execContainer)))
	}

	if phase == nil {
		return st
	}

	const generation = 3

	instanceID := containers.InstanceID(execContainer, generation)

	instanceStatus := containers.NewContainerInstanceStatus(containers.NamespaceName, instanceID)
	instanceStatus.Metadata().Labels().Set(containers.ContainerSpecIdLabel, execContainer)
	instanceStatus.TypedSpec().ContainerID = execContainer
	instanceStatus.TypedSpec().Generation = generation
	instanceStatus.TypedSpec().Phase = *phase

	require.NoError(t, st.Create(ctx, instanceStatus))

	if !withSpec {
		return st
	}

	instanceSpec := containers.NewContainerInstanceSpec(containers.NamespaceName, instanceID)
	instanceSpec.Metadata().Labels().Set(containers.ContainerSpecIdLabel, execContainer)
	instanceSpec.TypedSpec().ContainerID = execContainer
	instanceSpec.TypedSpec().Generation = generation

	require.NoError(t, st.Create(ctx, instanceSpec))

	if tearingDown {
		_, err := st.Teardown(ctx, instanceSpec.Metadata())
		require.NoError(t, err)
	}

	return st
}

func TestRunningInstance(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		declared    bool
		phase       *containers.ContainerInstancePhase
		withSpec    bool
		tearingDown bool

		expectedID   string
		expectedCode codes.Code
	}{
		{
			name:         "unknown container",
			expectedCode: codes.NotFound,
		},
		{
			name:         "no instance yet",
			declared:     true,
			expectedCode: codes.FailedPrecondition,
		},
		{
			name:         "instance terminated",
			declared:     true,
			phase:        new(containers.ContainerInstancePhaseTerminated),
			withSpec:     true,
			expectedCode: codes.FailedPrecondition,
		},
		{
			name:         "instance not started yet",
			declared:     true,
			phase:        new(containers.ContainerInstancePhaseCreated),
			withSpec:     true,
			expectedCode: codes.FailedPrecondition,
		},
		{
			name:         "instance being replaced",
			declared:     true,
			phase:        new(containers.ContainerInstancePhaseRunning),
			withSpec:     true,
			tearingDown:  true,
			expectedCode: codes.FailedPrecondition,
		},
		{
			name:         "instance spec gone",
			declared:     true,
			phase:        new(containers.ContainerInstancePhaseRunning),
			expectedCode: codes.FailedPrecondition,
		},
		{
			name:       "running",
			declared:   true,
			phase:      new(containers.ContainerInstancePhaseRunning),
			withSpec:   true,
			expectedID: "app-3",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			s := &Service{Resources: newExecState(t, test.declared, test.phase, test.withSpec, test.tearingDown)}

			instanceID, err := s.runningInstance(t.Context(), execContainer)

			if test.expectedCode != codes.OK {
				require.Error(t, err)
				assert.Equal(t, test.expectedCode, status.Code(err), "error: %v", err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expectedID, instanceID)
		})
	}
}

// fakeExecServer stands in for the server side of a ContainerExec stream.
type fakeExecServer struct {
	grpc.ServerStream

	ctx      context.Context //nolint:containedctx
	requests []*machine.DebugContainerExecRequest
}

func (f *fakeExecServer) Context() context.Context { return f.ctx }

func (f *fakeExecServer) Recv() (*machine.DebugContainerExecRequest, error) {
	if len(f.requests) == 0 {
		return nil, io.EOF
	}

	req := f.requests[0]
	f.requests = f.requests[1:]

	return req, nil
}

func (f *fakeExecServer) Send(*machine.DebugContainerExecResponse) error { return nil }

func TestContainerExecValidation(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name         string
		spec         *machine.DebugContainerExecRequestSpec
		expectedCode codes.Code
		expectedErr  string
	}{
		{
			name:         "no spec",
			expectedCode: codes.InvalidArgument,
			expectedErr:  "expected exec spec",
		},
		{
			name:         "no container",
			spec:         &machine.DebugContainerExecRequestSpec{Args: []string{"/bin/true"}},
			expectedCode: codes.InvalidArgument,
			expectedErr:  "container is required",
		},
		{
			name:         "no command",
			spec:         &machine.DebugContainerExecRequestSpec{Container: execContainer},
			expectedCode: codes.InvalidArgument,
			expectedErr:  "args are required",
		},
		{
			name:         "unknown container",
			spec:         &machine.DebugContainerExecRequestSpec{Container: execContainer, Args: []string{"/bin/true"}},
			expectedCode: codes.NotFound,
			expectedErr:  `container "app" not found`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			srv := &fakeExecServer{ctx: t.Context()}

			req := &machine.DebugContainerExecRequest{}
			if test.spec != nil {
				req.Request = &machine.DebugContainerExecRequest_Spec{Spec: test.spec}
			} else {
				req.Request = &machine.DebugContainerExecRequest_StdinData{StdinData: []byte("data")}
			}

			srv.requests = append(srv.requests, req)

			s := &Service{Resources: newExecState(t, false, nil, false, false)}

			err := s.ContainerExec(srv)
			require.Error(t, err)
			assert.Equal(t, test.expectedCode, status.Code(err))
			assert.ErrorContains(t, err, test.expectedErr)
		})
	}
}

func TestExecStreamRecv(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		tty      bool
		expected []controlMessage
	}{
		{
			name: "no terminal",
			expected: []controlMessage{
				{stdinData: []byte("data")},
				{stdinEOF: true},
				{signal: 15},
				{resize: &machine.DebugContainerTerminalResize{Width: 80, Height: 24}},
			},
		},
		{
			name: "terminal keeps stdin open",
			tty:  true,
			expected: []controlMessage{
				{stdinData: []byte("data")},
				{signal: 15},
				{resize: &machine.DebugContainerTerminalResize{Width: 80, Height: 24}},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			srv := &fakeExecServer{
				ctx: ctx,
				requests: []*machine.DebugContainerExecRequest{
					{Request: &machine.DebugContainerExecRequest_StdinData{StdinData: []byte("data")}},
					{Request: &machine.DebugContainerExecRequest_StdinEof{StdinEof: true}},
					{Request: &machine.DebugContainerExecRequest_Signal{Signal: 15}},
					{Request: &machine.DebugContainerExecRequest_TermResize{TermResize: &machine.DebugContainerTerminalResize{Width: 80, Height: 24}}},
				},
			}

			stream := execStream{srv: srv, tty: test.tty}

			messages := make([]controlMessage, 0, len(test.expected))

			for range test.expected {
				msg, err := stream.recv()
				require.NoError(t, err)

				messages = append(messages, msg)
			}

			assert.Equal(t, test.expected, messages)

			eofC := make(chan error, 1)

			go func() {
				_, err := stream.recv()
				eofC <- err
			}()

			if test.tty {
				// a half-close doesn't end stdin of a terminal before the call ends
				select {
				case err := <-eofC:
					t.Fatalf("half-close returned before the call ended: %v", err)
				case <-time.After(100 * time.Millisecond):
				}

				cancel()
			}

			assert.ErrorIs(t, <-eofC, io.EOF)
		})
	}
}
