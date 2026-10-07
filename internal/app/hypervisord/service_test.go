// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisord_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/digitalocean/go-libvirt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/siderolabs/talos/internal/app/hypervisord"
	"github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

const machineUUID = "c737f778-82a1-48dd-990b-67901031bcc5"

func setup(t *testing.T) state.State {
	t.Helper()

	st := state.WrapCore(namespaced.NewState(inmem.Build))
	system := hardware.NewSystemInformation(hardware.SystemInformationID)
	system.TypedSpec().UUID = machineUUID
	require.NoError(t, st.Create(t.Context(), system))

	return st
}

func addVM(t *testing.T, st state.State, serial bool, power string) {
	t.Helper()

	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm1")
	vm.TypedSpec().PowerState = power
	vm.TypedSpec().Console.Serial = serial
	require.NoError(t, st.Create(t.Context(), vm))
}

func client(t *testing.T, st state.State, open func(context.Context, domain.Domain) (io.ReadWriteCloser, error), options ...hypervisord.ServiceOption) machine.HypervisorServiceClient {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	machine.RegisterHypervisorServiceServer(server, hypervisord.NewService(st, open, options...))

	done := make(chan error, 1)

	go func() { done <- server.Serve(listener) }()

	conn, err := grpc.NewClient("passthrough:///console", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
		server.Stop()
		require.NoError(t, <-done)
	})

	return machine.NewHypervisorServiceClient(conn)
}

func attach(name string) *machine.ConsoleRequest {
	return &machine.ConsoleRequest{
		Request: &machine.ConsoleRequest_Attach{
			Attach: &machine.ConsoleAttach{Name: name},
		},
	}
}

func input(data []byte) *machine.ConsoleRequest {
	return &machine.ConsoleRequest{Request: &machine.ConsoleRequest_StdinData{StdinData: data}}
}

func openStream(t *testing.T, c machine.HypervisorServiceClient) (grpc.BidiStreamingClient[machine.ConsoleRequest, machine.ConsoleResponse], context.CancelFunc) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	stream, err := c.ConsoleStream(ctx)
	require.NoError(t, err)

	return stream, cancel
}

func TestAttachFirst(t *testing.T) {
	for _, request := range []*machine.ConsoleRequest{nil, {}, attach(""), input([]byte("before attach"))} {
		t.Run("invalid first frame", func(t *testing.T) {
			var called atomic.Bool

			c := client(t, setup(t), func(context.Context, domain.Domain) (io.ReadWriteCloser, error) {
				called.Store(true)

				return nil, status.Error(codes.Internal, "unexpected open")
			})

			stream, _ := openStream(t, c)
			if request == nil {
				require.NoError(t, stream.CloseSend())
			} else {
				require.NoError(t, stream.Send(request))
			}

			_, err := stream.Recv()
			require.Equal(t, codes.InvalidArgument, status.Code(err))
			require.False(t, called.Load())
		})
	}
}

func TestDesiredStateRejections(t *testing.T) {
	for _, test := range []struct {
		name    string
		present bool
		serial  bool
		power   string
		code    codes.Code
	}{
		{name: "absent", code: codes.NotFound},
		{name: "disabled", present: true, power: "running", code: codes.FailedPrecondition},
		{name: "stopped", present: true, serial: true, power: "stopped", code: codes.FailedPrecondition},
	} {
		t.Run(test.name, func(t *testing.T) {
			st := setup(t)
			if test.present {
				addVM(t, st, test.serial, test.power)
			}

			var called atomic.Bool

			c := client(t, st, func(context.Context, domain.Domain) (io.ReadWriteCloser, error) {
				called.Store(true)

				return nil, status.Error(codes.Internal, "unexpected open")
			})
			stream, _ := openStream(t, c)
			require.NoError(t, stream.Send(attach("vm1")))
			_, err := stream.Recv()
			require.Equal(t, test.code, status.Code(err))
			require.False(t, called.Load())
		})
	}
}

func TestLiveValidationDoesNotTrustObservedStatus(t *testing.T) {
	for _, reason := range []string{"absent", "stopped", "foreign"} {
		t.Run(reason, func(t *testing.T) {
			st := setup(t)
			addVM(t, st, true, "running")

			observed := hypervisor.NewVirtualMachineDomainStatus(hypervisor.NamespaceName, "vm1")
			observed.TypedSpec().UUID = domain.UUID(uuid.MustParse(machineUUID), "vm1").String()
			observed.TypedSpec().PowerState = hypervisor.VirtualMachinePowerStateRunning
			require.NoError(t, st.Create(t.Context(), observed))

			identities := make(chan domain.Domain, 2)
			c := client(t, st, func(_ context.Context, identity domain.Domain) (io.ReadWriteCloser, error) {
				identities <- identity

				return nil, status.Error(codes.FailedPrecondition, reason)
			})
			// Failed opens must release the exclusive slot too.
			for range 2 {
				stream, _ := openStream(t, c)
				require.NoError(t, stream.Send(attach("vm1")))
				_, err := stream.Recv()
				require.Equal(t, codes.FailedPrecondition, status.Code(err))
				require.Equal(t, domain.Domain{Name: "vm1", UUID: domain.UUID(uuid.MustParse(machineUUID), "vm1")}, <-identities)
			}
		})
	}
}

func TestOpenErrorStatus(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		code codes.Code
	}{
		{name: "missing live domain", err: fmt.Errorf("lookup: %w", libvirt.Error{Code: uint32(libvirt.ErrNoDomain), Message: "missing"}), code: codes.NotFound},
		{
			name: "external console occupied",
			err:  fmt.Errorf("attach: %w", libvirt.Error{Code: uint32(libvirt.ErrOperationFailed), Message: "Active console session exists for this domain"}),
			code: codes.FailedPrecondition,
		},
		{name: "other failed operation", err: libvirt.Error{Code: uint32(libvirt.ErrOperationFailed), Message: "generic failure"}, code: codes.FailedPrecondition},
		{name: "invalid operation", err: libvirt.Error{Code: uint32(libvirt.ErrOperationInvalid), Message: "invalid state"}, code: codes.FailedPrecondition},
		{name: "connection unavailable", err: libvirt.Error{Code: uint32(libvirt.ErrNoConnect), Message: "disconnected"}, code: codes.Unavailable},
		{name: "other error", err: fmt.Errorf("dial failed"), code: codes.Unavailable},
		{name: "existing status", err: fmt.Errorf("connector: %w", status.Error(codes.PermissionDenied, "access denied")), code: codes.PermissionDenied},
	} {
		t.Run(test.name, func(t *testing.T) {
			st := setup(t)
			addVM(t, st, true, "running")
			c := client(t, st, func(context.Context, domain.Domain) (io.ReadWriteCloser, error) {
				return nil, test.err
			})

			// An unsuccessful open must also release the local attachment slot.
			for range 2 {
				stream, _ := openStream(t, c)
				require.NoError(t, stream.Send(attach("vm1")))
				_, err := stream.Recv()
				require.Equal(t, test.code, status.Code(err))
			}
		})
	}
}
