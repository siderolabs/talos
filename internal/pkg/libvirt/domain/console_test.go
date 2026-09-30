// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain_test

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/siderolabs/talos/internal/app/hypervisord"
	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

const consoleMachineUUID = "c737f778-82a1-48dd-990b-67901031bcc5"

func TestConsoleRejectsInvalidIdentity(t *testing.T) {
	t.Parallel()
	_, err := libvirtdomain.New("/missing", "qemu:///system").OpenConsole(t.Context(), libvirtdomain.Domain{UUID: uuid.Nil})
	require.ErrorContains(t, err, "name and UUID")
}

func consoleXDRString(value string) []byte {
	p := binary.BigEndian.AppendUint32(nil, uint32(len(value)))
	p = append(p, value...)

	return append(p, make([]byte, (4-len(value)%4)%4)...)
}

func consoleFixture(t *testing.T, mode string) (*libvirtdomain.Connector, libvirtdomain.Domain, <-chan error, chan struct{}) {
	t.Helper()
	socket := filepath.Join(t.TempDir(), "console.sock")
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", socket)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })

	d := libvirtdomain.Domain{Name: "guest", UUID: libvirtdomain.UUID(uuid.MustParse(consoleMachineUUID), "guest")}
	done := make(chan error, 1)
	release := make(chan struct{})

	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			done <- acceptErr

			return
		}

		if deadlineErr := conn.SetDeadline(time.Now().Add(5 * time.Second)); deadlineErr != nil {
			done <- errors.Join(deadlineErr, conn.Close())

			return
		}

		fixtureErr := serveConsoleFixture(conn, d, mode, release)
		done <- errors.Join(fixtureErr, conn.Close())
	}()

	return libvirtdomain.New(socket, "qemu:///system"), d, done, release
}

func serveConsoleFixture(conn net.Conn, d libvirtdomain.Domain, mode string, release <-chan struct{}) error {
	if err := serveHandshake(conn); err != nil {
		return err
	}

	for _, proc := range []uint32{23, 14, 150, 201} {
		call, err := readCall(conn)
		if err != nil {
			if consoleFixtureRejectsBeforeAttach(mode) {
				return nil
			}

			return err
		}

		if binary.BigEndian.Uint32(call[8:12]) != proc {
			return fmt.Errorf("unexpected procedure: %x", call)
		}

		if mode == "transport-failure" && proc == 23 {
			return nil
		}

		if proc == 201 {
			return attachConsoleFixture(conn, call, mode, release)
		}

		if err = replyCall(conn, call, consoleFixturePayload(d, mode, proc)); err != nil {
			return err
		}
	}

	return nil
}

func consoleFixtureRejectsBeforeAttach(mode string) bool {
	return strings.HasPrefix(mode, "reject-") || mode == "malformed-xml"
}

func consoleFixturePayload(d libvirtdomain.Domain, mode string, proc uint32) []byte {
	var payload []byte

	switch proc {
	case 23:
		payload = consoleXDRString(d.Name)

		id := d.UUID
		if mode == "reject-uuid" {
			id = uuid.New()
		}

		payload = append(payload, id[:]...)
		payload = binary.BigEndian.AppendUint32(payload, 7)
	case 14:
		if mode == "malformed-xml" {
			return consoleXDRString(`<domain`)
		}

		metadata := `<metadata><definition xmlns="https://talos.dev/libvirt/domain">digest</definition></metadata>`
		serial := `<devices><serial type="pty"><target port="0"/></serial></devices>`

		if mode == "reject-owner" {
			metadata = ""
		}

		if mode == "reject-serial" {
			serial = ""
		}

		payload = consoleXDRString(fmt.Sprintf(`<domain><name>%s</name><uuid>%s</uuid>%s%s</domain>`, d.Name, d.UUID, metadata, serial))
	case 150:
		active := uint32(1)
		if mode == "reject-inactive" {
			active = 0
		}

		payload = binary.BigEndian.AppendUint32(nil, active)
	}

	return payload
}

func attachConsoleFixture(conn net.Conn, call []byte, mode string, release <-chan struct{}) error {
	if strings.HasPrefix(mode, "reject-") {
		return fmt.Errorf("unsafe domain reached console attachment")
	}

	if binary.BigEndian.Uint32(call[len(call)-4:]) != 2 {
		return fmt.Errorf("console must use SAFE only: %x", call)
	}

	if mode == "busy" {
		return consoleReply(conn, call, 1, 1, append(make([]byte, 8), 0, 0, 0, 0))
	}

	if err := replyCall(conn, call, nil); err != nil {
		return err
	}

	return serveConsoleStream(conn, call, mode, release)
}

