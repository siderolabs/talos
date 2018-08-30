// nolint: dupl,golint
package service

import (
	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/service/conditions"
	"github.com/autonomy/dianemo/src/initramfs/pkg/userdata"
)

// Trustd implements the Service interface. It serves as the concrete type with
// the required methods.
type Trustd struct{}

// Pre implements the Service interface.
func (p *Trustd) Pre(data userdata.UserData) error {
	return nil
}

// Post implements the Service interface.
func (p *Trustd) Post(data userdata.UserData) (err error) {
	return nil
}

// Cmd implements the Service interface.
func (p *Trustd) Cmd(data userdata.UserData, cmdArgs *CmdArgs) error {
	cmdArgs.Name = "trustd"
	cmdArgs.Path = "/bin/trustd"
	cmdArgs.Args = []string{
		"--port=50001",
		"--userdata=" + constants.UserDataPath,
	}

	return nil
}

// Condition implements the Service interface.
func (p *Trustd) Condition(data userdata.UserData) func() (bool, error) {
	return conditions.None()
}

// Env implements the Service interface.
func (p *Trustd) Env() []string { return []string{} }

// Type implements the Service interface.
func (p *Trustd) Type() Type { return Forever }
