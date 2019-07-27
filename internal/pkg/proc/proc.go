/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package proc

import (
	"fmt"
	"io/ioutil"
	"path"
	"strings"

	"code.cloudfoundry.org/bytefmt"
	"github.com/prometheus/procfs"
)

// SystemProperty represents a kernel system property.
type SystemProperty struct {
	Key   string
	Value string
}

// WriteSystemProperty writes a value to a key under /proc/sys.
func WriteSystemProperty(prop *SystemProperty) error {
	keyPath := path.Join("/proc/sys", strings.Replace(prop.Key, ".", "/", -1))
	return ioutil.WriteFile(keyPath, []byte(prop.Value), 0644)
}

// ProcessList contains all of the process stats we want
// to display via top
type ProcessList struct {
	Pid            int
	PPID           int
	NumThreads     int
	VirtualMemory  uint64
	ResidentMemory uint64
	CPUTime        float64
	State          string
	Command        string
	Executable     string
	Args           string
}

// List processes the list of running processes and
func List() ([]ProcessList, error) {
	p, err := procfs.AllProcs()
	if err != nil {
		return nil, err
	}

	return format(p)
}

func format(procs procfs.Procs) ([]ProcessList, error) {
	pl := make([]ProcessList, 0, len(procs))
	var (
		cmdline []string
		comm    string
		err     error
		exe     string
		stats   procfs.ProcStat
	)

	for _, proc := range procs {
		comm, err = proc.Comm()
		if err != nil {
			return nil, err
		}
		exe, err = proc.Executable()
		if err != nil {
			return nil, err
		}
		cmdline, err = proc.CmdLine()
		if err != nil {
			return nil, err
		}
		stats, err = proc.Stat()
		if err != nil {
			return nil, err
		}

		p := ProcessList{
			Pid:            proc.PID,
			PPID:           stats.PPID,
			State:          stats.State,
			NumThreads:     stats.NumThreads,
			CPUTime:        stats.CPUTime(),
			VirtualMemory:  uint64(stats.VirtualMemory()),
			ResidentMemory: uint64(stats.ResidentMemory()),
			Command:        comm,
			Executable:     exe,
			Args:           strings.Join(cmdline, " "),
		}

		pl = append(pl, p)
	}

	return pl, nil
}

func (p *ProcessList) String() string {
	return fmt.Sprintf("%6d 	%1s 	%4d 	%8.2f 	%7s 	%7s 	%s 	%s", p.Pid, p.State, p.NumThreads, p.CPUTime, bytefmt.ByteSize(p.VirtualMemory), bytefmt.ByteSize(p.ResidentMemory), p.Command, p.Executable)
}