func consoleReply(conn net.Conn, call []byte, kind, status uint32, payload []byte) error {
	frame := binary.BigEndian.AppendUint32(nil, uint32(28+len(payload)))
	frame = append(frame, call[:24]...)
	binary.BigEndian.PutUint32(frame[16:20], kind)
	binary.BigEndian.PutUint32(frame[24:28], status)
	frame = append(frame, payload...)
	_, err := conn.Write(frame)

	return err
}

func serveConsoleStream(conn net.Conn, call []byte, mode string, release <-chan struct{}) error {
	if mode == "roundtrip" {
		if err := consoleRoundtrip(conn, call); err != nil {
			return err
		}
	}

	<-release

	if status, payload, ok := consoleTerminalFrame(mode); ok {
		if err := consoleReply(conn, call, 3, status, payload); err != nil {
			return err
		}
	}

	switch mode {
	case "loss":
		return nil
	case "flood":
		return floodConsole(conn, call)
	}

	_, err := io.Copy(io.Discard, conn)

	return err
}

func consoleTerminalFrame(mode string) (uint32, []byte, bool) {
	switch mode {
	case "eof", "roundtrip", "eof-blocked":
		return 0, nil, true
	case "empty-eof":
		return 2, nil, true
	case "unknown-status-empty":
		return 99, nil, true
	case "unknown-status-payload":
		return 99, []byte("output"), true
	default:
		return 0, nil, false
	}
}

func floodConsole(conn net.Conn, call []byte) error {
	for range 64 {
		if err := consoleReply(conn, call, 3, 2, []byte("output")); err != nil {
			return err
		} // overflow closes the peer
	}

	_, err := io.Copy(io.Discard, conn)

	return err
}

func TestConsoleWireBoundsUnreadOutput(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	connector, d, done, release := consoleFixture(t, "flood")
	stream, err := connector.OpenConsole(t.Context(), d)

	require.NoError(t, err)
	defer func() { require.NoError(t, stream.Close()) }()

	close(release)

	select {
	case <-done: // the peer may observe EOF or a write error at the queue bound
	case <-time.After(3 * time.Second):
		t.Fatal("unread output did not terminate at queue bound")
	}

	_, err = io.ReadAll(stream)
	require.ErrorContains(t, err, "queue overflow")
}

func consoleRoundtrip(conn net.Conn, call []byte) error {
	input, err := readCall(conn)
	if err != nil {
		return err
	}

	if string(input[24:]) != "hello" || binary.BigEndian.Uint32(input[12:16]) != 3 {
		return fmt.Errorf("unexpected console input %x", input)
	}

	return consoleReply(conn, call, 3, 2, []byte("world"))
}

func TestConsoleWireLifecycle(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	for _, mode := range []string{"roundtrip", "eof", "empty-eof", "loss", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			connector, d, done, release := consoleFixture(t, mode)

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			stream, err := connector.OpenConsole(ctx, d)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, stream.Close()) })

			if mode == "roundtrip" {
				n, writeErr := stream.Write([]byte("hello"))
				require.NoError(t, writeErr)
				require.Equal(t, 5, n)

				out := make([]byte, 5)
				_, err = io.ReadFull(stream, out)
				require.NoError(t, err)
				require.Equal(t, "world", string(out))
			}

			close(release)

			if mode == "cancel" {
				cancel()
			}

			result := make(chan error, 1)
			go func() { _, readErr := stream.Read(make([]byte, 1)); result <- readErr }()

			select {
			case err = <-result:
				require.Error(t, err)
			case <-time.After(3 * time.Second):
				t.Fatal("idle console did not terminate")
			}

			require.NoError(t, stream.Close())
			require.NoError(t, <-done)
		})
	}
}

func TestConsoleWireRejectsUnknownStreamStatus(t *testing.T) {
	for _, mode := range []string{"unknown-status-empty", "unknown-status-payload"} {
		t.Run(mode, func(t *testing.T) {
			connector, d, done, release := consoleFixture(t, mode)
			stream, err := connector.OpenConsole(t.Context(), d)
			require.NoError(t, err)

			close(release)

			_, err = io.ReadAll(stream)
			require.ErrorContains(t, err, "invalid console stream status")
			require.NoError(t, stream.Close())
			require.NoError(t, <-done)
		})
	}
}

