// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package hypervisord serves live, host-local VM serial sessions.
package hypervisord

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/digitalocean/go-libvirt"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

const maxInputChunk = 64 * 1024

// Service attaches to managed, running virtual machine consoles.
type Service struct {
	machine.UnimplementedHypervisorServiceServer
	resources   state.State
	open        func(context.Context, domain.Domain) (io.ReadWriteCloser, error)
	mu          sync.Mutex
	attached    map[string]struct{}
	openVNC     func(context.Context, domain.Domain) (io.ReadWriteCloser, error)
	vncAttached map[string]struct{}
}

// ServiceOption configures optional hypervisor service capabilities.
type ServiceOption func(*Service)

// WithVNCConnector enables VNC attachment. The connector must verify live ownership,
// UUID, running state and a supported graphics endpoint, honor ctx, and return a
// connection whose Close interrupts concurrent reads and writes.
func WithVNCConnector(open func(context.Context, domain.Domain) (io.ReadWriteCloser, error)) ServiceOption {
	return func(s *Service) { s.openVNC = open }
}

// NewService binds resource state and a dedicated libvirt console connector.
// The connector must verify live ownership, UUID and running state, honor ctx,
// and return a console whose Close interrupts concurrent reads and writes.
func NewService(resources state.State, open func(context.Context, domain.Domain) (io.ReadWriteCloser, error), options ...ServiceOption) *Service {
	s := &Service{
		resources:   resources,
		open:        open,
		attached:    make(map[string]struct{}),
		vncAttached: make(map[string]struct{}),
	}

	for _, option := range options {
		option(s)
	}

	return s
}

// ConsoleStream requires attach first. Input EOF detaches the entire session.
func (s *Service) ConsoleStream(stream grpc.BidiStreamingServer[machine.ConsoleRequest, machine.ConsoleResponse]) (retErr error) {
	first, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "receive console attach: %v", err)
	}

	if first.GetAttach().GetName() == "" {
		return status.Error(codes.InvalidArgument, "first console frame must attach a named virtual machine")
	}

	identity, err := s.identity(stream.Context(), first.GetAttach().GetName())
	if err != nil {
		return err
	}

	if !s.acquire(identity.Name) {
		return status.Error(codes.AlreadyExists, "virtual machine console is already attached")
	}
	defer s.release(identity.Name)

	console, err := s.open(stream.Context(), identity)
	if err != nil {
		return openConsoleError(err)
	}

	// Close exactly once, including when cancellation races normal completion.
	var (
		closeOnce sync.Once
		closeErr  error
	)

	closeConsole := func() { closeOnce.Do(func() { closeErr = console.Close() }) }
	defer func() {
		closeConsole()

		if retErr == nil && closeErr != nil {
			retErr = fmt.Errorf("close console: %w", closeErr)
		}
	}()

	stop := context.AfterFunc(stream.Context(), closeConsole)
	defer stop()

	return relay(stream, console)
}

func openConsoleError(err error) error {
	if rpcStatus, ok := status.FromError(err); ok {
		return rpcStatus.Err()
	}

	code := codes.Unavailable

	if preconditionErr, ok := errors.AsType[*domain.ConsolePreconditionError](err); ok && preconditionErr != nil {
		code = codes.FailedPrecondition
	} else if rpcErr, ok := errors.AsType[libvirt.Error](err); ok {
		switch rpcErr.Code {
		case uint32(libvirt.ErrNoDomain):
			code = codes.NotFound
		case uint32(libvirt.ErrOperationFailed), uint32(libvirt.ErrOperationInvalid):
			// These codes cover more than console contention: the requested
			// operation cannot proceed, but its exact cause is not known here.
			code = codes.FailedPrecondition
		default:
			// Unknown libvirt errors remain unavailable.
		}
	}

	return status.Errorf(code, "open virtual machine console: %v", err)
}

func (s *Service) identity(ctx context.Context, name string) (domain.Domain, error) {
	return s.attachmentIdentity(ctx, name, false)
}

func (s *Service) attachmentIdentity(ctx context.Context, name string, vnc bool) (domain.Domain, error) {
	spec, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](ctx, s.resources, name)
	if state.IsNotFoundError(err) {
		return domain.Domain{}, status.Error(codes.NotFound, "virtual machine is not managed")
	}

	if err != nil {
		return domain.Domain{}, fmt.Errorf("read virtual machine spec: %w", err)
	}

	enabled := spec.TypedSpec().Console.Serial
	if vnc {
		enabled = spec.TypedSpec().Console.VNC
	}

	if spec.Metadata().Phase() != resource.PhaseRunning || !enabled || spec.TypedSpec().PowerState != "running" {
		return domain.Domain{}, status.Error(codes.FailedPrecondition, "virtual machine attachment is not enabled and running")
	}

	system, err := safe.StateGetByID[*hardware.SystemInformation](ctx, s.resources, hardware.SystemInformationID)
	if err != nil {
		return domain.Domain{}, fmt.Errorf("read system information: %w", err)
	}

	machineUUID, err := uuid.Parse(system.TypedSpec().UUID)
	if err != nil || machineUUID == uuid.Nil {
		return domain.Domain{}, status.Error(codes.FailedPrecondition, "invalid machine UUID")
	}

	// Observed status may be stale or absent. The connector checks the live domain.
	return domain.Domain{Name: name, UUID: domain.UUID(machineUUID, name)}, nil
}

func (s *Service) acquire(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.attached[name]; exists {
		return false
	}

	s.attached[name] = struct{}{}

	return true
}

func (s *Service) release(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.attached, name)
}

func relay(stream grpc.BidiStreamingServer[machine.ConsoleRequest, machine.ConsoleResponse], console io.ReadWriter) error {
	// There is no guest-data queue. Each direction blocks on its destination.
	// Returning closes the console and lets gRPC cancel blocked Send/Recv calls.
	// Each worker can publish its terminal result without waiting for the handler.
	done := make(chan error, 2)

	go func() { done <- sendOutput(stream, console) }()
	go func() { done <- receiveInput(stream, console) }()

	select {
	case <-stream.Context().Done():
		return status.FromContextError(stream.Context().Err()).Err()
	case err := <-done:
		if errors.Is(err, io.EOF) {
			return nil
		}

		return err
	}
}

func sendOutput(stream grpc.BidiStreamingServer[machine.ConsoleRequest, machine.ConsoleResponse], console io.Reader) error {
	buf := make([]byte, 32*1024)

	for {
		n, err := console.Read(buf)
		if n > 0 {
			if sendErr := stream.Send(&machine.ConsoleResponse{StdoutData: append([]byte(nil), buf[:n]...)}); sendErr != nil {
				return sendErr
			}
		}

		if err != nil {
			return err
		}

		if n == 0 {
			return io.ErrNoProgress
		}
	}
}

func receiveInput(stream grpc.BidiStreamingServer[machine.ConsoleRequest, machine.ConsoleResponse], console io.Writer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}

		data, ok := req.GetRequest().(*machine.ConsoleRequest_StdinData)
		if !ok {
			return status.Error(codes.InvalidArgument, "expected console input data")
		}

		if len(data.StdinData) > maxInputChunk {
			return status.Error(codes.ResourceExhausted, "console input frame exceeds 64 KiB")
		}

		if err = writeInput(console, data.StdinData); err != nil {
			return err
		}
	}
}

func writeInput(console io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := console.Write(data)
		if err != nil {
			return err
		}

		if n <= 0 || n > len(data) {
			return io.ErrShortWrite
		}

		data = data[n:]
	}

	return nil
}
