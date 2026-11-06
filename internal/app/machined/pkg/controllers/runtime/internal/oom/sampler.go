// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package oom

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/cgroups"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const (
	numPSITypes     = 2 // some, full
	numPSISpans     = 4 // avg10, avg60, avg300, total
	numMemoryParams = 3 // current, max, peak
	numQoSClasses   = int(runtime.QoSCgroupClassSystem) + 1

	// rootClass marks the root cgroup, which feeds the non-QoS metrics.
	rootClass = runtime.QoSCgroupClass(-1)

	initialReadBufferSize   = 512
	initialDirentBufferSize = 8192
)

var (
	psiTypeNames     = [numPSITypes]string{"some", "full"}
	psiSpanNames     = [numPSISpans]string{"avg10", "avg60", "avg300", "total"}
	memoryParamNames = [numMemoryParams]string{"current", "max", "peak"}
	memoryFiles      = [numMemoryParams]string{"memory.current", "memory.max", "memory.peak"}

	// evaluation context keys, precomputed to avoid allocating them on each sample.
	psiKeys, dPSIKeys, qosPSIKeys, dQoSPSIKeys [numPSITypes][numPSISpans]string
	qosMemoryKeys, dQoSMemoryKeys              [numMemoryParams]string
)

func init() {
	for t, psiType := range psiTypeNames {
		for s, span := range psiSpanNames {
			name := "memory_" + psiType + "_" + span

			psiKeys[t][s] = name
			dPSIKeys[t][s] = "d_" + name
			qosPSIKeys[t][s] = "qos_" + name
			dQoSPSIKeys[t][s] = "d_qos_" + name
		}
	}

	for p, param := range memoryParamNames {
		qosMemoryKeys[p] = "qos_memory_" + param
		dQoSMemoryKeys[p] = "d_qos_memory_" + param
	}
}

type psiValues [numPSITypes][numPSISpans]float64

type qosValues [numQoSClasses]float64

// cgroupState is the state of a single sampled cgroup.
type cgroupState struct {
	// path is the cgroup path relative to the root.
	path string

	// name is the cgroup directory name, set only for Guaranteed pod cgroups.
	name string

	pressureFile cgroupFile
	memoryFiles  [numMemoryParams]cgroupFile

	prevPSI    psiValues
	prevMemory [numMemoryParams]float64

	// generation is the generation of the last refresh which listed the Guaranteed pod cgroup.
	generation uint64

	class runtime.QoSCgroupClass

	hasPrevPSI, hasPrevMemory bool
}

func newCgroupState(root, path, name string, class runtime.QoSCgroupClass) *cgroupState {
	dir := filepath.Join(root, path)

	cg := &cgroupState{
		path:         path,
		name:         name,
		class:        class,
		pressureFile: newCgroupFile(filepath.Join(dir, "memory.pressure")),
	}

	for p, file := range memoryFiles {
		cg.memoryFiles[p] = newCgroupFile(filepath.Join(dir, file))
	}

	return cg
}

// close closes all files of the cgroup.
func (cg *cgroupState) close() {
	cg.pressureFile.close()

	for p := range cg.memoryFiles {
		cg.memoryFiles[p].close()
	}
}

// cgroupFile is a cgroupfs file which is kept open across samples.
//
// Opening a file requires a path walk, which is the most expensive part of reading a cgroupfs file,
// so the file is opened once and re-read with pread at offset zero: cgroupfs (kernfs) regenerates
// the file contents on each read from offset zero.
//
// Once the cgroup is removed, reads from an open file fail with ENODEV, so the file is re-opened by the path
// (in case the cgroup was re-created with the same path).
type cgroupFile struct {
	// NUL-terminated path.
	path []byte

	// fd is -1 if the file is not open.
	fd int
}

func newCgroupFile(path string) cgroupFile {
	return cgroupFile{
		path: cString(path),
		fd:   -1,
	}
}

func (f *cgroupFile) open() bool {
	fd, err := openPath(f.path, unix.O_RDONLY)
	if err != nil {
		return false
	}

	f.fd = fd

	return true
}

func (f *cgroupFile) close() {
	if f.fd < 0 {
		return
	}

	unix.Close(f.fd) //nolint:errcheck

	f.fd = -1
}

