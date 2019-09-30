/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/containerd/containerd/oci"
	criconstants "github.com/containerd/cri/pkg/constants"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"

	"github.com/talos-systems/talos/internal/app/machined/internal/cni"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/constants"
)

// Kubelet implements the Service interface. It serves as the concrete type with
// the required methods.
type Kubelet struct{}

// ID implements the Service interface.
func (k *Kubelet) ID(config config.Configurator) string {
	return "kubelet"
}

// PreFunc implements the Service interface.
func (k *Kubelet) PreFunc(ctx context.Context, config config.Configurator) error {
	return nil
}

// PostFunc implements the Service interface.
func (k *Kubelet) PostFunc(config config.Configurator) (err error) {
	return nil
}

// Condition implements the Service interface.
func (k *Kubelet) Condition(config config.Configurator) conditions.Condition {
	return conditions.WaitForFilesToExist("/var/lib/kubelet/kubeadm-flags.env")
}

// DependsOn implements the Service interface.
func (k *Kubelet) DependsOn(config config.Configurator) []string {
	return []string{"containerd", "kubeadm"}
}

// Runner implements the Service interface.
func (k *Kubelet) Runner(config config.Configurator) (runner.Runner, error) {
	image := fmt.Sprintf("%s:v%s", constants.KubernetesImage, config.Cluster().Version())

	// Set the process arguments.
	args := runner.Args{
		ID: k.ID(config),
		ProcessArgs: []string{
			"/hyperkube",
			"kubelet",
			"--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf",
			"--kubeconfig=/etc/kubernetes/kubelet.conf",
			"--config=/var/lib/kubelet/config.yaml",
			"--container-runtime=remote",
			"--runtime-request-timeout=15m",
			"--container-runtime-endpoint=unix://" + constants.ContainerdAddress,
		},
	}

	fileBytes, err := ioutil.ReadFile("/var/lib/kubelet/kubeadm-flags.env")
	if err != nil {
		return nil, err
	}
	argsString := strings.TrimPrefix(string(fileBytes), "KUBELET_KUBEADM_ARGS=\"")
	argsString = strings.TrimSuffix(argsString, "\"\n")
	args.ProcessArgs = append(args.ProcessArgs, strings.Split(argsString, " ")...)
	// Set the required kubelet mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/dev", Source: "/dev", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "sysfs", Destination: "/sys", Source: "/sys", Options: []string{"bind", "ro"}},
		{Type: "cgroup", Destination: "/sys/fs/cgroup", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/lib/modules", Source: "/lib/modules", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/etc/kubernetes", Source: "/etc/kubernetes", Options: []string{"bind", "rw"}},
		{Type: "bind", Destination: "/etc/os-release", Source: "/etc/os-release", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/usr/libexec/kubernetes", Source: "/usr/libexec/kubernetes", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/run", Source: "/run", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/lib/containerd", Source: "/var/lib/containerd", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/lib/kubelet", Source: "/var/lib/kubelet", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/log/pods", Source: "/var/log/pods", Options: []string{"rbind", "rshared", "rw"}},
	}

	// Add in the additional CNI mounts.
	cniMounts, err := cni.Mounts(config)
	if err != nil {
		return nil, err
	}
	mounts = append(mounts, cniMounts...)

	// Add extra mounts.
	// TODO(andrewrynhard): We should verify that the mount source is
	// whitelisted. There is the potential that a user can expose
	// sensitive information.
	mounts = append(mounts, config.Machine().Kubelet().ExtraMounts()...)

	env := []string{}
	for key, val := range config.Machine().Env() {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	return restart.New(containerd.NewRunner(
		config.Debug(),
		&args,
		runner.WithNamespace(criconstants.K8sContainerdNamespace),
		runner.WithContainerImage(image),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			containerd.WithRootfsPropagation("shared"),
			oci.WithMounts(mounts),
			oci.WithHostNamespace(specs.PIDNamespace),
			oci.WithParentCgroupDevices,
			oci.WithPrivileged,
		),
	),
		restart.WithType(restart.Forever),
	), nil
}

// HealthFunc implements the HealthcheckedService interface
func (k *Kubelet) HealthFunc(config.Configurator) health.Check {
	return func(ctx context.Context) error {
		req, err := http.NewRequest("GET", "http://127.0.0.1:10248/healthz", nil)
		if err != nil {
			return err
		}
		req = req.WithContext(ctx)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		// nolint: errcheck
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return errors.Errorf("expected HTTP status OK, got %s", resp.Status)
		}

		return nil
	}
}

// HealthSettings implements the HealthcheckedService interface
func (k *Kubelet) HealthSettings(config.Configurator) *health.Settings {
	settings := health.DefaultSettings
	settings.InitialDelay = 2 * time.Second // increase initial delay as kubelet is slow on startup

	return &settings
}
