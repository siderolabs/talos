package process

import (
	"os"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/process/conditions"
)

type Kubelet struct{}

func init() {
	os.Mkdir("/run/flannel", os.ModeDir)
	os.MkdirAll("/etc/cni/net.d", os.ModeDir)
}

func (p *Kubelet) Cmd() (name string, args []string) {
	name = "/bin/kubelet"
	args = []string{
		"--container-runtime=remote",
		"--container-runtime-endpoint=unix:///var/run/crio/crio.sock",
		"--runtime-request-timeout=10m",
		"--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf",
		"--kubeconfig=/etc/kubernetes/kubelet.conf",
		"--pod-manifest-path=/etc/kubernetes/manifests",
		"--allow-privileged=true",
		"--network-plugin=cni",
		"--cni-conf-dir=/etc/cni/net.d",
		"--cni-bin-dir=/opt/cni/bin",
		"--cluster-dns=10.96.0.10",
		"--cluster-domain=cluster.local",
		"--authorization-mode=Webhook",
		"--client-ca-file=/etc/kubernetes/pki/ca.crt",
		"--cgroup-driver=cgroupfs",
		"--cadvisor-port=0",
		"--rotate-certificates=true",
		"--serialize-image-pulls=false",
		"--feature-gates=ExperimentalCriticalPodAnnotation=true",
	}

	return name, args
}

func (p *Kubelet) Condition() func() (bool, error) {
	return conditions.WaitForFileExists("/etc/kubernetes/kubelet.conf")
}

func (p *Kubelet) Env() []string { return []string{} }

func (p *Kubelet) Type() Type { return Forever }
