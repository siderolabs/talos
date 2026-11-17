// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logind

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/cgroup"
	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

const authTimeout = 10 * time.Second

// DBusBroker implements simplified D-Bus broker which allows to connect
// kubelet D-Bus connection with Talos logind mock.
//
// Broker doesn't actually implement auth, and it is supposed that service
// connects on one socket, while a client connects on another socket.
// The service must be this process, and the client must run in clientCgroup.
type DBusBroker struct {
	listenService, listenClient net.Listener
	clientCgroup                string
}

// NewBroker initializes new broker.
//
// clientCgroup is the cgroup of the client as /proc/<pid>/cgroup shows it.
func NewBroker(ctx context.Context, serviceSocketPath, clientSocketPath, clientCgroup string) (*DBusBroker, error) {
	// remove socket paths as with Docker mode paths might persist across container restarts
	for _, socketPath := range []string{serviceSocketPath, clientSocketPath} {
		if err := os.RemoveAll(socketPath); err != nil {
			return nil, fmt.Errorf("error cleaning up D-Bus socket paths: %w", err)
		}
	}

	broker := &DBusBroker{
		clientCgroup: clientCgroup,
	}

	var err error

	broker.listenService, err = (&net.ListenConfig{}).Listen(ctx, "unix", serviceSocketPath)
	if err != nil {
		return nil, err
	}

	if err = selinux.SetLabel(serviceSocketPath, constants.DBusServiceSocketLabel); err != nil {
		return nil, err
	}

	broker.listenClient, err = (&net.ListenConfig{}).Listen(ctx, "unix", clientSocketPath)
	if err != nil {
		return nil, err
	}

	if err = selinux.SetLabel(clientSocketPath, constants.DBusClientSocketLabel); err != nil {
		return nil, err
	}

	return broker, nil
}

// Close the listen sockets.
func (broker *DBusBroker) Close() error {
	if err := broker.listenClient.Close(); err != nil {
		return err
	}

	return broker.listenService.Close()
}

// Run the broker.
func (broker *DBusBroker) Run(ctx context.Context) error {
	eg, ctx := errgroup.WithContext(ctx)

	var (
		connClient, connService net.Conn
		mu                      sync.Mutex
	)

	eg.Go(func() error {
		return broker.run(ctx, broker.listenService, checkService, &mu, &connService, &connClient)
	})
	eg.Go(func() error {
		return broker.run(ctx, broker.listenClient, broker.checkClient, &mu, &connClient, &connService)
	})

	return eg.Wait()
}

func checkService(pid int32) error {
	if int(pid) != os.Getpid() {
		return errors.New("it is not the logind mock")
	}

	return nil
}

func (broker *DBusBroker) checkClient(pid int32) error {
	return cgroup.CheckMembership(fmt.Sprintf("/proc/%d/cgroup", pid), broker.clientCgroup)
}

// verifyPeer runs checkPeer on the PID of the process which connected conn, in the PID namespace of this process.
func verifyPeer(conn *net.UnixConn, checkPeer func(pid int32) error) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}

	var (
		cred    *unix.Ucred
		credErr error
	)

	if err = raw.Control(func(fd uintptr) {
		cred, credErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		return err
	}

	if credErr != nil {
		return credErr
	}

	if err = checkPeer(cred.Pid); err != nil {
		return fmt.Errorf("PID %d: %w", cred.Pid, err)
	}

	return nil
}

func (broker *DBusBroker) run(ctx context.Context, l net.Listener, checkPeer func(pid int32) error, mu *sync.Mutex, ours, theirs *net.Conn) error {
	var wg sync.WaitGroup

	defer wg.Wait()

	for ctx.Err() == nil {
		conn, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}

			return err
		}

		wg.Go(func() { handleConn(ctx, conn.(*net.UnixConn), checkPeer, mu, ours, theirs) })
	}

	return nil
}

func extractFiles(oob []byte) []int {
	if len(oob) == 0 {
		return nil
	}

	var fds []int

	scms, err := syscall.ParseSocketControlMessage(oob)
	if err == nil {
		for _, scm := range scms {
			var files []int

			files, err = syscall.ParseUnixRights(&scm)
			if err == nil {
				fds = append(fds, files...)
			}
		}
	}

	return fds
}

