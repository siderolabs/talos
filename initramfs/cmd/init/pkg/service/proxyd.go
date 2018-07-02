// nolint: dupl,golint
package service

import (
	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/service/conditions"
	"github.com/autonomy/dianemo/initramfs/pkg/userdata"
)

// ProxyD implements the Service interface. It serves as the concrete type with
// the required methods.
type ProxyD struct{}

// Pre implements the Service interface.
func (p *ProxyD) Pre(data userdata.UserData) error {
	return nil
}

// Cmd implements the Service interface.
func (p *ProxyD) Cmd(data userdata.UserData) (name string, args []string) {
	name = "/bin/proxyd"
	args = []string{}

	return name, args
}

// Condition implements the Service interface.
func (p *ProxyD) Condition(data userdata.UserData) func() (bool, error) {
	return conditions.WaitForFileExists("/etc/kubernetes/admin.conf")
}

// Env implements the Service interface.
func (p *ProxyD) Env() []string { return []string{} }

// Type implements the Service interface.
func (p *ProxyD) Type() Type { return Forever }
