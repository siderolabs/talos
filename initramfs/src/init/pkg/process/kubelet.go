package process

import (
	"fmt"
	"os"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/process/conditions"
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/userdata"
)

type Kubelet struct{}

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

	return nil
}

func (p *Kubelet) Cmd(data userdata.UserData) (name string, args []string) {
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
		"--v=4",
	}

	if data.Join {
		labels := "--node-labels="
		for k, v := range data.Labels {
			labels += k + "=" + v + ","
		}
		args = append(args, labels)
	}

	return name, args
}

func (p *Kubelet) Condition() func() (bool, error) {
	return conditions.WaitForFileExists("/etc/containers/policy.json")
}

func (p *Kubelet) Env() []string { return []string{} }

func (p *Kubelet) Type() Type { return Forever }
