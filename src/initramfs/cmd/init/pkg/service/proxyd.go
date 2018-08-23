// nolint: dupl,golint
package service

import (
	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/service/conditions"
	"github.com/autonomy/dianemo/src/initramfs/pkg/userdata"
)

// ProxyD implements the Service interface. It serves as the concrete type with
// the required methods.
type ProxyD struct{}

// Pre implements the Service interface.
func (p *ProxyD) Pre(data userdata.UserData) error {
	return nil
}

// Cmd implements the Service interface.
func (p *ProxyD) Cmd(data userdata.UserData, cmdArgs *CmdArgs) error {
	cmdArgs.Name = "proxyd"
	cmdArgs.Path = "/bin/proxyd"
	cmdArgs.Args = []string{}

	return nil
}

// Condition implements the Service interface.
func (p *ProxyD) Condition(data userdata.UserData) func() (bool, error) {
	return conditions.WaitForFileExists("/etc/kubernetes/admin.conf")
}

// Env implements the Service interface.
func (p *ProxyD) Env() []string { return []string{} }

// Type implements the Service interface.
func (p *ProxyD) Type() Type { return Forever }
