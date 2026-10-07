// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	containerdapi "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
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
	"github.com/siderolabs/talos/internal/pkg/containers/image/console"
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
	mu       sync.Mutex
	prepared *k8s.KubeletSpec
	ready    chan struct{}
	launch   *kubeletLaunch

	pullImageFn func(context.Context, runtime.Runtime, string) (string, error)
	newRunnerFn func(bool, *runner.Args, ...runner.Option) runner.Runner
}

type kubeletLaunch struct {
	spec   *k8s.KubeletSpec
	imgRef string
}

// Retire prevents subsequent launches from using files being replaced.
// Repeated preparation attempts keep the same open channel for existing waiters.
func (k *Kubelet) Retire() {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.ready == nil || k.prepared != nil {
		k.ready = make(chan struct{})
	}

	k.prepared = nil
}

// Publish makes a successfully prepared snapshot available to pending starts.
func (k *Kubelet) Publish(spec *k8s.KubeletSpec) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.ready == nil {
		k.ready = make(chan struct{})
	}

	if k.prepared == nil {
		close(k.ready)
	}

	k.prepared = spec.DeepCopy().(*k8s.KubeletSpec)
}

func (k *Kubelet) waitPrepared(ctx context.Context) (*k8s.KubeletSpec, error) {
	for {
		k.mu.Lock()
		if k.ready == nil {
			k.ready = make(chan struct{})
		}

		spec, ready := k.prepared, k.ready
		k.mu.Unlock()

		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if spec != nil {
			return spec, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ready:
		}
	}
}

// ID implements the Service interface.
func (k *Kubelet) ID(runtime.Runtime) string {
	return "kubelet"
}

// PreFunc implements the Service interface.
func (k *Kubelet) PreFunc(ctx context.Context, r runtime.Runtime) error {
	spec, err := k.waitPrepared(ctx)
	if err != nil {
		return err
	}

	pullImage := k.pullImageFn
	if pullImage == nil {
		pullImage = pullKubeletImage
	}

	imgRef, err := pullImage(ctx, r, spec.TypedSpec().Image)
	if err != nil {
		return err
	}

	if err = ctx.Err(); err != nil {
		return err
	}

	k.mu.Lock()
	k.launch = &kubeletLaunch{spec: spec, imgRef: imgRef}
	k.mu.Unlock()

	// Create lifecycle resource to signal that the kubelet is about to start.
	err = r.State().V1Alpha2().Resources().Create(ctx, k8s.NewKubeletLifecycle(k8s.NamespaceName, k8s.KubeletLifecycleID))
	if err != nil && !state.IsConflictError(err) { // ignore if the lifecycle resource already exists
		return err
	}

	return nil
}

