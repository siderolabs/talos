package process

import (
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/process/conditions"
)

type Docker struct{}

func (p *Docker) Cmd() (name string, args []string) {
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

func (p *Docker) Condition() func() (bool, error) {
	return conditions.None()
}

func (p *Docker) Env() []string { return []string{} }

func (p *Docker) Type() Type { return Forever }
