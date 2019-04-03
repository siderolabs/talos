/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/oci"
	criconstants "github.com/containerd/cri/pkg/constants"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/talos-systems/talos/internal/app/init/internal/rootfs/cni"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
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
	requiredMounts := []string{
		"/dev/disk/by-path",
		"/etc/kubernetes",
		"/run",
		"/sys/fs/cgroup",
		"/usr/libexec/kubernetes",
		"/var/lib/containerd",
		"/var/lib/kubelet",
		"/var/log/pods",
	}

	for _, dir := range requiredMounts {
		if err := os.MkdirAll(dir, os.ModeDir); err != nil {
			return fmt.Errorf("create %s: %s", dir, err.Error())
		}
	}

	return nil
}

// PostFunc implements the Service interface.
func (k *Kubelet) PostFunc(data *userdata.UserData) (err error) {
	return nil
}

// ConditionFunc implements the Service interface.
func (k *Kubelet) ConditionFunc(data *userdata.UserData) conditions.ConditionFunc {
	return conditions.WaitForFilesToExist("/var/lib/kubelet/kubeadm-flags.env", defaults.DefaultAddress)
}

// Start implements the Service interface.
func (k *Kubelet) Start(data *userdata.UserData) error {
	image := constants.KubernetesImage

	// Set the process arguments.
	args := runner.Args{
		ID: k.ID(data),
		ProcessArgs: []string{
			"/hyperkube",
			"kubelet",
			"--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf",
			"--kubeconfig=/etc/kubernetes/kubelet.conf",
			"--config=/var/lib/kubelet/config.yaml",
			"--container-runtime=remote",
			"--runtime-request-timeout=15m",
			"--container-runtime-endpoint=unix://" + defaults.DefaultAddress,
		},
	}

	fileBytes, err := ioutil.ReadFile("/var/lib/kubelet/kubeadm-flags.env")
	if err != nil {
		return err
	}
	argsString := strings.TrimPrefix(string(fileBytes), "KUBELET_KUBEADM_ARGS=")
	argsString = strings.TrimSuffix(argsString, "\n")
	args.ProcessArgs = append(args.ProcessArgs, strings.Split(argsString, " ")...)

	// Set the required kubelet mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/dev", Source: "/dev", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/etc/kubernetes", Source: "/etc/kubernetes", Options: []string{"bind", "rw"}},
		{Type: "bind", Destination: "/etc/os-release", Source: "/etc/os-release", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/var/run", Source: "/run", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "cgroup", Destination: "/sys/fs/cgroup", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/usr/libexec/kubernetes", Source: "/usr/libexec/kubernetes", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/lib/containerd", Source: "/var/lib/containerd", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/lib/kubelet", Source: "/var/lib/kubelet", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/log/pods", Source: "/var/log/pods", Options: []string{"rbind", "rshared", "rw"}},
	}

	// Add in the additional CNI mounts
	cniMounts, err := cni.Mounts(data)
	if err != nil {
		return err
	}
	mounts = append(mounts, cniMounts...)

	env := []string{}
	for key, val := range data.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	r := containerd.Containerd{}

	return r.Run(
		data,
		args,
		runner.WithNamespace(criconstants.K8sContainerdNamespace),
		runner.WithContainerImage(image),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			containerd.WithRootfsPropagation("slave"),
			oci.WithMounts(mounts),
			oci.WithHostNamespace(specs.PIDNamespace),
			oci.WithParentCgroupDevices,
			oci.WithPrivileged,
		),
		runner.WithType(runner.Forever),
	)
}
