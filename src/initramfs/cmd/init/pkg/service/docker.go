package service

import (
	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/service/conditions"
	"github.com/autonomy/dianemo/src/initramfs/pkg/userdata"
)

// Docker implements the Service interface. It serves as the concrete type with
// the required methods.
type Docker struct{}

// Pre implements the Service interface.
func (p *Docker) Pre(data userdata.UserData) error {
	return nil
}

// Post implements the Service interface.
func (p *Docker) Post(data userdata.UserData) (err error) {
	return nil
}

// Cmd implements the Service interface.
func (p *Docker) Cmd(data userdata.UserData, cmdArgs *CmdArgs) error {
	cmdArgs.Name = "docker"
	cmdArgs.Path = "/bin/dockerd"
	cmdArgs.Args = []string{
		"--live-restore",
		"--iptables=false",
		"--ip-masq=false",
		"--storage-driver=overlay2",
		"--selinux-enabled=false",
		"--exec-opt=native.cgroupdriver=cgroupfs",
		"--log-opt=max-size=10m",
		"--log-opt=max-file=3",
	}

	return nil
}

// Condition implements the Service interface.
func (p *Docker) Condition(data userdata.UserData) func() (bool, error) {
	return conditions.None()
}

// Env implements the Service interface.
func (p *Docker) Env() []string {
	return []string{"DOCKER_NOFILE=1000000"}
}

// Type implements the Service interface.
func (p *Docker) Type() Type { return Forever }
