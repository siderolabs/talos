// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package miniprocfs

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/talos-systems/talos/pkg/machinery/api/machine"
)

const (
	procsPageSize = 256
	procsBufSize  = 16 * 1024
	userHz        = 100
)

// Processes wraps iterative walker over processes under /proc.
type Processes struct {
	fd       *os.File
	dirnames []string
	idx      int

	buf      []byte
	pagesize int

	RootPath string
}

// NewProcesses initializes process info iterator with path /proc.
func NewProcesses() (*Processes, error) {
	return NewProcessesWithPath("/proc")
}

// NewProcessesWithPath initializes process info iterator with non-default path.
func NewProcessesWithPath(rootPath string) (*Processes, error) {
	procs := &Processes{
		RootPath: rootPath,
		buf:      make([]byte, procsBufSize),
	}

	var err error

	procs.fd, err = os.Open(rootPath)
	if err != nil {
		return nil, err
	}

	procs.pagesize = os.Getpagesize()

	return procs, nil
}

// Close the iterator.
func (procs *Processes) Close() error {
	return procs.fd.Close()
}

// Next returns process info until the list of processes is exhausted.
//
// Next returns nil, nil when all processes were processed.
// Next skips processes which can't be analyzed.
func (procs *Processes) Next() (*machine.ProcessInfo, error) {
	for {
		if procs.idx >= len(procs.dirnames) {
			var err error

			procs.dirnames, err = procs.fd.Readdirnames(procsPageSize)
			if err == io.EOF {
				return nil, nil
			}

			if err != nil {
				return nil, err
			}

			procs.idx = 0
		}

		info, err := procs.readProc(procs.dirnames[procs.idx])
		procs.idx++

		// if err != nil, this process was killed before we were able to read /proc data
		if err == nil {
			return info, nil
		}
	}
}

//nolint:gocyclo
func (procs *Processes) readProc(pidString string) (*machine.ProcessInfo, error) {
	pid, err := strconv.ParseInt(pidString, 10, 32)
	if err != nil {
		return nil, err
	}

	path := procs.RootPath + "/" + pidString + "/"

	executable, err := os.Readlink(path + "exe")
	if err != nil {
		return nil, err
	}

	if err = procs.readFileIntoBuf(path + "comm"); err != nil {
		return nil, err
	}

	command := string(bytes.TrimSpace(procs.buf))

	if err = procs.readFileIntoBuf(path + "cmdline"); err != nil {
		return nil, err
	}

	args := string(bytes.ReplaceAll(bytes.TrimRight(procs.buf, "\x00"), []byte{0}, []byte{' '}))

	if err = procs.readFileIntoBuf(path + "stat"); err != nil {
		return nil, err
	}

	rbracket := bytes.LastIndexByte(procs.buf, ')')
	if rbracket == -1 {
		return nil, fmt.Errorf("unexpected format")
	}

	fields := bytes.Fields(procs.buf[rbracket+2:])

	state := string(fields[0])

	ppid, err := strconv.ParseInt(string(fields[1]), 10, 32)
	if err != nil {
		return nil, err
	}

	numThreads, err := strconv.ParseInt(string(fields[17]), 10, 32)
	if err != nil {
		return nil, err
	}

	uTime, err := strconv.ParseUint(string(fields[11]), 10, 64)
	if err != nil {
		return nil, err
	}

	sTime, err := strconv.ParseUint(string(fields[12]), 10, 64)
	if err != nil {
		return nil, err
	}

	vSize, err := strconv.ParseUint(string(fields[20]), 10, 64)
	if err != nil {
		return nil, err
	}

	rss, err := strconv.ParseUint(string(fields[21]), 10, 64)
	if err != nil {
		return nil, err
	}

	return &machine.ProcessInfo{
		Pid:            int32(pid),
		Ppid:           int32(ppid),
		State:          state,
		Threads:        int32(numThreads),
		CpuTime:        float64(uTime+sTime) / userHz,
		VirtualMemory:  vSize,
		ResidentMemory: rss * uint64(procs.pagesize),
		Command:        command,
		Executable:     executable,
		Args:           args,
	}, nil
}

func (procs *Processes) readFileIntoBuf(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	procs.buf = procs.buf[:cap(procs.buf)]

	n, err := f.Read(procs.buf)
	if err != nil {
		return err
	}

	procs.buf = procs.buf[:n]

	return f.Close()
}
