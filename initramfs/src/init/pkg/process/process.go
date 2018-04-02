package process

import (
	"fmt"
	"log"
	"os/exec"
	"path"
	"time"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/constants"
	logstream "github.com/autonomy/dianemo/initramfs/src/init/pkg/log"
)

type Type int

const (
	Forever Type = iota
	Once
	OnceAndOnlyOnce
)

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
	w, err := logstream.New(path.Base(name))
	if err != nil {
		return nil, fmt.Errorf("process log handler: %s", err.Error())
	}
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
