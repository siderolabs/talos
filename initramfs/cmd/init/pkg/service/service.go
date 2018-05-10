package service

import (
	"fmt"
	"log"
	"os/exec"
	"path"
	"time"

	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/constants"
	servicelog "github.com/autonomy/dianemo/initramfs/cmd/init/pkg/service/log"
	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/userdata"
)

// Type represents the service's restart policy.
type Type int

const (
	// Forever will always restart a process.
	Forever Type = iota
	// Once will restart the process only if it did not exit successfully.
	Once
)

// Service is an interface describing a process that is to be run as a system
// level service.
type Service interface {
	// Pre is invoked before a command is executed. It is useful for things like
	// preparing files that the process might depend on.
	Pre(userdata.UserData) error
	// Cmd describes the path to the binary, and the set of arguments to be
	// passed into it upon execution.
	Cmd(userdata.UserData) (string, []string)
	// Condition is invoked just before starting the process.
	Condition(userdata.UserData) func() (bool, error)
	// Env describes the service's environment variables. Elements should be in
	// the format <key=<value>
	Env() []string
	// Type describes the service's restart policy.
	Type() Type
}

// Manager is a type with helper methods that build a service and invoke the set
// of methods defined in the Service interface.
type Manager struct {
	UserData userdata.UserData
}

func (m *Manager) build(proc Service) (cmd *exec.Cmd, err error) {
	// Build the exec.Cmd
	name, args := proc.Cmd(m.UserData)
	cmd = exec.Command(name, args...)

	// Set the environment for the service.
	cmd.Env = append(proc.Env(), fmt.Sprintf("PATH=%s", constants.PATH))

	// Setup logging.
	w, err := servicelog.New(path.Base(name))
	if err != nil {
		err = fmt.Errorf("service log handler: %s", err.Error())
		return
	}
	cmd.Stdout = w
	cmd.Stderr = w

	return cmd, nil
}

// Start will invoke the service's Pre, Condition, and Type funcs. If the any
// error occurs in the Pre or Condition invocations, it is up to the caller to
// to restart the service.
func (m *Manager) Start(proc Service) {
	go func(proc Service) {
		err := proc.Pre(m.UserData)
		if err != nil {
			log.Printf("pre: %s", err.Error())
		}
		satisfied, err := proc.Condition(m.UserData)()
		if err != nil {
			// TODO: Write the error to the log writer.
			log.Printf("condition: %s", err.Error())
		}
		if !satisfied {
			log.Printf("condition not satisfied")
			return
		}
		// Wait for the command to exit. Then, based on the service Type, take
		// the requested action.
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

func (m *Manager) waitAndRestart(proc Service) (err error) {
	cmd, err := m.build(proc)
	if err != nil {
		return
	}
	if err = cmd.Start(); err != nil {
		return
	}
	state, err := cmd.Process.Wait()
	if err != nil {
		return
	}
	if state.Exited() {
		time.Sleep(5 * time.Second)
		return m.waitAndRestart(proc)
	}

	return nil
}

func (m *Manager) waitForSuccess(proc Service) (err error) {
	cmd, err := m.build(proc)
	if err != nil {
		return
	}
	if err = cmd.Start(); err != nil {
		return
	}
	state, err := cmd.Process.Wait()
	if err != nil {
		return
	}
	if !state.Success() {
		time.Sleep(5 * time.Second)
		return m.waitForSuccess(proc)
	}

	return nil
}
