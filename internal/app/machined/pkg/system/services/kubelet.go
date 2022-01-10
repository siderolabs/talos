// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	containerdapi "github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/cosi-project/runtime/pkg/resource"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/internal/pkg/capability"
	"github.com/talos-systems/talos/internal/pkg/containers/image"
	"github.com/talos-systems/talos/pkg/conditions"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
	timeresource "github.com/talos-systems/talos/pkg/machinery/resources/time"
)

// Kubelet implements the Service interface. It serves as the concrete type with
// the required methods.
type Kubelet struct{}

// ID implements the Service interface.
func (k *Kubelet) ID(r runtime.Runtime) string {
	return "kubelet"
}

// PreFunc implements the Service interface.
func (k *Kubelet) PreFunc(ctx context.Context, r runtime.Runtime) error {
	specResource, err := r.State().V1Alpha2().Resources().Get(ctx, resource.NewMetadata(k8s.NamespaceName, k8s.KubeletSpecType, k8s.KubeletID, resource.VersionUndefined))
	if err != nil {
		return err
	}

	spec := specResource.(*k8s.KubeletSpec).TypedSpec()

	client, err := containerdapi.New(constants.CRIContainerdAddress)
	if err != nil {
		return err
	}
	//nolint:errcheck
	defer client.Close()

	// Pull the image and unpack it.
	containerdctx := namespaces.WithNamespace(ctx, constants.SystemContainerdNamespace)

	_, err = image.Pull(containerdctx, r.Config().Machine().Registries(), client, spec.Image, image.WithSkipIfAlreadyPulled())
	if err != nil {
		return err
	}

	return nil
}

// PostFunc implements the Service interface.
func (k *Kubelet) PostFunc(r runtime.Runtime, state events.ServiceState) (err error) {
	return nil
}

// Condition implements the Service interface.
func (k *Kubelet) Condition(r runtime.Runtime) conditions.Condition {
	return conditions.WaitForAll(
		timeresource.NewSyncCondition(r.State().V1Alpha2().Resources()),
		network.NewReadyCondition(r.State().V1Alpha2().Resources(), network.AddressReady, network.HostnameReady, network.EtcFilesReady),
	)
}

// DependsOn implements the Service interface.
func (k *Kubelet) DependsOn(r runtime.Runtime) []string {
	return []string{"cri"}
}

// Runner implements the Service interface.
func (k *Kubelet) Runner(r runtime.Runtime) (runner.Runner, error) {
	specResource, err := r.State().V1Alpha2().Resources().Get(context.Background(), resource.NewMetadata(k8s.NamespaceName, k8s.KubeletSpecType, k8s.KubeletID, resource.VersionUndefined))
	if err != nil {
		return nil, err
	}

	spec := specResource.(*k8s.KubeletSpec).TypedSpec()

	// Set the process arguments.
	args := runner.Args{
		ID:          k.ID(r),
		ProcessArgs: append([]string{"/usr/local/bin/kubelet"}, spec.Args...),
	}
	// Set the required kubelet mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/dev", Source: "/dev", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "sysfs", Destination: "/sys", Source: "/sys", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: constants.CgroupMountPath, Source: constants.CgroupMountPath, Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/lib/modules", Source: "/lib/modules", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/etc/kubernetes", Source: "/etc/kubernetes", Options: []string{"bind", "rshared", "rw"}},
		{Type: "bind", Destination: "/etc/machine-id", Source: "/etc/machine-id", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/etc/os-release", Source: "/etc/os-release", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/etc/cni", Source: "/etc/cni", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/usr/libexec/kubernetes", Source: "/usr/libexec/kubernetes", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/run", Source: "/run", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/lib/containerd", Source: "/var/lib/containerd", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/lib/kubelet", Source: "/var/lib/kubelet", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/log/containers", Source: "/var/log/containers", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/log/pods", Source: "/var/log/pods", Options: []string{"rbind", "rshared", "rw"}},
	}

	// Add extra mounts.
	// TODO(andrewrynhard): We should verify that the mount source is
	// allowlisted. There is the potential that a user can expose
	// sensitive information.
	for _, mount := range spec.ExtraMounts {
		if err = os.MkdirAll(mount.Source, 0o700); err != nil {
			return nil, err
		}

		mounts = append(mounts, mount)
	}

	env := []string{}
	for key, val := range r.Config().Machine().Env() {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	return restart.New(containerd.NewRunner(
		r.Config().Debug() && r.Config().Machine().Type() == machine.TypeWorker, // enable debug logs only for the worker nodes
		&args,
		runner.WithLoggingManager(r.Logging()),
		runner.WithNamespace(constants.SystemContainerdNamespace),
		runner.WithContainerImage(spec.Image),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			containerd.WithRootfsPropagation("shared"),
			oci.WithCgroup(constants.CgroupKubelet),
			oci.WithMounts(mounts),
			oci.WithHostNamespace(specs.NetworkNamespace),
			oci.WithHostNamespace(specs.PIDNamespace),
			oci.WithParentCgroupDevices,
			oci.WithMaskedPaths(nil),
			oci.WithReadonlyPaths(nil),
			oci.WithWriteableSysfs,
			oci.WithWriteableCgroupfs,
			oci.WithSelinuxLabel(""),
			oci.WithApparmorProfile(""),
			oci.WithAllDevicesAllowed,
			oci.WithCapabilities(capability.AllGrantableCapabilities()), // TODO: kubelet doesn't need all of these, we should consider limiting capabilities
		),
		runner.WithOOMScoreAdj(constants.KubeletOOMScoreAdj),
		runner.WithCustomSeccompProfile(kubeletSeccomp),
	),
		restart.WithType(restart.Forever),
	), nil
}

// HealthFunc implements the HealthcheckedService interface.
func (k *Kubelet) HealthFunc(runtime.Runtime) health.Check {
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
		//nolint:errcheck
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("expected HTTP status OK, got %s", resp.Status)
		}

		return nil
	}
}

// HealthSettings implements the HealthcheckedService interface.
func (k *Kubelet) HealthSettings(runtime.Runtime) *health.Settings {
	settings := health.DefaultSettings
	settings.InitialDelay = 2 * time.Second // increase initial delay as kubelet is slow on startup

	return &settings
}

// APIRestartAllowed implements APIRestartableService.
func (k *Kubelet) APIRestartAllowed(runtime.Runtime) bool {
	return true
}

// APIStartAllowed implements APIStartableService.
func (k *Kubelet) APIStartAllowed(runtime.Runtime) bool {
	return true
}

func kubeletSeccomp(seccomp *specs.LinuxSeccomp) {
	// for cephfs mounts
	seccomp.Syscalls = append(seccomp.Syscalls,
		specs.LinuxSyscall{
			Names: []string{
				"add_key",
				"request_key",
			},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
	)
}
