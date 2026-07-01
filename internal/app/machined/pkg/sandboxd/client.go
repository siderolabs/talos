// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sandboxd

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
)

// Client is machined's launcher for the shared sandbox PID+mount namespace. It
// talks to sandboxd over a control socket; each launch runs inside the
// namespace as a child of the init (no setns). It is published by the sandboxd
// runner and replaced whenever the namespace is recreated.
// All methods are safe for concurrent use.
type Client struct {
	control *net.UnixConn // control socket to sandboxd
	mu      sync.Mutex    // serializes launch requests on the shared control socket
}

// close releases the control socket.
func (c *Client) close() {
	c.control.Close() //nolint:errcheck
}

// Launch asks sandboxd to fork a service inside the sandbox namespace.
// It sends the request plus [stdin, stdout, stderr, per-service socket] over the
// control socket, then receives the service's pidfd over the per-service socket.
func (c *Client) Launch(cfg runtime.LaunchConfig) (runtime.SandboxHandle, error) { //nolint:gocyclo
	stdin := cfg.Stdin

	var devNull *os.File

	if stdin == nil {
		var err error

		devNull, err = os.Open(os.DevNull)
		if err != nil {
			return nil, fmt.Errorf("opening /dev/null: %w", err)
		}

		defer devNull.Close() //nolint:errcheck

		stdin = devNull
	}

	// Per-service socket: init returns the pidfd and (later) the exit status on it;
	// its EOF also tells machined the namespace was lost.
	svcFds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_SEQPACKET|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("creating service socketpair: %w", err)
	}

	svcMachined := os.NewFile(uintptr(svcFds[0]), "svc-machined")
	svcInit := os.NewFile(uintptr(svcFds[1]), "svc-init")

	defer svcInit.Close() //nolint:errcheck

	var reqBuf bytes.Buffer

	if err := gob.NewEncoder(&reqBuf).Encode(launchRequest{
		Args:                cfg.Args,
		Env:                 cfg.Env,
		DroppedCapabilities: cfg.DroppedCapabilities,
		SelinuxLabel:        cfg.SelinuxLabel,
	}); err != nil {
		svcMachined.Close() //nolint:errcheck

		return nil, fmt.Errorf("encoding launch request: %w", err)
	}

	rights := syscall.UnixRights(int(stdin.Fd()), int(cfg.Stdout.Fd()), int(cfg.Stderr.Fd()), int(svcInit.Fd()))

	c.mu.Lock()
	_, _, err = c.control.WriteMsgUnix(reqBuf.Bytes(), rights, nil)
	c.mu.Unlock()

	if err != nil {
		svcMachined.Close() //nolint:errcheck

		return nil, fmt.Errorf("sending launch request: %w", err)
	}

	svcConn, err := unixConn(svcMachined)
	if err != nil {
		return nil, fmt.Errorf("wrapping service socket: %w", err)
	}

	// Receive launchResponse + pidfd via SCM_RIGHTS.
	respBuf := make([]byte, 4096)
	oob := make([]byte, syscall.CmsgSpace(4))

	n, oobn, _, _, err := svcConn.ReadMsgUnix(respBuf, oob)
	if err != nil {
		svcConn.Close() //nolint:errcheck

		return nil, fmt.Errorf("reading launch response: %w", err)
	}

	var resp launchResponse

	if err := gob.NewDecoder(bytes.NewReader(respBuf[:n])).Decode(&resp); err != nil {
		svcConn.Close() //nolint:errcheck

		return nil, fmt.Errorf("decoding launch response: %w", err)
	}

	if resp.Err != "" {
		svcConn.Close() //nolint:errcheck

		return nil, fmt.Errorf("sandbox launch: %s", resp.Err)
	}

	scms, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil || len(scms) == 0 {
		svcConn.Close() //nolint:errcheck

		return nil, fmt.Errorf("parsing SCM_RIGHTS from launch response: %w", err)
	}

	pidfds, err := syscall.ParseUnixRights(&scms[0])
	if err != nil || len(pidfds) == 0 {
		svcConn.Close() //nolint:errcheck

		return nil, fmt.Errorf("extracting pidfd from launch response: %w", err)
	}

	hostPID, err := hostPIDFromPidfd(pidfds[0])
	if err != nil {
		svcConn.Close()          //nolint:errcheck
		syscall.Close(pidfds[0]) //nolint:errcheck

		return nil, fmt.Errorf("resolving host PID from pidfd: %w", err)
	}

	return &ServiceHandle{conn: svcConn, hostPID: hostPID, pidfd: pidfds[0]}, nil
}

