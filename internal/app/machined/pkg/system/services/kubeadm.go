/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"context"
	"fmt"
	"os"
	"strings"

	containerdapi "github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	criconstants "github.com/containerd/cri/pkg/constants"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/services/kubeadm"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Kubeadm implements the Service interface. It serves as the concrete type with
// the required methods.
type Kubeadm struct{}

// ID implements the Service interface.
func (k *Kubeadm) ID(data *userdata.UserData) string {
	return "kubeadm"
}

// PreFunc implements the Service interface.
// nolint: gocyclo
func (k *Kubeadm) PreFunc(ctx context.Context, data *userdata.UserData) (err error) {
	if data.Services.Kubeadm.IsBootstrap() {
		if err = kubeadm.WritePKIFiles(data); err != nil {
			return err
		}
	}

	if err = kubeadm.WriteConfig(data); err != nil {
		return err
	}

	client, err := containerdapi.New(constants.ContainerdAddress)
	if err != nil {
		return err
	}
	// nolint: errcheck
	defer client.Close()

	// Pull the image and unpack it.
	containerdctx := namespaces.WithNamespace(ctx, "k8s.io")
	image := fmt.Sprintf("%s:v%s", constants.KubernetesImage, data.KubernetesVersion)
	if _, err = client.Pull(containerdctx, image, containerdapi.WithPullUnpack); err != nil {
		return fmt.Errorf("failed to pull image %q: %v", image, err)
	}

	return nil
}

// PostFunc implements the Service interface.
func (k *Kubeadm) PostFunc(data *userdata.UserData) error {
	return nil
}

// DependsOn implements the Service interface.
func (k *Kubeadm) DependsOn(data *userdata.UserData) []string {
	deps := []string{"containerd", "networkd"}

	return deps
}

// Condition implements the Service interface.
func (k *Kubeadm) Condition(data *userdata.UserData) conditions.Condition {
	return nil
}

// Runner implements the Service interface.
func (k *Kubeadm) Runner(data *userdata.UserData) (runner.Runner, error) {
	image := fmt.Sprintf("%s:v%s", constants.KubernetesImage, data.KubernetesVersion)

	// We only wan't to run kubeadm if it hasn't been ran already.
	if _, err := os.Stat("/etc/kubernetes/kubelet.conf"); !os.IsNotExist(err) {
		return nil, nil
	}

	// Set the process arguments.
	args := runner.Args{
		ID: k.ID(data),
	}

	ignorePreflightErrors := []string{"cri", "kubeletversion", "numcpu", "ipvsproxiercheck"}
	ignorePreflightErrors = append(ignorePreflightErrors, data.Services.Kubeadm.IgnorePreflightErrors...)
	ignore := "--ignore-preflight-errors=" + strings.Join(ignorePreflightErrors, ",")

	switch {
	case data.Services.Kubeadm.InitConfiguration != nil:
		args.ProcessArgs = []string{
			"kubeadm",
			"init",
			"--config=/etc/kubernetes/kubeadm-config.yaml",
			ignore,
			"--skip-token-print",
			"--skip-certificate-key-print",
			"--upload-certs",
		}
	case data.Services.Kubeadm.JoinConfiguration != nil:
		// Worker
		args.ProcessArgs = []string{
			"kubeadm",
			"join",
			"--config=/etc/kubernetes/kubeadm-config.yaml",
			ignore,
		}
	default:
		return nil, errors.New("invalid kubeadm configuration type")
	}

	args.ProcessArgs = append(args.ProcessArgs, data.Services.Kubeadm.ExtraArgs...)

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
	for key, val := range data.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	return containerd.NewRunner(
		data,
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
