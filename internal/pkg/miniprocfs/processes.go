// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package miniprocfs

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

const (
	procsBufSize  = 16 * 1024
	direntBufSize = 8 * 1024
	userHz        = 100

	// maxPIDLength is the maximum length of the PID directory name (math.MaxInt32 is 10 digits).
	maxPIDLength = 10
)

// NUL-terminated names of the files under /proc/<pid>/.
var (
	exeName         = []byte("exe\x00")
	commName        = []byte("comm\x00")
	cmdlineName     = []byte("cmdline\x00")
	statName        = []byte("stat\x00")
	attrCurrentName = []byte("attr/current\x00")
)

// buffers are reused across Processes iterators, as the Processes API is polled (e.g. by the dashboard).
type buffers struct {
	contents [procsBufSize]byte
	dirents  [direntBufSize]byte
	link     [unix.PathMax]byte
	pidName  [maxPIDLength + 1]byte
}

var buffersPool = sync.Pool{
	New: func() any {
		return new(buffers)
	},
}

// Processes wraps iterative walker over processes under /proc.
//
// The iterator doesn't allocate except for the returned process info.
type Processes struct {
	bufs *buffers

	// unparsed directory entries remaining in bufs.dirents
	dirents []byte

	// closes the fd if the iterator is not closed explicitly
	cleanup runtime.Cleanup

	fd       int
	pagesize int

	RootPath string
}

// NewProcesses initializes process info iterator with path /proc.
func NewProcesses() (*Processes, error) {
	return NewProcessesWithPath("/proc")
}

// NewProcessesWithPath initializes process info iterator with non-default path.
func NewProcessesWithPath(rootPath string) (*Processes, error) {
	fd, err := unix.Open(rootPath, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: rootPath, Err: err}
	}

	procs := &Processes{
		RootPath: rootPath,
		bufs:     buffersPool.Get().(*buffers), //nolint:forcetypeassert
		fd:       fd,
		pagesize: os.Getpagesize(),
	}

	procs.cleanup = runtime.AddCleanup(procs, closeFD, fd)

	return procs, nil
}

func closeFD(fd int) {
	unix.Close(fd) //nolint:errcheck
}

// Close the iterator.
//
// The iterator should always be closed: otherwise the directory is only closed when the iterator is garbage collected,
// and the buffers are not reused.
func (procs *Processes) Close() error {
	if procs.bufs == nil {
		return os.ErrClosed
	}

	procs.cleanup.Stop()

	buffersPool.Put(procs.bufs)
	procs.bufs = nil
	procs.dirents = nil

	return unix.Close(procs.fd)
}

// Next returns process info until the list of processes is exhausted.
//
// Next returns nil, nil when all processes were processed.
// Next skips processes which can't be analyzed.
func (procs *Processes) Next() (*machine.ProcessInfo, error) {
	if procs.bufs == nil {
		return nil, os.ErrClosed
	}

	for {
		if len(procs.dirents) == 0 {
			n, err := unix.Getdents(procs.fd, procs.bufs.dirents[:])
			if err != nil {
				return nil, &os.PathError{Op: "getdents", Path: procs.RootPath, Err: err}
			}

			if n <= 0 {
				return nil, nil
			}

			procs.dirents = procs.bufs.dirents[:n]
		}

		name, err := procs.nextDirent()
		if err != nil {
			return nil, err
		}

		pid, ok := parsePID(name)
		if !ok {
			continue
		}

		info, err := procs.readProc(pid, name)

		// if err != nil, this process was killed before we were able to read /proc data
		if err == nil {
			return info, nil
		}
	}
}

// nextDirent consumes the next directory entry and returns its name.
func (procs *Processes) nextDirent() ([]byte, error) {
	// struct linux_dirent64 { u64 d_ino; s64 d_off; u16 d_reclen; u8 d_type; char d_name[]; }
	const nameOffset = 19

	buf := procs.dirents

	if len(buf) < nameOffset {
		return nil, errors.New("malformed directory entry")
	}

	reclen := int(binary.NativeEndian.Uint16(buf[16:18]))
	if reclen < nameOffset || reclen > len(buf) {
		return nil, errors.New("malformed directory entry")
	}

	procs.dirents = buf[reclen:]

	name := buf[nameOffset:reclen]
	if i := bytes.IndexByte(name, 0); i >= 0 {
		name = name[:i]
	}

	return name, nil
}

// parsePID parses the name of the PID directory, it returns false for non-PID entries (e.g. "self").
func parsePID(name []byte) (int32, bool) {
	if len(name) == 0 || len(name) > maxPIDLength {
		return 0, false
	}

	var pid int64

	for _, c := range name {
		if c < '0' || c > '9' {
			return 0, false
		}

		pid = pid*10 + int64(c-'0')
	}

	if pid > math.MaxInt32 {
		return 0, false
	}

	return int32(pid), true
}

