// nolint: dupl,golint
package service

import (
	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/service/conditions"
	"github.com/autonomy/dianemo/initramfs/pkg/userdata"
)

// ROTD implements the Service interface. It serves as the concrete type with
// the required methods.
type ROTD struct{}

// Pre implements the Service interface.
func (p *ROTD) Pre(data userdata.UserData) error {
	return nil
}

// Cmd implements the Service interface.
func (p *ROTD) Cmd(data userdata.UserData) (name string, args []string) {
	name = "/bin/rotd"
	args = []string{
		"--port=50001",
		"--userdata=" + constants.UserDataPath,
	}

	return name, args
}

// Condition implements the Service interface.
func (p *ROTD) Condition(data userdata.UserData) func() (bool, error) {
	return conditions.None()
}

// Env implements the Service interface.
func (p *ROTD) Env() []string { return []string{} }

// Type implements the Service interface.
func (p *ROTD) Type() Type { return Forever }