func pullKubeletImage(ctx context.Context, r runtime.Runtime, imageRef string) (string, error) {
	client, err := containerdapi.New(constants.CRIContainerdAddress)
	if err != nil {
		return "", err
	}
	//nolint:errcheck
	defer client.Close()

	// Pull the image and unpack it.
	containerdctx := namespaces.WithNamespace(ctx, constants.SystemContainerdNamespace)

	img, err := image.PullWithRetriesAndTimeout(
		containerdctx,
		cri.RegistryBuilder(r.State().V1Alpha2().Resources()),
		r.State().V1Alpha2().Resources(),
		client, imageRef,
		image.WithSkipIfAlreadyPulled(),
		image.WithProgressReporter(console.NewProgressReporter),
	)
	if err != nil {
		return "", err
	}

	return img.Target().Digest.String(), nil
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

// Volumes implements the Service interface.
func (k *Kubelet) Volumes(runtime.Runtime) []string {
	return []string{
		"/var/lib",
		constants.KubeletDataVolumeID,
		constants.LogVolumeID,
		"/var/log/audit",
		"/var/log/containers",
		"/var/log/pods",
		"/var/lib/kubelet/seccomp",
		constants.SeccompProfilesDirectory,
		constants.KubernetesAuditLogDir,
		constants.UserVolumeMountPoint,
	}
}

// Runner implements the Service interface.
func (k *Kubelet) Runner(r runtime.Runtime) (runner.Runner, error) {
	k.mu.Lock()
	launch := k.launch
	k.mu.Unlock()

	if launch == nil {
		return nil, fmt.Errorf("kubelet has no prepared launch")
	}

	spec := launch.spec.TypedSpec()

	newRunner := k.newRunnerFn
	if newRunner == nil {
		newRunner = containerd.NewRunner
	}

	// Set the process arguments from the same prepared snapshot as the files and image.
	processArgs, binding := kubeletCPUArguments(launch.spec)
	args := runner.Args{
		ID:          k.ID(r),
		ProcessArgs: processArgs,
	}

	// Set the required kubelet mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/dev", Source: "/dev", Options: []string{"bind", "rw"}},
		{Type: "sysfs", Destination: "/sys", Source: "/sys", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: constants.CgroupMountPath, Source: constants.CgroupMountPath, Options: []string{"bind", "rw"}},
		{Type: "bind", Destination: "/lib/modules", Source: "/usr/lib/modules", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/etc/kubernetes", Source: "/etc/kubernetes", Options: []string{"bind", "rw"}},
		{Type: "bind", Destination: constants.KubeletCredentialProviderBinDir, Source: constants.KubeletCredentialProviderBinDir, Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/etc/nfsmount.conf", Source: "/etc/nfsmount.conf", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/etc/machine-id", Source: "/etc/machine-id", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/etc/os-release", Source: "/etc/os-release", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: constants.PodResolvConfPath, Source: constants.PodResolvConfPath, Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/etc/cni", Source: "/etc/cni", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/var/run", Source: "/run", Options: []string{"rbind", "rslave", "rw"}},
		{Type: "bind", Destination: "/var/lib/containerd", Source: "/var/lib/containerd", Options: []string{"rbind", "rslave", "rw"}},
		{Type: "bind", Destination: "/var/lib/kubelet", Source: "/var/lib/kubelet", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/log/containers", Source: "/var/log/containers", Options: []string{"bind", "rw"}},
		{Type: "bind", Destination: "/var/log/pods", Source: "/var/log/pods", Options: []string{"bind", "rw"}},
		{Type: "bind", Destination: constants.UserVolumeMountPoint, Source: constants.UserVolumeMountPoint, Options: []string{"rbind", "rslave", "ro"}},
	}

	if _, err := os.Stat("/sys/kernel/security"); err == nil {
		mounts = append(
			mounts,
			specs.Mount{Type: "securityfs", Destination: "/sys/kernel/security", Source: "/sys/kernel/security", Options: []string{"bind", "ro"}},
		)
	}

	// Add extra mounts.
	// TODO(andrewrynhard): We should verify that the mount source is
	// allowlisted. There is the potential that a user can expose
	// sensitive information.
	for _, mount := range spec.ExtraMounts {
		if err := os.MkdirAll(mount.Source, 0o700); err != nil {
			return nil, err
		}

		mounts = append(mounts, mount)
	}

	return restart.New(
		newRunner(
			r.Config().Debug() && r.Config().Machine().Type() == machine.TypeWorker, // enable debug logs only for the worker nodes
			&args,
			runner.WithLoggingManager(r.Logging()),
			runner.WithNamespace(constants.SystemContainerdNamespace),
			runner.WithContainerImage(launch.imgRef),
			runner.WithEnv(environment.Get(r.Config())),
			runner.WithCgroupPath(constants.CgroupKubelet),
			runner.WithSelinuxLabel(constants.SelinuxLabelKubelet),
			runner.WithOCISpecOpts(
				containerd.WithRootfsPropagation("shared"),
				oci.WithMounts(mounts),
				oci.WithHostNamespace(specs.NetworkNamespace),
				oci.WithHostNamespace(specs.PIDNamespace),
				binding,
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

func kubeletCPUArguments(spec *k8s.KubeletSpec) ([]string, oci.SpecOpts) {
	return append([]string{"/usr/local/bin/kubelet"}, spec.TypedSpec().Args...),
		oci.WithAnnotations(map[string]string{
			k8s.KubeletSpecTokenAnnotation:  k8s.KubeletSpecToken(spec),
			k8s.KubeletCPUManagedAnnotation: strconv.FormatBool(k8s.KubeletCPUManaged(spec)),
		})
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
	seccomp.Syscalls = append(
		seccomp.Syscalls,
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

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
