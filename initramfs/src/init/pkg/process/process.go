package process

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/constants"
)

type Type int

const (
	Forever Type = iota
	Once
	OnceAndOnlyOnce
)

var outputstreams = map[string]*io.PipeReader{}

type Process interface {
	Cmd() (string, []string)
	Condition() func() (bool, error)
	Env() []string
	Type() Type
}

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) build(proc Process) (*exec.Cmd, error) {
	name, args := proc.Cmd()
	cmd := exec.Command(name, args...)
	// Set the environment for the process.
	cmd.Env = append(proc.Env(), fmt.Sprintf("PATH=%s", constants.PATH))
	// Create a buffer for the stdout and stderr of the process.
	r, w := io.Pipe()
	outputstreams[path.Base(name)] = r
	cmd.Stdout = w
	cmd.Stderr = w

	return cmd, nil
}

func (m *Manager) Start(proc Process) error {
	go func(proc Process) {
		satisfied, err := proc.Condition()()
		if !satisfied || err != nil {
			// TODO: Write the error to the log writer.
			return
		}
		// Wait for the command to exit. Then, based on the process Type, take
		// the appropriate actions.
		switch proc.Type() {
		case Forever:
			m.waitAndRestart(proc)
		case Once:
			m.waitForSuccess(proc)
		}
	}(proc)

	return nil
}

func StreamHandleFunc(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/logs/")
	stream, ok := outputstreams[name]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
	}

	if stream == nil {
		w.Write([]byte(fmt.Sprintf("process has not started: %s", name)))
		return
	}

	buffer := make([]byte, 1024)
	for {
		n, err := stream.Read(buffer)
		if err != nil {
			stream.Close()
			break
		}
		data := buffer[0:n]
		w.Write(data)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		for i := 0; i < n; i++ {
			buffer[i] = 0
		}
	}
}

func (m *Manager) waitAndRestart(proc Process) {
	cmd, err := m.build(proc)
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		log.Println(err.Error())
		return
	}
	state, err := cmd.Process.Wait()
	if err != nil {
		// TODO: Write the error to the log writer.
	}
	if state.Exited() {
		time.Sleep(5 * time.Second)
		m.waitAndRestart(proc)
	}
}

func (m *Manager) waitForSuccess(proc Process) {
	cmd, err := m.build(proc)
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		log.Println(err.Error())
		return
	}
	state, err := cmd.Process.Wait()
	if err != nil {
		// TODO: Write the error to the log writer.
	}
	if !state.Success() {
		time.Sleep(5 * time.Second)
		m.waitForSuccess(proc)
	}
}
