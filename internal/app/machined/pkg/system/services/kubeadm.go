/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	containerdapi "github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	criconstants "github.com/containerd/cri/pkg/constants"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/services/kubeadm"
	"github.com/talos-systems/talos/internal/pkg/cis"
	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/constants"
)

// Kubeadm implements the Service interface. It serves as the concrete type with
// the required methods.
type Kubeadm struct{}

// ID implements the Service interface.
func (k *Kubeadm) ID(config config.Configurator) string {
	return "kubeadm"
}

// PreFunc implements the Service interface.
// nolint: gocyclo
func (k *Kubeadm) PreFunc(ctx context.Context, config config.Configurator) (err error) {
	switch config.Machine().Type() {
	case machine.Bootstrap:
		fallthrough
	case machine.ControlPlane:
		if err = kubeadm.WritePKIFiles(config); err != nil {
			return err
		}
		if err = cis.EnforceCommonMasterRequirements(config.Cluster().AESCBCEncryptionSecret()); err != nil {
			return err
		}
	}

	s, err := config.Cluster().Config(config.Machine().Type())
	if err != nil {
		return err
	}

	if err = ioutil.WriteFile(constants.KubeadmConfig, []byte(s), 0400); err != nil {
		return fmt.Errorf("write %s: %v", constants.KubeadmConfig, err)
	}

	client, err := containerdapi.New(constants.ContainerdAddress)
	if err != nil {
		return err
	}
	// nolint: errcheck
	defer client.Close()

	// Pull the image and unpack it.
	containerdctx := namespaces.WithNamespace(ctx, "k8s.io")
	image := fmt.Sprintf("%s:v%s", constants.KubernetesImage, config.Cluster().Version())
	if _, err = client.Pull(containerdctx, image, containerdapi.WithPullUnpack); err != nil {
		return fmt.Errorf("failed to pull image %q: %v", image, err)
	}

	return nil
}

// PostFunc implements the Service interface.
func (k *Kubeadm) PostFunc(config config.Configurator) error {
	return nil
}

// DependsOn implements the Service interface.
func (k *Kubeadm) DependsOn(config config.Configurator) []string {
	var deps []string

	switch config.Machine().Type() {
	case machine.Bootstrap:
		fallthrough
	case machine.ControlPlane:
		deps = []string{"containerd", "networkd", "etcd"}
	default:
		deps = []string{"containerd", "networkd"}
	}

	return deps
}

// Condition implements the Service interface.
func (k *Kubeadm) Condition(config config.Configurator) conditions.Condition {
	return nil
}

// Runner implements the Service interface.
func (k *Kubeadm) Runner(config config.Configurator) (runner.Runner, error) {
	image := fmt.Sprintf("%s:v%s", constants.KubernetesImage, config.Cluster().Version())

	// We only wan't to run kubeadm if it hasn't been ran already.
	if _, err := os.Stat("/etc/kubernetes/kubelet.conf"); !os.IsNotExist(err) {
		return nil, nil
	}

	// Set the process arguments.
	args := runner.Args{
		ID: k.ID(config),
	}

	switch {
	case config.Machine().Type() == machine.Bootstrap:
		args.ProcessArgs = []string{
			"kubeadm",
			"init",
			"--config=/etc/kubernetes/kubeadm-config.yaml",
			"--ignore-preflight-errors=all",
			"--skip-token-print",
			"--skip-certificate-key-print",
			"--upload-certs",
		}
	default:
		// Worker
		args.ProcessArgs = []string{
			"kubeadm",
			"join",
			"--config=/etc/kubernetes/kubeadm-config.yaml",
			"--ignore-preflight-errors=all",
		}
	}

	// Set the mounts.
	// nolint: dupl
	mounts := []specs.Mount{
		{Type: "cgroup", Destination: "/sys/fs/cgroup", Options: []string{"ro"}},
		{Type: "bind", Destination: "/var/run", Source: "/run", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/lib/kubelet", Source: "/var/lib/kubelet", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/lib/modules", Source: "/lib/modules", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/etc/kubernetes", Source: "/etc/kubernetes", Options: []string{"bind", "rw"}},
		{Type: "bind", Destination: "/etc/os-release", Source: "/etc/os-release", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/bin/crictl", Source: "/bin/crictl", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/bin/kubeadm", Source: "/bin/kubeadm", Options: []string{"bind", "ro"}},
	}

	env := []string{}
	for key, val := range config.Machine().Env() {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	return containerd.NewRunner(
		config.Debug(),
		&args,
		runner.WithNamespace(criconstants.K8sContainerdNamespace),
		runner.WithContainerImage(image),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			containerd.WithMemoryLimit(int64(1000000*512)),
			containerd.WithRootfsPropagation("slave"),
			oci.WithMounts(mounts),
			oci.WithHostNamespace(specs.PIDNamespace),
			oci.WithParentCgroupDevices,
			oci.WithPrivileged,
		),
	), nil
}