func (procs *Processes) readProc(pid int32, name []byte) (*machine.ProcessInfo, error) {
	bufs := procs.bufs

	pidName := append(bufs.pidName[:0], name...)
	pidName = append(pidName, 0)

	dirfd, err := openat(procs.fd, pidName, unix.O_PATH|unix.O_DIRECTORY)
	if err != nil {
		return nil, err
	}

	defer unix.Close(dirfd) //nolint:errcheck

	n, err := readlinkat(dirfd, exeName, bufs.link[:])
	if err != nil {
		return nil, err
	}

	executable := string(bufs.link[:n])

	contents, err := procs.readFile(dirfd, commName)
	if err != nil {
		return nil, err
	}

	command := string(bytes.TrimSpace(contents))

	contents, err = procs.readFile(dirfd, cmdlineName)
	if err != nil {
		return nil, err
	}

	// arguments are NUL-separated
	contents = bytes.TrimRight(contents, "\x00")

	for i, c := range contents {
		if c == 0 {
			contents[i] = ' '
		}
	}

	args := string(contents)

	contents, err = procs.readFile(dirfd, statName)
	if err != nil {
		return nil, err
	}

	stat, err := parseStat(contents)
	if err != nil {
		return nil, err
	}

	var label string

	if contents, err = procs.readFile(dirfd, attrCurrentName); err == nil {
		label = string(bytes.Trim(contents, "\x00\n"))
	}

	return &machine.ProcessInfo{
		Pid:            pid,
		Ppid:           stat.ppid,
		State:          stat.state,
		Threads:        stat.numThreads,
		CpuTime:        float64(stat.uTime+stat.sTime) / userHz,
		VirtualMemory:  stat.vSize,
		ResidentMemory: stat.rss * uint64(procs.pagesize),
		Command:        command,
		Executable:     executable,
		Args:           args,
		Label:          label,
	}, nil
}

// readFile reads the file (with a single read) into the contents buffer.
//
// The returned slice is valid until the next call.
func (procs *Processes) readFile(dirfd int, name []byte) ([]byte, error) {
	fd, err := openat(dirfd, name, unix.O_RDONLY)
	if err != nil {
		return nil, err
	}

	defer unix.Close(fd) //nolint:errcheck

	n, err := unix.Read(fd, procs.bufs.contents[:])
	if err != nil {
		return nil, err
	}

	return procs.bufs.contents[:n], nil
}

type procStat struct {
	state                    string
	uTime, sTime, vSize, rss uint64
	ppid, numThreads         int32
}

// parseStat parses the /proc/<pid>/stat contents.
//
// See proc_pid_stat(5) for the format.
func parseStat(contents []byte) (procStat, error) {
	// the command (2nd field) might contain any characters, so skip to the closing bracket
	_, rest, ok := bytes.CutLast(contents, []byte(")"))
	if !ok {
		return procStat{}, errors.New("unexpected format")
	}

	// fields starting with the 3rd one (state), up to the 24th one (rss)
	var (
		fields [22][]byte
		n      int
	)

	for field := range bytes.FieldsSeq(rest) {
		fields[n] = field
		n++

		if n == len(fields) {
			break
		}
	}

	if n < len(fields) {
		return procStat{}, errors.New("unexpected format")
	}

	ppid, errPPID := parseInt32(fields[1])
	uTime, errUTime := parseUint64(fields[11])
	sTime, errSTime := parseUint64(fields[12])
	numThreads, errNumThreads := parseInt32(fields[17])
	vSize, errVSize := parseUint64(fields[20])
	rss, errRSS := parseUint64(fields[21])

	if err := cmp.Or(errPPID, errUTime, errSTime, errNumThreads, errVSize, errRSS); err != nil {
		return procStat{}, err
	}

	return procStat{
		state:      string(fields[0]),
		ppid:       ppid,
		uTime:      uTime,
		sTime:      sTime,
		numThreads: numThreads,
		vSize:      vSize,
		rss:        rss,
	}, nil
}

var errInvalidNumber = errors.New("invalid number")

func parseUint64(b []byte) (uint64, error) {
	if len(b) == 0 {
		return 0, errInvalidNumber
	}

	var v uint64

	for _, c := range b {
		if c < '0' || c > '9' {
			return 0, errInvalidNumber
		}

		d := uint64(c - '0')

		if v > (math.MaxUint64-d)/10 {
			return 0, errInvalidNumber
		}

		v = v*10 + d
	}

	return v, nil
}

func parseInt32(b []byte) (int32, error) {
	neg := len(b) > 0 && b[0] == '-'
	if neg {
		b = b[1:]
	}

	v, err := parseUint64(b)
	if err != nil {
		return 0, err
	}

	if neg {
		if v > -math.MinInt32 {
			return 0, errInvalidNumber
		}

		return int32(-int64(v)), nil
	}

	if v > math.MaxInt32 {
		return 0, errInvalidNumber
	}

	return int32(v), nil
}

// openat opens a file by a NUL-terminated path relative to dirfd.
//
// Unlike unix.Openat, it doesn't allocate a copy of the path on each call.
func openat(dirfd int, path []byte, flags int) (int, error) {
	fd, _, errno := unix.Syscall6(
		unix.SYS_OPENAT,
		uintptr(dirfd),
		uintptr(unsafe.Pointer(unsafe.SliceData(path))),
		uintptr(flags|unix.O_CLOEXEC),
		0, 0, 0,
	)
	if errno != 0 {
		return -1, errno
	}

	return int(fd), nil
}

// readlinkat reads the symlink target by a NUL-terminated path relative to dirfd into buf.
//
// Unlike unix.Readlinkat, it doesn't allocate a copy of the path on each call.
func readlinkat(dirfd int, path, buf []byte) (int, error) {
	n, _, errno := unix.Syscall6(
		unix.SYS_READLINKAT,
		uintptr(dirfd),
		uintptr(unsafe.Pointer(unsafe.SliceData(path))),
		uintptr(unsafe.Pointer(unsafe.SliceData(buf))),
		uintptr(len(buf)),
		0, 0,
	)
	if errno != 0 {
		return 0, errno
	}

	return int(n), nil
}
