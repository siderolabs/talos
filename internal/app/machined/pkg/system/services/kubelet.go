// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	containerdapi "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/health"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/siderolabs/talos/internal/pkg/capability"
	"github.com/siderolabs/talos/internal/pkg/containers/image"
	"github.com/siderolabs/talos/internal/pkg/environment"
	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	timeresource "github.com/siderolabs/talos/pkg/machinery/resources/time"
)

var _ system.HealthcheckedService = (*Kubelet)(nil)

// Kubelet implements the Service interface. It serves as the concrete type with
// the required methods.
type Kubelet struct {
	imgRef string
}

// ID implements the Service interface.
func (k *Kubelet) ID(runtime.Runtime) string {
	return "kubelet"
}

// PreFunc implements the Service interface.
func (k *Kubelet) PreFunc(ctx context.Context, r runtime.Runtime) error {
	specResource, err := safe.ReaderGet[*k8s.KubeletSpec](ctx, r.State().V1Alpha2().Resources(), resource.NewMetadata(k8s.NamespaceName, k8s.KubeletSpecType, k8s.KubeletID, resource.VersionUndefined))
	if err != nil {
		return err
	}

	spec := specResource.TypedSpec()

	client, err := containerdapi.New(constants.CRIContainerdAddress)
	if err != nil {
		return err
	}
	//nolint:errcheck
	defer client.Close()

	// Pull the image and unpack it.
	containerdctx := namespaces.WithNamespace(ctx, constants.SystemContainerdNamespace)

	img, err := image.Pull(containerdctx, cri.RegistryBuilder(r.State().V1Alpha2().Resources()), client, spec.Image, image.WithSkipIfAlreadyPulled())
	if err != nil {
		return err
	}

	k.imgRef = img.Target().Digest.String()

	// Create lifecycle resource to signal that the kubelet is about to start.
	err = r.State().V1Alpha2().Resources().Create(ctx, k8s.NewKubeletLifecycle(k8s.NamespaceName, k8s.KubeletLifecycleID))
	if err != nil && !state.IsConflictError(err) { // ignore if the lifecycle resource already exists
		return err
	}

	return nil
}

// PostFunc implements the Service interface.
func (k *Kubelet) PostFunc(runtime.Runtime, events.ServiceState) (err error) {
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
func (k *Kubelet) DependsOn(runtime.Runtime) []string {
	return []string{"cri"}
}

// Runner implements the Service interface.
func (k *Kubelet) Runner(r runtime.Runtime) (runner.Runner, error) {
	specResource, err := safe.ReaderGet[*k8s.KubeletSpec](
		context.Background(),
		r.State().V1Alpha2().Resources(),
		resource.NewMetadata(k8s.NamespaceName, k8s.KubeletSpecType, k8s.KubeletID, resource.VersionUndefined),
	)
	if err != nil {
		return nil, err
	}

	spec := specResource.TypedSpec()

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
		{Type: "bind", Destination: "/lib/modules", Source: "/usr/lib/modules", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/etc/kubernetes", Source: "/etc/kubernetes", Options: []string{"bind", "rshared", "rw"}},
		{Type: "bind", Destination: constants.KubeletCredentialProviderBinDir, Source: constants.KubeletCredentialProviderBinDir, Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/etc/nfsmount.conf", Source: "/etc/nfsmount.conf", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/etc/machine-id", Source: "/etc/machine-id", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/etc/os-release", Source: "/etc/os-release", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: constants.PodResolvConfPath, Source: constants.PodResolvConfPath, Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/etc/cni", Source: "/etc/cni", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/usr/libexec/kubernetes", Source: "/usr/libexec/kubernetes", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/run", Source: "/run", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/lib/containerd", Source: "/var/lib/containerd", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/lib/kubelet", Source: "/var/lib/kubelet", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/log/containers", Source: "/var/log/containers", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/log/pods", Source: "/var/log/pods", Options: []string{"rbind", "rshared", "rw"}},
	}

	if _, err := os.Stat("/sys/kernel/security"); err == nil {
		mounts = append(mounts,
			specs.Mount{Type: "securityfs", Destination: "/sys/kernel/security", Source: "/sys/kernel/security", Options: []string{"bind", "ro"}},
		)
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

	return restart.New(containerd.NewRunner(
		r.Config().Debug() && r.Config().Machine().Type() == machine.TypeWorker, // enable debug logs only for the worker nodes
		&args,
		runner.WithLoggingManager(r.Logging()),
		runner.WithNamespace(constants.SystemContainerdNamespace),
		runner.WithContainerImage(k.imgRef),
		runner.WithEnv(environment.Get(r.Config())),
		runner.WithCgroupPath(constants.CgroupKubelet),
		runner.WithSelinuxLabel(constants.SelinuxLabelKubelet),
		runner.WithOCISpecOpts(
			containerd.WithRootfsPropagation("shared"),
			oci.WithMounts(mounts),
			oci.WithHostNamespace(specs.NetworkNamespace),
			oci.WithHostNamespace(specs.PIDNamespace),
			oci.WithParentCgroupDevices,
			oci.WithMaskedPaths(nil),
			oci.WithReadonlyPaths(nil),
			oci.WithWriteableSysfs,
			oci.WithWriteableCgroupfs,
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
	return func(ctx context.Context) error { return simpleHealthCheck(ctx, "http://127.0.0.1:10248/healthz") }
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

func simpleHealthCheck(ctx context.Context, url string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req) //nolint:bodyclose
	if err != nil {
		return err
	}

	bodyCloser := sync.OnceValue(resp.Body.Close)

	defer bodyCloser() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected HTTP status OK, got %s", resp.Status)
	}

	return bodyCloser()
}