func writeFull(file *os.File, data []byte) error {
	for len(data) > 0 {
		n, err := file.Write(data)
		if err != nil {
			return err
		}

		if n == 0 {
			return io.ErrShortWrite
		}

		data = data[n:]
	}

	return nil
}

// hostPIDFromPidfd reads /proc/self/fdinfo/<pidfd> to obtain the host-namespace
// PID of the process referenced by pidfd. The "Pid:" field in fdinfo is resolved
// in the reader's PID namespace (machined = host), so the value is the host PID
// regardless of which namespace opened the pidfd.
func hostPIDFromPidfd(pidfd int) (int, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/self/fdinfo/%d", pidfd))
	if err != nil {
		return 0, fmt.Errorf("reading fdinfo for pidfd %d: %w", pidfd, err)
	}

	for _, line := range strings.SplitN(string(data), "\n", 20) {
		if !strings.HasPrefix(line, "Pid:") {
			continue
		}

		pidStr := strings.TrimSpace(strings.TrimPrefix(line, "Pid:"))

		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			return 0, fmt.Errorf("parsing Pid from fdinfo: %w", err)
		}

		if pid == -1 {
			return 0, fmt.Errorf("process already exited (Pid: -1 in fdinfo)")
		}

		return pid, nil
	}

	return 0, fmt.Errorf("pid field not found in /proc/self/fdinfo/%d", pidfd)
}

// ServiceHandle is returned by Client.Launch and lets the caller signal and wait
// for the service process.
type ServiceHandle struct {
	conn    *net.UnixConn
	hostPID int
	pidfd   int
}

// HostPID returns the host-namespace PID of the launched service.
func (h *ServiceHandle) HostPID() int {
	return h.hostPID
}

// Signal sends sig to the launched service via its pidfd.
func (h *ServiceHandle) Signal(sig syscall.Signal) error {
	return unix.PidfdSendSignal(h.pidfd, sig, nil, 0)
}

// Wait asks sandboxd to report the service's exit and blocks until it
// does. A closed connection (e.g. the namespace was torn down) returns an error,
// which drives the service's restart into the recreated namespace.
func (h *ServiceHandle) Wait() (int, error) {
	defer h.conn.Close() //nolint:errcheck

	if _, err := h.conn.Write([]byte{0}); err != nil {
		h.kill()

		return -1, fmt.Errorf("sending wait signal: %w", err)
	}

	var buf [256]byte

	n, err := h.conn.Read(buf[:])
	if err != nil {
		h.kill()

		return -1, fmt.Errorf("reading wait response: %w", err)
	}

	var resp waitResponse

	if err := gob.NewDecoder(bytes.NewReader(buf[:n])).Decode(&resp); err != nil {
		return -1, fmt.Errorf("decoding wait response: %w", err)
	}

	if resp.ExitErr != "" {
		return resp.ExitCode, fmt.Errorf("%s", resp.ExitErr)
	}

	return resp.ExitCode, nil
}

func (h *ServiceHandle) kill() {
	if h.pidfd >= 0 {
		unix.PidfdSendSignal(h.pidfd, syscall.SIGKILL, nil, 0) //nolint:errcheck
	}
}

// Close releases the pidfd held by the handle.
func (h *ServiceHandle) Close() error {
	if h.pidfd < 0 {
		return nil
	}

	err := syscall.Close(h.pidfd)
	h.pidfd = -1

	return err
}
