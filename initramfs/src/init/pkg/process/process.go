package process

import (
	"fmt"
	"log"
	"os/exec"
	"path"
	"time"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/constants"
	logstream "github.com/autonomy/dianemo/initramfs/src/init/pkg/log"
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/userdata"
)

type Type int

const (
	Forever Type = iota
	Once
	OnceAndOnlyOnce
)

type Process interface {
	Pre(userdata.UserData) error
	Cmd(userdata.UserData) (string, []string)
	Condition() func() (bool, error)
	Env() []string
	Type() Type
}

type Manager struct {
	UserData userdata.UserData
}

func (m *Manager) build(proc Process) (*exec.Cmd, error) {
	name, args := proc.Cmd(m.UserData)
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

func (m *Manager) Start(proc Process) {
	go func(proc Process) {
		err := proc.Pre(m.UserData)
		if err != nil {
			log.Printf("pre: %s", err.Error())
		}
		satisfied, err := proc.Condition()()
		if err != nil {
			// TODO: Write the error to the log writer.
			log.Printf("condition: %s", err.Error())
		}
		if !satisfied {
			log.Printf("condition not satisfied")
			return
		}
		// Wait for the command to exit. Then, based on the process Type, take
		// the appropriate actions.
		switch proc.Type() {
		case Forever:
			if err := m.waitAndRestart(proc); err != nil {
				log.Printf("run: %s", err.Error())
			}
		case Once:
			if err := m.waitForSuccess(proc); err != nil {
				log.Printf("run: %s", err.Error())
			}
		}
	}(proc)
}

func (m *Manager) waitAndRestart(proc Process) error {
	cmd, err := m.build(proc)
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	state, err := cmd.Process.Wait()
	if err != nil {
		// TODO: Write the error to the log writer.
	}
	if state.Exited() {
		time.Sleep(5 * time.Second)
		return m.waitAndRestart(proc)
	}

	return nil
}

func (m *Manager) waitForSuccess(proc Process) error {
	cmd, err := m.build(proc)
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	state, err := cmd.Process.Wait()
	if err != nil {
		// TODO: Write the error to the log writer.
	}
	if !state.Success() {
		time.Sleep(5 * time.Second)
		return m.waitForSuccess(proc)
	}

	return nil
}