// Sampler samples memory pressure (PSI) and memory usage of the cgroups used in the OOM trigger expression.
//
// The sampled cgroups are:
//   - the root cgroup (memory_* and d_memory_* variables);
//   - QoS class cgroups and each Guaranteed pod cgroup (qos_memory_* and d_qos_memory_* variables).
//
// Sampler runs every OOM sample interval (500ms by default), so it reuses all buffers and paths
// across samples: it doesn't allocate unless the set of Guaranteed pods changes.
//
// Sampler is not safe for concurrent use.
type Sampler struct {
	root string

	readBuf   []byte
	direntBuf []byte

	kubepodsPath []byte

	fixed      []*cgroupState
	guaranteed []*cgroupState
	scratch    []*cgroupState

	generation uint64

	evalContext map[string]any

	// results of the last sample
	psi, dPSI          psiValues
	qosPSI, dQoSPSI    [numPSITypes][numPSISpans]qosValues
	qosMemory, dQoSMem [numMemoryParams]qosValues
}

// NewSampler creates a new Sampler for the cgroup hierarchy at root.
func NewSampler(root string) *Sampler {
	return &Sampler{
		root:         root,
		readBuf:      make([]byte, initialReadBufferSize),
		direntBuf:    make([]byte, initialDirentBufferSize),
		kubepodsPath: cString(filepath.Join(root, constants.CgroupKubepods)),
		fixed: []*cgroupState{
			newCgroupState(root, "", "", rootClass),
			newCgroupState(root, constants.CgroupInit, "", runtime.QoSCgroupClassSystem),
			newCgroupState(root, constants.CgroupSystem, "", runtime.QoSCgroupClassSystem),
			newCgroupState(root, constants.CgroupPodRuntimeRoot, "", runtime.QoSCgroupClassPodruntime),
			newCgroupState(root, constants.CgroupKubepods+"/"+cgroupBesteffort, "", runtime.QoSCgroupClassBesteffort),
			newCgroupState(root, constants.CgroupKubepods+"/"+cgroupBurstable, "", runtime.QoSCgroupClassBurstable),
		},
	}
}

// Close closes all cgroupfs files kept open by the Sampler.
//
// The Sampler can still be used after Close, it re-opens the files on the next sample.
func (s *Sampler) Close() {
	for _, cg := range s.fixed {
		cg.close()
	}

	for _, cg := range s.guaranteed {
		cg.close()
	}
}

// Sample reads the current values from cgroupfs.
//
// The rate of change (d_* variables) is calculated against the previous sample of the same cgroup
// over the sampleInterval; it is zero on the first sample of a cgroup.
func (s *Sampler) Sample(sampleInterval time.Duration) error {
	if sampleInterval <= 0 {
		return errors.New("sample interval must be greater than zero")
	}

	s.refreshGuaranteed()

	s.psi, s.dPSI = psiValues{}, psiValues{}
	s.qosPSI, s.dQoSPSI = [numPSITypes][numPSISpans]qosValues{}, [numPSITypes][numPSISpans]qosValues{}
	s.qosMemory, s.dQoSMem = [numMemoryParams]qosValues{}, [numMemoryParams]qosValues{}

	seconds := sampleInterval.Seconds()

	for _, cg := range s.fixed {
		if err := s.sampleCgroup(cg, seconds); err != nil {
			return err
		}
	}

	for _, cg := range s.guaranteed {
		if err := s.sampleCgroup(cg, seconds); err != nil {
			return err
		}
	}

	return nil
}

// EvalContext returns the evaluation context for the OOM trigger expression filled with the values of the last sample.
//
// The returned map (including nested maps) is owned by the Sampler, and it is overwritten on the next call.
// The caller might add more variables to the map, they are preserved across calls.
func (s *Sampler) EvalContext() map[string]any {
	if s.evalContext == nil {
		s.evalContext = make(map[string]any, 1+4*numPSITypes*numPSISpans+2*numMemoryParams)
	}

	for t := range numPSITypes {
		for sp := range numPSISpans {
			s.evalContext[psiKeys[t][sp]] = s.psi[t][sp]
			s.evalContext[dPSIKeys[t][sp]] = s.dPSI[t][sp]

			s.setQoS(qosPSIKeys[t][sp], &s.qosPSI[t][sp])
			s.setQoS(dQoSPSIKeys[t][sp], &s.dQoSPSI[t][sp])
		}
	}

	for p := range numMemoryParams {
		s.setQoS(qosMemoryKeys[p], &s.qosMemory[p])
		s.setQoS(dQoSMemoryKeys[p], &s.dQoSMem[p])
	}

	return s.evalContext
}

// setQoS updates the per-class map in the evaluation context, reusing the map from the previous call.
//
// Every QoS class is always present in the map, even if no cgroup of that class exists (e.g. no Guaranteed pods).
func (s *Sampler) setQoS(key string, values *qosValues) {
	m, ok := s.evalContext[key].(map[int]float64)
	if !ok {
		m = make(map[int]float64, numQoSClasses)
		s.evalContext[key] = m
	}

	for _, class := range qosClasses {
		m[int(class)] = values[class]
	}
}

