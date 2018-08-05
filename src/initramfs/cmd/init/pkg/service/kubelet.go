package service

import (
	"fmt"
	"os"

	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/service/conditions"
	"github.com/autonomy/dianemo/src/initramfs/pkg/userdata"
)

// Kubelet implements the Service interface. It serves as the concrete type with
// the required methods.
type Kubelet struct{}

// Pre implements the Service interface.
func (p *Kubelet) Pre(data userdata.UserData) error {
	if err := os.Mkdir("/run/flannel", os.ModeDir); err != nil {
		return fmt.Errorf("create /run/flannel: %s", err.Error())
	}
	if err := os.MkdirAll("/etc/cni/net.d", os.ModeDir); err != nil {
		return fmt.Errorf("create /etc/cni/net.d: %s", err.Error())
	}
	if err := os.MkdirAll("/etc/kubernetes/manifests", os.ModeDir); err != nil {
		return fmt.Errorf("create /etc/kubernetes/manifests: %s", err.Error())
	}
	if err := os.MkdirAll("/var/lib/kubelet", os.ModeDir); err != nil {
		return fmt.Errorf("create /var/lib/kubelet: %s", err.Error())
	}

	return nil
}

// Cmd implements the Service interface.
func (p *Kubelet) Cmd(data userdata.UserData, cmdArgs *CmdArgs) {
	cmdArgs.Name = "kubelet"
	cmdArgs.Path = "/bin/docker"
	cmdArgs.Args = []string{
		"run",
		"--volume=/dev:/dev:shared",
		"--volume=/sys:/sys:ro",
		"--volume=/sys/fs/cgroup:/sys/fs/cgroup:rw",
		"--volume=/var/run:/var/run:rw",
		"--volume=/run:/run:rw",
		"--volume=/var/lib/docker:/var/lib/docker:rw",
		"--volume=/var/lib/kubelet:/var/lib/kubelet:rshared",
		"--volume=/var/log:/var/log",
		"--volume=/etc/cni:/etc/cni:ro",
		"--volume=/etc/kubernetes:/etc/kubernetes:shared",
		"--volume=/etc/os-release:/etc/os-release:ro",
		"--volume=/etc/ssl/certs:/etc/ssl/certs:ro",
		"--volume=/lib/modules:/lib/modules:ro",
		"--volume=/var/libexec/kubernetes:/usr/libexec/kubernetes:shared",
		"--rm",
		"--net=host",
		"--pid=host",
		"--privileged",
		"--name=kubelet",
		"gcr.io/google_containers/hyperkube:v1.11.1",
		"/hyperkube",
		"kubelet",
	}

	kubeletArgs := []string{
		"--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf",
		"--kubeconfig=/etc/kubernetes/kubelet.conf",
		"--config=/var/lib/kubelet/config.yaml",
		// "--runtime-request-timeout=10m",
		// "--pod-manifest-path=/etc/kubernetes/manifests",
		// "--allow-privileged=true",
		// "--network-plugin=cni",
		// "--cni-conf-dir=/etc/cni/net.d",
		// "--cni-bin-dir=/opt/cni/bin",
		// "--cluster-dns=10.96.0.10",
		// "--cluster-domain=cluster.local",
		// "--authorization-mode=Webhook",
		// "--client-ca-file=/etc/kubernetes/pki/ca.crt",
		// "--cgroup-driver=cgroupfs",
		// "--cadvisor-port=0",
		// "--rotate-certificates=true",
		// "--serialize-image-pulls=false",
		// "--v=2",
	}

	cmdArgs.Args = append(cmdArgs.Args, kubeletArgs...)

	switch data.Services.Kubeadm.ContainerRuntime {
	case constants.ContainerRuntimeCRIO:
		cmdArgs.Args = append(cmdArgs.Args, "--container-runtime=remote", "--container-runtime-endpoint=unix:///var/run/crio/crio.sock")
	default:
	}
}

// Condition implements the Service interface.
func (p *Kubelet) Condition(data userdata.UserData) func() (bool, error) {
	switch data.Services.Kubeadm.ContainerRuntime {
	case constants.ContainerRuntimeDocker:
		return conditions.None()
	case constants.ContainerRuntimeCRIO:
		return conditions.WaitForFileExists("/etc/containers/policy.json")
	default:
		return conditions.None()
	}
}

// Env implements the Service interface.
func (p *Kubelet) Env() []string { return []string{} }

// Type implements the Service interface.
func (p *Kubelet) Type() Type { return Forever }