//nolint:gocyclo
func handleConn(ctx context.Context, conn *net.UnixConn, checkPeer func(pid int32) error, mu *sync.Mutex, ours, theirs *net.Conn) {
	defer conn.Close() //nolint: errcheck

	if err := verifyPeer(conn, checkPeer); err != nil {
		log.Printf("rejected D-Bus peer: %s", err)

		return
	}

	// unblock the reads below on shutdown
	stop := context.AfterFunc(ctx, func() { conn.Close() }) //nolint:errcheck
	defer stop()

	r := bufio.NewReader(conn)

	if err := conn.SetDeadline(time.Now().Add(authTimeout)); err != nil {
		return
	}

	if err := handleAuth(r, conn); err != nil {
		log.Printf("auth failed: %s", err)

		return
	}

	if err := conn.SetDeadline(time.Time{}); err != nil {
		return
	}

	mu.Lock()

	// the newest connection wins, so a restarted kubelet replaces a peer which never disconnected
	if *ours != nil {
		(*ours).Close() //nolint:errcheck
	}

	*ours = conn

	mu.Unlock()

	defer func() {
		mu.Lock()

		if *ours == net.Conn(conn) {
			*ours = nil
		}

		mu.Unlock()
	}()

	buf := make([]byte, 4096)
	oob := make([]byte, 4096)

	for ctx.Err() == nil {
		var (
			n, oobn int
			err     error
		)

		if r.Buffered() > 0 {
			// read remaining buffered data
			n, err = r.Read(buf[:r.Buffered()])
		} else {
			// read the message and OOB data from the UNIX socket
			n, oobn, _, _, err = conn.ReadMsgUnix(buf, oob)
		}

		if err != nil {
			return
		}

		// capture all file descriptors in the OOB message
		// broker needs to close the file descriptors as they get passed to the other peer
		fds := extractFiles(oob[:oobn])

		// find the other side of the connection
		var w net.Conn

		for range 10 {
			mu.Lock()

			w = *theirs

			mu.Unlock()

			if w != nil {
				break
			}

			select {
			case <-time.After(time.Second):
			case <-ctx.Done():
				return
			}
		}

		if w == nil {
			// drop data, as there's no other connection
			continue
		}

		// send the message and OOB date
		// this will pass the file descriptors if they are in the OOB date
		if _, _, err = w.(*net.UnixConn).WriteMsgUnix(buf[:n], oob[:oobn], nil); err != nil {
			return
		}

		// close fds to make sure broker doesn't hold the fds on its side
		for _, fd := range fds {
			syscall.Close(fd) //nolint:errcheck
		}
	}
}

//nolint:gocyclo
func handleAuth(r *bufio.Reader, w io.Writer) error {
	readLine := func() (string, error) {
		l, err := r.ReadString('\n')
		if err != nil {
			return l, err
		}

		l = strings.TrimRight(l, "\r\n")

		return l, nil
	}

	// first, should receive AUTH command preceded by zero byte
	line, err := readLine()
	if err != nil {
		return err
	}

	if line != "\x00AUTH" {
		return fmt.Errorf("unexpected line, expected AUTH: %q", line)
	}

	if _, err = w.Write([]byte("REJECTED EXTERNAL\r\n")); err != nil {
		return err
	}

	// now real auth command
	line, err = readLine()
	if err != nil {
		return err
	}

	if !strings.HasPrefix(line, "AUTH EXTERNAL") {
		return fmt.Errorf("unexpected line, expected AUTH EXTERNAL: %q", line)
	}

	if _, err = w.Write([]byte("OK 1234deadbeef\r\n")); err != nil {
		return err
	}

	// negotiate unix FDs
	line, err = readLine()
	if err != nil {
		return err
	}

	if line != "NEGOTIATE_UNIX_FD" {
		return fmt.Errorf("unexpected line, expected NEGOTIATE_UNIX_FD: %q", line)
	}

	if _, err = w.Write([]byte("AGREE_UNIX_FD\r\n")); err != nil {
		return err
	}

	// BEGIN
	line, err = readLine()
	if err != nil {
		return err
	}

	if line != "BEGIN" {
		return fmt.Errorf("unexpected line, expected BEGIN: %q", line)
	}

	return nil
}
