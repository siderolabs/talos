package service

import (
	"fmt"
	"os"

	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/service/conditions"
	"github.com/autonomy/dianemo/initramfs/pkg/userdata"
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

	return nil
}

// Cmd implements the Service interface.
func (p *Kubelet) Cmd(data userdata.UserData) (name string, args []string) {
	name = "/bin/kubelet"
	args = []string{
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
		"--v=2",
	}

	switch data.Kubernetes.ContainerRuntime {
	case constants.ContainerRuntimeCRIO:
		args = append(args, "--container-runtime=remote", "--container-runtime-endpoint=unix:///var/run/crio/crio.sock")
	default:
	}

	for k, v := range data.Kubernetes.Kubelet.ExtraArgs {
		arg := "--" + k + "=" + v
		args = append(args, arg)
	}

	if len(data.Kubernetes.Kubelet.FeatureGates) != 0 {
		featureGates := "--feature-gates="
		for k, v := range data.Kubernetes.Kubelet.FeatureGates {
			featureGates += k + "=" + v + ","
		}
		args = append(args, featureGates)
	}

	if len(data.Kubernetes.Kubelet.Labels) != 0 {
		labels := "--node-labels="
		for k, v := range data.Kubernetes.Kubelet.Labels {
			labels += k + "=" + v + ","
		}
		args = append(args, labels)
	}

	return name, args
}

// Condition implements the Service interface.
func (p *Kubelet) Condition(data userdata.UserData) func() (bool, error) {
	switch data.Kubernetes.ContainerRuntime {
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
