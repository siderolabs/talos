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
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

const maxInputChunk = 64 * 1024

// ConfigPatcher applies a change to the machine configuration which is live, and persists it.
//
// The implementation serializes the change against every other writer of the machine
// configuration, and rejects a change which does not validate.
type ConfigPatcher interface {
	PatchConfiguration(ctx context.Context, patch func(config.Container) (config.Provider, error)) error
}

// Service attaches to managed, running virtual machine consoles, and drives the power state of
// managed virtual machines.
type Service struct {
	machine.UnimplementedHypervisorServiceServer
	resources   state.State
	open        func(context.Context, domain.Domain) (io.ReadWriteCloser, error)
	mu          sync.Mutex
	attached    map[string]struct{}
	openVNC     func(context.Context, domain.Domain) (io.ReadWriteCloser, error)
	vncAttached map[string]struct{}
	domains     func(context.Context) (domain.Client, error)
	config      ConfigPatcher
	power       state.State
	powerMu     sync.Mutex
}

// ServiceOption configures optional hypervisor service capabilities.
type ServiceOption func(*Service)

// WithVNCConnector enables VNC attachment. The connector must verify live ownership,
// UUID, running state and a supported graphics endpoint, honor ctx, and return a
// connection whose Close interrupts concurrent reads and writes.
func WithVNCConnector(open func(context.Context, domain.Domain) (io.ReadWriteCloser, error)) ServiceOption {
	return func(s *Service) { s.openVNC = open }
}

// WithDomainConnector enables the power operations which act on a live domain rather than on the
// machine configuration. The connector must honor ctx and return a client whose Close releases the
// connection.
func WithDomainConnector(open func(context.Context) (domain.Client, error)) ServiceOption {
	return func(s *Service) { s.domains = open }
}

// WithConfigPatcher enables the power operations which are carried out by writing the machine
// configuration.
func WithConfigPatcher(patcher ConfigPatcher) ServiceOption {
	return func(s *Service) { s.config = patcher }
}

// WithPowerState enables the power operations which read and write resource state: recording how
// a stop is carried out, and checking a virtual machine is ready to be rebooted.
//
// The state is unfiltered, unlike the one the console and VNC sessions read through: a stop mode is
// not something a client may write for itself, and a reboot by an operator reads the sensitive
// VirtualMachineSpec. Each RPC which reaches it is authorized by its own role set, and it is used
// for nothing else.
func WithPowerState(power state.State) ServiceOption {
	return func(s *Service) { s.power = power }
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

	machineUUID, err := readMachineUUID(ctx, s.resources)
	if err != nil {
		return domain.Domain{}, err
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
