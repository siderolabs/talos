package service

import (
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/service/conditions"
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/userdata"
)

type Docker struct{}

func (p *Docker) Pre(data userdata.UserData) error {
	return nil
}

func (p *Docker) Cmd(data userdata.UserData) (name string, args []string) {
	name = "/bin/dockerd"
	args = []string{
		"--live-restore",
		"--iptables=false",
		"--ip-masq=false",
		"--storage-driver=overlay2",
		"--selinux-enabled=false",
		"--exec-opt=native.cgroupdriver=cgroupfs",
		"--log-opt=max-size=10m",
		"--log-opt=max-file=3",
	}

	return name, args
}

func (p *Docker) Condition(data userdata.UserData) func() (bool, error) {
	return conditions.None()
}

func (p *Docker) Env() []string {
	return []string{"DOCKER_NOFILE=1000000"}
}

func (p *Docker) Type() Type { return Forever }
