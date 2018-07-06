// nolint: dupl,golint
package service

import (
	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/service/conditions"
	"github.com/autonomy/dianemo/src/initramfs/pkg/userdata"
)

// OSD implements the Service interface. It serves as the concrete type with
// the required methods.
type OSD struct{}

// Pre implements the Service interface.
func (p *OSD) Pre(data userdata.UserData) error {
	return nil
}

// Cmd implements the Service interface.
func (p *OSD) Cmd(data userdata.UserData) (name string, args []string) {
	name = "/bin/osd"
	args = []string{
		"--port=50000",
		"--userdata=" + constants.UserDataPath,
	}

	if data.OS.Security.RootsOfTrust.Generate {
		args = append(args, "--generate=true")
	}

	return name, args
}

// Condition implements the Service interface.
func (p *OSD) Condition(data userdata.UserData) func() (bool, error) {
	return conditions.None()
}

// Env implements the Service interface.
func (p *OSD) Env() []string { return []string{} }

// Type implements the Service interface.
func (p *OSD) Type() Type { return Forever }