func (s *Sampler) sampleCgroup(cg *cgroupState, seconds float64) error {
	// Default non-existent cgroups to all-zero, e.g. during system boot.
	var current psiValues

	if contents, ok := s.readFile(&cg.pressureFile); ok {
		var err error

		current, err = parsePSI(contents)
		if err != nil {
			return err
		}
	}

	for t := range numPSITypes {
		for sp := range numPSISpans {
			value := current[t][sp]

			var diff float64
			if cg.hasPrevPSI {
				diff = (value - cg.prevPSI[t][sp]) / seconds
			}

			if cg.class == rootClass {
				s.psi[t][sp] = value
				s.dPSI[t][sp] = diff
			} else {
				s.qosPSI[t][sp][cg.class] += value
				s.dQoSPSI[t][sp][cg.class] += diff
			}
		}
	}

	cg.prevPSI = current
	cg.hasPrevPSI = true

	if cg.class != rootClass {
		// memory usage is only tracked per QoS class
		s.sampleMemory(cg, seconds)
	}

	return nil
}

func (s *Sampler) sampleMemory(cg *cgroupState, seconds float64) {
	for p := range numMemoryParams {
		// Best effort: if the file is not present, the value is unset, and it's reported as zero.
		var value cgroups.Value

		if contents, ok := s.readFile(&cg.memoryFiles[p]); ok {
			value = parseSingleValue(contents)
		}

		fvalue := value.Float64()

		// These values cannot be expressed in JSON
		if math.IsNaN(fvalue) || math.IsInf(fvalue, 0) {
			fvalue = 0
		}

		var diff float64
		if cg.hasPrevMemory {
			diff = (fvalue - cg.prevMemory[p]) / seconds
		}

		s.qosMemory[p][cg.class] += fvalue
		s.dQoSMem[p][cg.class] += diff

		cg.prevMemory[p] = fvalue
	}

	cg.hasPrevMemory = true
}

// refreshGuaranteed updates the list of Guaranteed pod cgroups.
//
// Guaranteed pods live directly under the kubepods cgroup, next to the besteffort/burstable
// class cgroups (which are excluded here). The state of the cgroups which are still present is kept,
// so that the rate of change can be calculated; the state of the removed cgroups is dropped (and their files are closed).
func (s *Sampler) refreshGuaranteed() {
	// drop the references to the states from the sample before the previous one
	clear(s.scratch)
	s.scratch = s.scratch[:0]

	s.generation++

	fd, err := openPath(s.kubepodsPath, unix.O_RDONLY|unix.O_DIRECTORY)
	if err == nil {
		s.scratch = s.readGuaranteed(fd, s.scratch)

		unix.Close(fd) //nolint:errcheck
	}

	// Getdents doesn't return entries in a stable order, but sum order should be deterministic.
	slices.SortFunc(s.scratch, func(a, b *cgroupState) int {
		return strings.Compare(a.name, b.name)
	})

	for _, cg := range s.guaranteed {
		if cg.generation != s.generation {
			cg.close()
		}
	}

	s.guaranteed, s.scratch = s.scratch, s.guaranteed
}

func (s *Sampler) readGuaranteed(fd int, out []*cgroupState) []*cgroupState {
	for {
		n, err := unix.Getdents(fd, s.direntBuf)
		if err != nil {
			if errors.Is(err, unix.EINVAL) {
				// buffer is too small for the next entry
				s.direntBuf = make([]byte, 2*len(s.direntBuf))

				continue
			}

			return out
		}

		if n <= 0 {
			return out
		}

		for buf := s.direntBuf[:n]; len(buf) > 0; {
			// struct linux_dirent64 { u64 d_ino; s64 d_off; u16 d_reclen; u8 d_type; char d_name[]; }
			const nameOffset = 19

			reclen := int(binary.NativeEndian.Uint16(buf[16:18]))
			if reclen < nameOffset || reclen > len(buf) {
				return out
			}

			name := buf[nameOffset:reclen]
			if i := bytes.IndexByte(name, 0); i >= 0 {
				name = name[:i]
			}

			if s.isGuaranteedDir(fd, name, buf[18]) {
				out = append(out, s.guaranteedState(name))
			}

			buf = buf[reclen:]
		}
	}
}

