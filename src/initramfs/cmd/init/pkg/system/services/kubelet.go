package services

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/system/conditions"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/system/runner"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/system/runner/containerd"
	"github.com/autonomy/talos/src/initramfs/pkg/userdata"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// Kubelet implements the Service interface. It serves as the concrete type with
// the required methods.
type Kubelet struct{}

// ID implements the Service interface.
func (k *Kubelet) ID(data *userdata.UserData) string {
	return "kubelet"
}

// PreFunc implements the Service interface.
func (k *Kubelet) PreFunc(data *userdata.UserData) error {
	if err := os.Mkdir("/run/flannel", os.ModeDir); err != nil {
		return fmt.Errorf("create /run/flannel: %s", err.Error())
	}
	if err := os.Mkdir("/run/calico", os.ModeDir); err != nil {
		return fmt.Errorf("create /run/calico: %s", err.Error())
	}
	if err := os.MkdirAll("/var/etc/cni/net.d", os.ModeDir); err != nil {
		return fmt.Errorf("create /var/etc/cni/net.d: %s", err.Error())
	}
	if err := os.MkdirAll("/var/opt/cni/bin", os.ModeDir); err != nil {
		return fmt.Errorf("create /var/opt/cni/bin: %s", err.Error())
	}
	if err := os.MkdirAll("/var/etc/kubernetes/manifests", os.ModeDir); err != nil {
		return fmt.Errorf("create /var/etc/kubernetes/manifests: %s", err.Error())
	}
	if err := os.MkdirAll("/var/lib/kubelet", os.ModeDir); err != nil {
		return fmt.Errorf("create /var/lib/kubelet: %s", err.Error())
	}
	if err := os.MkdirAll("/var/lib/calico", os.ModeDir); err != nil {
		return fmt.Errorf("create /var/lib/calico: %s", err.Error())
	}
	if err := os.MkdirAll("/var/libexec/kubernetes", os.ModeDir); err != nil {
		return fmt.Errorf("create /var/libexec/kubernetes: %s", err.Error())
	}
	return nil
}

// PostFunc implements the Service interface.
func (k *Kubelet) PostFunc(data *userdata.UserData) (err error) {
	return nil
}

// ConditionFunc implements the Service interface.
func (k *Kubelet) ConditionFunc(data *userdata.UserData) conditions.ConditionFunc {
	var conditionFunc conditions.ConditionFunc
	switch data.Services.Init.ContainerRuntime {
	case constants.ContainerRuntimeDocker:
		conditionFunc = conditions.WaitForFilesToExist("/var/lib/kubelet/kubeadm-flags.env")
	case constants.ContainerRuntimeCRIO:
		conditionFunc = conditions.WaitForFilesToExist("/var/lib/kubelet/kubeadm-flags.env", "/var/etc/containers/policy.json")
	}

	return conditionFunc
}

// Start implements the Service interface.
func (k *Kubelet) Start(data *userdata.UserData) error {
	// Set the image.
	var image string
	if data.Services.Kubelet != nil && data.Services.Kubelet.Image != "" {
		image = data.Services.Kubelet.Image
	} else {
		image = constants.KubernetesImage
	}

	// Set the process arguments.
	args := runner.Args{
		ID: k.ID(data),
		ProcessArgs: []string{
			"/hyperkube",
			"kubelet",
			"--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf",
			"--kubeconfig=/etc/kubernetes/kubelet.conf",
			"--config=/var/lib/kubelet/config.yaml",
		},
	}

	if data.Services.Init.ContainerRuntime == constants.ContainerRuntimeCRIO {
		args.ProcessArgs = append(args.ProcessArgs, "--container-runtime=remote", "--runtime-request-timeout=15m", "--container-runtime-endpoint=unix:///var/run/crio/crio.sock")
	}

	fileBytes, err := ioutil.ReadFile("/var/lib/kubelet/kubeadm-flags.env")
	if err != nil {
		return err
	}
	argsString := strings.TrimPrefix(string(fileBytes), "KUBELET_KUBEADM_ARGS=")
	argsString = strings.TrimSuffix(argsString, "\n")
	args.ProcessArgs = append(args.ProcessArgs, strings.Split(argsString, " ")...)

	// Set the mounts.
	mounts := []specs.Mount{
		{Type: "cgroup", Destination: "/sys/fs/cgroup", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/dev", Source: "/dev", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/run", Source: "/run", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/lib/kubelet", Source: "/var/lib/kubelet", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/lib/calico", Source: "/var/lib/calico", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/etc/kubernetes", Source: "/var/etc/kubernetes", Options: []string{"bind", "rw"}},
		{Type: "bind", Destination: "/etc/cni", Source: "/var/etc/cni", Options: []string{"rbind", "rshared", "ro"}},
		{Type: "bind", Destination: "/opt/cni", Source: "/var/opt/cni", Options: []string{"rbind", "rshared", "ro"}},
		{Type: "bind", Destination: "/etc/os-release", Source: "/etc/os-release", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/usr/libexec/kubernetes", Source: "/var/libexec/kubernetes", Options: []string{"rbind", "rshared", "rw"}},
	}

	switch data.Services.Init.ContainerRuntime {
	case constants.ContainerRuntimeDocker:
		mounts = append(mounts, specs.Mount{Type: "bind", Destination: "/var/lib/docker", Source: "/var/lib/docker", Options: []string{"rbind", "rshared", "rw"}})
	case constants.ContainerRuntimeCRIO:
		mounts = append(mounts, specs.Mount{Type: "bind", Destination: "/etc/containers", Source: "/var/etc/containers", Options: []string{"bind", "rw"}})
	}

	env := []string{}
	for key, val := range data.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	r := containerd.Containerd{}

	return r.Run(
		data,
		args,
		runner.WithContainerImage(image),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			containerd.WithMemoryLimit(int64(1000000*2048)),
			containerd.WithRootfsPropagation("slave"),
			oci.WithMounts(mounts),
			oci.WithHostNamespace(specs.PIDNamespace),
			oci.WithParentCgroupDevices,
			oci.WithPrivileged,
		),
		runner.WithType(runner.Forever),
	)
}