func TestConsoleWireIdleEOFReleasesAttachment(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	connector, d, done, release := consoleFixture(t, "eof")
	stream, err := connector.OpenConsole(t.Context(), d)

	require.NoError(t, err)
	defer func() { require.NoError(t, stream.Close()) }()

	close(release)
	// No application Read or Write: the socket reader must still release the
	// attachment and its goroutines when the guest terminates the stream.
	select {
	case err = <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("idle guest EOF did not close transport")
	}

	require.NoError(t, stream.Close())
}

func TestConsoleWireUnblocksBothDirections(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	for _, mode := range []string{"cancel-blocked", "eof-blocked"} {
		t.Run(mode, func(t *testing.T) {
			connector, d, done, release := consoleFixture(t, mode)

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			stream, err := connector.OpenConsole(ctx, d)

			require.NoError(t, err)
			defer func() { require.NoError(t, stream.Close()) }()

			results := make(chan error, 2)
			go func() { _, writeErr := stream.Write(make([]byte, 16<<20)); results <- writeErr }()
			go func() { _, readErr := stream.Read(make([]byte, 1)); results <- readErr }()
			// The peer deliberately does not drain input until after the terminal event.
			if mode == "cancel-blocked" {
				cancel()
			}

			close(release)

			for range 2 {
				select {
				case err = <-results:
					require.Error(t, err)
				case <-time.After(3 * time.Second):
					t.Fatal("console I/O remained blocked")
				}
			}

			require.NoError(t, stream.Close())
			require.NoError(t, <-done)
		})
	}
}

func TestConsoleWireRejectsUnsafeAttachment(t *testing.T) {
	for _, mode := range []string{"reject-uuid", "reject-owner", "reject-serial", "reject-inactive", "busy"} {
		t.Run(mode, func(t *testing.T) {
			connector, d, done, _ := consoleFixture(t, mode)
			stream, err := connector.OpenConsole(t.Context(), d)
			require.Error(t, err)
			require.Nil(t, stream)
			require.NoError(t, <-done)
		})
	}
}

func TestConsoleLiveValidationRPCStatus(t *testing.T) {
	for _, test := range []struct {
		mode string
		code codes.Code
	}{
		{mode: "reject-uuid", code: codes.FailedPrecondition},
		{mode: "reject-owner", code: codes.FailedPrecondition},
		{mode: "reject-serial", code: codes.FailedPrecondition},
		{mode: "reject-inactive", code: codes.FailedPrecondition},
		{mode: "malformed-xml", code: codes.Unavailable},
		{mode: "transport-failure", code: codes.Unavailable},
	} {
		t.Run(test.mode, func(t *testing.T) {
			connector, d, peerDone, _ := consoleFixture(t, test.mode)
			st := state.WrapCore(namespaced.NewState(inmem.Build))

			system := hardware.NewSystemInformation(hardware.SystemInformationID)
			system.TypedSpec().UUID = consoleMachineUUID
			require.NoError(t, st.Create(t.Context(), system))

			vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, d.Name)
			vm.TypedSpec().Console.Serial = true
			vm.TypedSpec().PowerState = "running"
			require.NoError(t, st.Create(t.Context(), vm))

			listener := bufconn.Listen(1024 * 1024)
			server := grpc.NewServer()
			machine.RegisterHypervisorServiceServer(server, hypervisord.NewService(st, connector.OpenConsole))

			serveDone := make(chan error, 1)
			go func() { serveDone <- server.Serve(listener) }()

			conn, err := grpc.NewClient(
				"passthrough:///console-live-validation",
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
					return listener.DialContext(ctx)
				}),
			)
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, conn.Close())
				server.Stop()
				require.NoError(t, <-serveDone)
			})

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			stream, err := machine.NewHypervisorServiceClient(conn).ConsoleStream(ctx)
			require.NoError(t, err)
			require.NoError(t, stream.Send(&machine.ConsoleRequest{
				Request: &machine.ConsoleRequest_Attach{
					Attach: &machine.ConsoleAttach{Name: d.Name},
				},
			}))

			_, err = stream.Recv()
			require.Equal(t, test.code, status.Code(err), "console open returned %v", err)
			require.NoError(t, <-peerDone)
		})
	}
}

