package services

import (
	"os"

	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/system/conditions"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/system/runner"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/system/runner/process"
	"github.com/autonomy/talos/src/initramfs/pkg/userdata"
)

// Containerd implements the Service interface. It serves as the concrete type with
// the required methods.
type Containerd struct{}

// ID implements the Service interface.
func (c *Containerd) ID(data *userdata.UserData) string {
	return "containerd"
}

// PreFunc implements the Service interface.
func (c *Containerd) PreFunc(data *userdata.UserData) error {
	return os.MkdirAll("/var/lib/containerd", os.ModeDir)
}

// PostFunc implements the Service interface.
func (c *Containerd) PostFunc(data *userdata.UserData) (err error) {
	return nil
}

// ConditionFunc implements the Service interface.
func (c *Containerd) ConditionFunc(data *userdata.UserData) conditions.ConditionFunc {
	return conditions.None()
}

// Start implements the Service interface.
func (c *Containerd) Start(data *userdata.UserData) error {
	// Set the process arguments.
	args := &runner.Args{
		ID:          c.ID(data),
		ProcessArgs: []string{"/bin/containerd"},
	}

	r := process.Process{}

	return r.Run(
		data,
		args,
	)
}