func (s *Sampler) isGuaranteedDir(fd int, name []byte, typ byte) bool {
	switch string(name) {
	case ".", "..", cgroupBesteffort, cgroupBurstable:
		return false
	}

	switch typ {
	case unix.DT_DIR:
		return true
	case unix.DT_UNKNOWN:
		// cgroupfs always reports the type, but some filesystems (used in tests) might not
		var st unix.Stat_t

		if err := unix.Fstatat(fd, string(name), &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return false
		}

		return st.Mode&unix.S_IFMT == unix.S_IFDIR
	default:
		return false
	}
}

// guaranteedState returns the state of the Guaranteed pod cgroup, creating it if the cgroup was not seen in the previous sample.
func (s *Sampler) guaranteedState(name []byte) *cgroupState {
	for _, cg := range s.guaranteed {
		if cg.name == string(name) {
			cg.generation = s.generation

			return cg
		}
	}

	nameStr := string(name)

	cg := newCgroupState(s.root, constants.CgroupKubepods+"/"+nameStr, nameStr, runtime.QoSCgroupClassGuaranteed)
	cg.generation = s.generation

	return cg
}

// readFile reads the whole (small) file into the read buffer, opening the file if needed.
//
// The returned slice is valid until the next call; ok is false if the file can't be read.
func (s *Sampler) readFile(f *cgroupFile) (contents []byte, ok bool) {
	reopened := false

	for {
		if f.fd < 0 {
			if !f.open() {
				return nil, false
			}

			reopened = true
		}

		contents, err := s.pread(f.fd)
		if err == nil {
			return contents, true
		}

		// the cgroup was removed (ENODEV), but it might have been re-created, so retry once with a freshly opened file
		f.close()

		if reopened {
			return nil, false
		}
	}
}

// pread reads the whole file from offset zero into the read buffer with a single read.
//
// cgroupfs files are generated in full on each read from offset zero, so a short read means the end of the file.
// If the buffer is filled completely, the file is re-read into a bigger buffer, so that the contents are consistent.
func (s *Sampler) pread(fd int) ([]byte, error) {
	for {
		n, err := unix.Pread(fd, s.readBuf, 0)
		if err != nil {
			return nil, err
		}

		if n < len(s.readBuf) {
			return s.readBuf[:n], nil
		}

		s.readBuf = make([]byte, 2*len(s.readBuf))
	}
}

// parsePSI parses the PSI file contents:
//
//	some avg10=0.00 avg60=0.00 avg300=0.00 total=0
//	full avg10=0.00 avg60=0.00 avg300=0.00 total=0
//
//nolint:gocyclo
func parsePSI(contents []byte) (psiValues, error) {
	var (
		values psiValues
		found  [numPSITypes][numPSISpans]bool
	)

	for line := range bytes.Lines(contents) {
		key, fields, _ := bytes.Cut(bytes.TrimRight(line, "\n"), []byte(" "))

		t := indexOf(psiTypeNames[:], key)
		if t < 0 {
			continue
		}

		for field := range bytes.FieldsSeq(fields) {
			spanName, value, ok := bytes.Cut(field, []byte("="))
			if !ok {
				continue
			}

			sp := indexOf(psiSpanNames[:], spanName)
			if sp < 0 {
				continue
			}

			v, err := cgroups.ParseValueBytes(value)
			if err != nil || !v.IsSet || v.IsMax {
				return psiValues{}, fmt.Errorf("PSI is not defined: type: %s, span: %s", psiTypeNames[t], psiSpanNames[sp])
			}

			values[t][sp] = v.Float64()
			found[t][sp] = true
		}
	}

	for t := range numPSITypes {
		for sp := range numPSISpans {
			if !found[t][sp] {
				return psiValues{}, fmt.Errorf("cannot find memory pressure entry: type: %s, span: %s", psiTypeNames[t], psiSpanNames[sp])
			}
		}
	}

	return values, nil
}

// indexOf returns the index of name in names, or -1.
func indexOf(names []string, name []byte) int {
	for i := range names {
		if names[i] == string(name) {
			return i
		}
	}

	return -1
}

// parseSingleValue parses a single value file (e.g. memory.current); the value is unset if it can't be parsed.
func parseSingleValue(contents []byte) cgroups.Value {
	line, _, _ := bytes.Cut(contents, []byte("\n"))

	v, err := cgroups.ParseValueBytes(line)
	if err != nil {
		return cgroups.Value{}
	}

	return v
}

// cString returns a NUL-terminated copy of the path.
func cString(path string) []byte {
	return append([]byte(path), 0)
}

// openPath opens a file by a NUL-terminated path.
//
// Unlike os.Open and unix.Open, it doesn't allocate a copy of the path on each call.
func openPath(path []byte, flags int) (int, error) {
	dirfd := unix.AT_FDCWD

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