func handshakeDisconnectFixture(t *testing.T, connections int) (string, <-chan error) {
	t.Helper()

	socket := filepath.Join(t.TempDir(), "handshake-disconnect.sock")
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", socket)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })

	done := make(chan error, 1)

	go func() {
		for range connections {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				done <- acceptErr

				return
			}

			if deadlineErr := conn.SetDeadline(time.Now().Add(5 * time.Second)); deadlineErr != nil {
				done <- errors.Join(deadlineErr, conn.Close())

				return
			}

			handshakeErr := serveHandshake(conn)
			closeErr := conn.Close()

			if handshakeErr != nil {
				done <- handshakeErr

				return
			}

			if closeErr != nil {
				done <- closeErr

				return
			}
		}

		done <- nil
	}()

	return socket, done
}

func TestConsoleImmediatePostHandshakeDisconnect(t *testing.T) {
	const attempts = 20

	socket, peerDone := handshakeDisconnectFixture(t, attempts)
	connector := libvirtdomain.New(socket, "qemu:///system")
	d := libvirtdomain.Domain{Name: "guest", UUID: uuid.New()}

	for attempt := range attempts {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		result := make(chan error, 1)

		go func() {
			stream, err := connector.OpenConsole(ctx, d)
			if stream != nil {
				err = errors.Join(err, stream.Close())
			}

			result <- err
		}()

		select {
		case err := <-result:
			require.Error(t, err)
		case <-time.After(2 * time.Second):
			cancel()
			t.Fatalf("OpenConsole exceeded its caller deadline after handshake disconnect, attempt %d", attempt)
		}

		cancel()
	}

	require.NoError(t, <-peerDone)
}

func TestConsoleImmediatePostHandshakeDisconnectReleasesServiceSlot(t *testing.T) {
	const connections = 2

	socket, peerDone := handshakeDisconnectFixture(t, connections)
	connector := libvirtdomain.New(socket, "qemu:///system")
	d := libvirtdomain.Domain{Name: "guest", UUID: libvirtdomain.UUID(uuid.MustParse(consoleMachineUUID), "guest")}
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	system := hardware.NewSystemInformation(hardware.SystemInformationID)
	system.TypedSpec().UUID = consoleMachineUUID
	require.NoError(t, st.Create(t.Context(), system))

	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, d.Name)
	vm.TypedSpec().Console.Serial = true
	vm.TypedSpec().PowerState = "running"
	require.NoError(t, st.Create(t.Context(), vm))

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	openDone := make(chan struct{}, connections)

	machine.RegisterHypervisorServiceServer(server, hypervisord.NewService(st, func(ctx context.Context, identity libvirtdomain.Domain) (io.ReadWriteCloser, error) {
		defer func() { openDone <- struct{}{} }()

		return connector.OpenConsole(ctx, identity)
	}))

	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()

	conn, err := grpc.NewClient(
		"passthrough:///console-handshake-disconnect",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
		server.Stop()
		require.NoError(t, <-serveDone)
	})

	client := machine.NewHypervisorServiceClient(conn)
	firstCtx, firstCancel := context.WithTimeout(t.Context(), time.Second)
	first, err := client.ConsoleStream(firstCtx)
	require.NoError(t, err)
	require.NoError(t, first.Send(&machine.ConsoleRequest{
		Request: &machine.ConsoleRequest_Attach{
			Attach: &machine.ConsoleAttach{Name: d.Name},
		},
	}))

	_, firstErr := first.Recv()

	firstCancel()
	require.Error(t, firstErr)

	select {
	case <-openDone:
	case <-time.After(time.Second):
		t.Fatal("console connector remained blocked after the first caller deadline")
	}

	nextCtx, nextCancel := context.WithTimeout(t.Context(), time.Second)
	defer nextCancel()

	next, err := client.ConsoleStream(nextCtx)
	require.NoError(t, err)
	require.NoError(t, next.Send(&machine.ConsoleRequest{
		Request: &machine.ConsoleRequest_Attach{
			Attach: &machine.ConsoleAttach{Name: d.Name},
		},
	}))

	_, nextErr := next.Recv()
	if status.Code(nextErr) == codes.AlreadyExists {
		t.Fatal("failed startup retained the exclusive console slot")
	}

	require.Error(t, nextErr)

	select {
	case <-openDone:
	case <-time.After(time.Second):
		t.Fatal("console connector remained blocked after the second caller deadline")
	}

	require.NoError(t, <-peerDone)
}

func startupFailureFixture(t *testing.T, modes ...string) (string, <-chan error) {
	t.Helper()

	socket := filepath.Join(t.TempDir(), "startup-failure.sock")
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", socket)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })

	done := make(chan error, 1)

	go func() {
		for _, mode := range modes {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				done <- acceptErr

				return
			}

			if deadlineErr := conn.SetDeadline(time.Now().Add(5 * time.Second)); deadlineErr != nil {
				done <- errors.Join(deadlineErr, conn.Close())

				return
			}

			fixtureErr := serveStartupFailure(conn, mode)

			closeErr := conn.Close()
			if err := errors.Join(fixtureErr, closeErr); err != nil {
				done <- err

				return
			}
		}

		done <- nil
	}()

	return socket, done
}

func serveStartupFailure(conn net.Conn, mode string) error {
	auth, err := readCall(conn)
	if err != nil {
		return err
	}

	if actual := binary.BigEndian.Uint32(auth[8:12]); actual != 66 {
		return fmt.Errorf("expected AUTH_LIST, got procedure %d", actual)
	}

	if err = replyCall(conn, auth, binary.BigEndian.AppendUint32(nil, 0)); err != nil {
		return err
	}

	connect, err := readCall(conn)
	if err != nil {
		return err
	}

	if actual := binary.BigEndian.Uint32(connect[8:12]); actual != 1 {
		return fmt.Errorf("expected CONNECT_OPEN, got procedure %d", actual)
	}

	success := rpcReply(connect, 0, nil)
	malformed := rpcFrame(201, 99, 0x7fffffff, 2, nil)

	actions := map[string]func() error{
		"eof-before-reply": func() error { return nil },
		"malformed-before-reply": func() error {
			_, err = conn.Write(malformed)

			return err
		},
		"unmatched-success": func() error {
			wrong := append([]byte(nil), connect...)
			binary.BigEndian.PutUint32(wrong[16:20], 12345)
			_, err = conn.Write(rpcReply(wrong, 0, nil))

			return err
		},
		"failed-reply": func() error {
			_, err = conn.Write(rpcReply(connect, 1, append(make([]byte, 8), 0, 0, 0, 0)))

			return err
		},
		"matching-invalid-header": func() error {
			invalid := append([]byte(nil), connect...)
			binary.BigEndian.PutUint32(invalid[8:12], 999)
			_, err = conn.Write(rpcReply(invalid, 0, nil))

			return err
		},
		"matching-unknown-status": func() error {
			_, err = conn.Write(rpcReply(connect, 99, nil))

			return err
		},
		"successful-reply-then-eof": func() error {
			_, err = conn.Write(success)

			return err
		},
		"successful-reply-then-malformed": func() error {
			if _, err = conn.Write(success[:7]); err != nil {
				return err
			}

			if _, err = conn.Write(success[7:]); err != nil {
				return err
			}

			_, err = conn.Write(malformed)

			return err
		},
		"unmatched-then-successful-then-malformed": func() error {
			wrong := append([]byte(nil), connect...)
			binary.BigEndian.PutUint32(wrong[16:20], 12345)
			packet := append(rpcReply(wrong, 0, nil), success...)
			packet = append(packet, malformed...)
			_, err = conn.Write(packet)

			return err
		},
		"repeated-success-then-eof": func() error {
			packet := append([]byte(nil), success...)
			packet = append(packet, success...)
			_, err = conn.Write(packet)

			return err
		},
		"successful-reply-then-invalid-duplicate-header": func() error {
			invalid := append([]byte(nil), connect...)
			binary.BigEndian.PutUint32(invalid[8:12], 999)

			packet := append([]byte(nil), success...)
			packet = append(packet, rpcReply(invalid, 0, nil)...)
			_, err = conn.Write(packet)

			return err
		},
		"successful-reply-then-invalid-duplicate-status": func() error {
			packet := append([]byte(nil), success...)
			packet = append(packet, rpcReply(connect, 99, nil)...)
			_, err = conn.Write(packet)

			return err
		},
	}

	action, ok := actions[mode]
	if !ok {
		return fmt.Errorf("unknown startup failure mode %q", mode)
	}

	return action()
}

func rpcReply(call []byte, status uint32, payload []byte) []byte {
	frame := binary.BigEndian.AppendUint32(nil, uint32(28+len(payload)))
	frame = append(frame, call[:24]...)
	binary.BigEndian.PutUint32(frame[16:20], 1)
	binary.BigEndian.PutUint32(frame[24:28], status)

	return append(frame, payload...)
}

func rpcFrame(procedure, kind, serial, status uint32, payload []byte) []byte {
	frame := make([]byte, 0, 28+len(payload))
	for _, value := range []uint32{uint32(28 + len(payload)), 0x20008086, 1, procedure, kind, serial, status} {
		frame = binary.BigEndian.AppendUint32(frame, value)
	}

	return append(frame, payload...)
}

func openConsoleMustTerminate(t *testing.T, socket string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	result := make(chan error, 1)

	go func() {
		stream, err := libvirtdomain.New(socket, "qemu:///system").OpenConsole(ctx, libvirtdomain.Domain{Name: "guest", UUID: uuid.New()})
		if stream != nil {
			err = errors.Join(err, stream.Close())
		}

		result <- err
	}()

	select {
	case err := <-result:
		require.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("OpenConsole remained blocked beyond caller cancellation")
	}
}

func TestConsoleStartupErrorMatrix(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	for _, mode := range []string{
		"eof-before-reply",
		"malformed-before-reply",
		"unmatched-success",
		"failed-reply",
		"matching-invalid-header",
		"matching-unknown-status",
		"successful-reply-then-eof",
		"successful-reply-then-malformed",
		"unmatched-then-successful-then-malformed",
		"repeated-success-then-eof",
		"successful-reply-then-invalid-duplicate-header",
		"successful-reply-then-invalid-duplicate-status",
	} {
		t.Run(mode, func(t *testing.T) {
			socket, peerDone := startupFailureFixture(t, mode)
			openConsoleMustTerminate(t, socket)
			require.NoError(t, <-peerDone)
		})
	}
}

func TestSolMalformedFrameImmediatelyAfterHandshakeDoesNotHang(t *testing.T) {
	socket, peerDone := startupFailureFixture(t, "successful-reply-then-malformed")
	openConsoleMustTerminate(t, socket)
	require.NoError(t, <-peerDone)
}

func TestAstraWrongSerialConnectReplyDoesNotHang(t *testing.T) {
	socket, peerDone := startupFailureFixture(t, "unmatched-success")
	openConsoleMustTerminate(t, socket)
	require.NoError(t, <-peerDone)
}

func TestAstraMalformedHandshakeReleasesServiceSlot(t *testing.T) {
	for _, mode := range []string{
		"successful-reply-then-malformed",
		"successful-reply-then-invalid-duplicate-header",
		"successful-reply-then-invalid-duplicate-status",
	} {
		t.Run(mode, func(t *testing.T) {
			testMalformedHandshakeReleasesServiceSlot(t, mode)
		})
	}
}

func testMalformedHandshakeReleasesServiceSlot(t *testing.T, mode string) {
	const connections = 2

	socket, peerDone := startupFailureFixture(t, mode, mode)
	connector := libvirtdomain.New(socket, "qemu:///system")
	d := libvirtdomain.Domain{Name: "guest", UUID: libvirtdomain.UUID(uuid.MustParse(consoleMachineUUID), "guest")}
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	system := hardware.NewSystemInformation(hardware.SystemInformationID)
	system.TypedSpec().UUID = consoleMachineUUID
	require.NoError(t, st.Create(t.Context(), system))

	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, d.Name)
	vm.TypedSpec().Console.Serial = true
	vm.TypedSpec().PowerState = "running"
	require.NoError(t, st.Create(t.Context(), vm))

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	openDone := make(chan struct{}, connections)

	machine.RegisterHypervisorServiceServer(server, hypervisord.NewService(st, func(ctx context.Context, identity libvirtdomain.Domain) (io.ReadWriteCloser, error) {
		defer func() { openDone <- struct{}{} }()

		return connector.OpenConsole(ctx, identity)
	}))

	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()

	conn, err := grpc.NewClient(
		"passthrough:///console-malformed-handshake",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
		server.Stop()
		require.NoError(t, <-serveDone)
	})

	client := machine.NewHypervisorServiceClient(conn)

	for attempt := range connections {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		stream, streamErr := client.ConsoleStream(ctx)
		require.NoError(t, streamErr)
		require.NoError(t, stream.Send(&machine.ConsoleRequest{
			Request: &machine.ConsoleRequest_Attach{Attach: &machine.ConsoleAttach{Name: d.Name}},
		}))

		_, recvErr := stream.Recv()

		cancel()

		if status.Code(recvErr) == codes.AlreadyExists {
			t.Fatalf("failed startup retained the exclusive console slot on attempt %d", attempt)
		}

		require.Error(t, recvErr)

		select {
		case <-openDone:
		case <-time.After(time.Second):
			t.Fatalf("console connector remained blocked after caller deadline on attempt %d", attempt)
		}
	}

	require.NoError(t, <-peerDone)
}
