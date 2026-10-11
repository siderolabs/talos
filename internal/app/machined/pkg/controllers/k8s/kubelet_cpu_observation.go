// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/core/events"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/errdefs"
	"github.com/containerd/typeurl/v2"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// KubeletCPUClient exposes the container/task reads needed for CPU binding evidence.
type KubeletCPUClient interface {
	Info(context.Context) (containers.Container, error)
	Task(context.Context) (uint32, containerd.ProcessStatus, error)
	Events(context.Context) (<-chan *events.Envelope, <-chan error)
	Close() error
}

// KubeletCPUObservationController observes CPU arguments independently of service startup.
type KubeletCPUObservationController struct {
	NewClient func() (KubeletCPUClient, error)
}

// Name implements controller.Controller.
func (*KubeletCPUObservationController) Name() string { return "k8s.KubeletCPUObservationController" }

// Inputs implements controller.Controller.
func (*KubeletCPUObservationController) Inputs() []controller.Input {
	return []controller.Input{
		{Namespace: k8s.NamespaceName, Type: k8s.KubeletSpecType, ID: optional.Some(k8s.KubeletID), Kind: controller.InputWeak},
		{Namespace: k8s.NamespaceName, Type: k8s.KubeletCPUReservationType, ID: optional.Some(k8s.KubeletID), Kind: controller.InputWeak},
		{Namespace: runtimeres.NamespaceName, Type: runtimeres.CPUPartitionStatusType, ID: optional.Some(runtimeres.CPUPartitionStatusID), Kind: controller.InputWeak},
		{Namespace: v1alpha1.NamespaceName, Type: v1alpha1.ServiceType, ID: optional.Some("kubelet"), Kind: controller.InputWeak},
	}
}

// Outputs implements controller.Controller.
func (*KubeletCPUObservationController) Outputs() []controller.Output {
	return []controller.Output{{Type: k8s.KubeletCPUObservationType, Kind: controller.OutputExclusive}}
}

// Run implements controller.Controller.
func (ctrl *KubeletCPUObservationController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	client, err := ctrl.openClient()
	if err != nil {
		return errors.Join(err, ctrl.publish(ctx, r, k8s.KubeletCPUObservationSpec{}))
	}
	defer client.Close() //nolint:errcheck

	ctx, cancel := context.WithCancel(namespaces.WithNamespace(ctx, constants.SystemContainerdNamespace))
	defer cancel()

	events, failures := client.Events(ctx)
	for {
		pending, err := ctrl.reconcile(ctx, r, client)
		if err != nil {
			return err
		}

		r.ResetRestartBackoff()

		retry := pendingKubeletCPUWakeup(pending)

		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-retry:
		case _, ok := <-events:
			if !ok {
				return fmt.Errorf("kubelet container event stream closed")
			}
		case err := <-failures:
			return errors.Join(fmt.Errorf("kubelet container event stream failed: %v", err), ctrl.publish(ctx, r, k8s.KubeletCPUObservationSpec{}))
		}
	}
}

func (ctrl *KubeletCPUObservationController) openClient() (KubeletCPUClient, error) {
	if ctrl.NewClient != nil {
		return ctrl.NewClient()
	}

	return newKubeletCPUClient()
}

func pendingKubeletCPUWakeup(pending bool) <-chan time.Time {
	if pending {
		return time.After(time.Second)
	}

	return nil
}

func (ctrl *KubeletCPUObservationController) reconcile(ctx context.Context, r controller.Runtime, client KubeletCPUClient) (bool, error) {
	observation, err := observeKubeletCPU(ctx, client)
	if err != nil {
		return false, errors.Join(err, ctrl.publish(ctx, r, k8s.KubeletCPUObservationSpec{}))
	}

	if err = ctrl.publish(ctx, r, observation); err != nil {
		return false, err
	}

	needed, err := kubeletCPUBindingRequired(ctx, r)
	if err != nil || !needed {
		return false, err
	}

	spec, err := safe.ReaderGetByID[*k8s.KubeletSpec](ctx, r, k8s.KubeletID)
	if state.IsNotFoundError(err) {
		return true, nil
	}

	if err != nil {
		return false, err
	}

	return observation.SpecToken != k8s.KubeletSpecToken(spec) || observation.Managed != k8s.KubeletCPUManaged(spec), nil
}

func kubeletCPUBindingRequired(ctx context.Context, r controller.Reader) (bool, error) {
	reservation, err := safe.ReaderGetByID[*k8s.KubeletCPUReservation](ctx, r, k8s.KubeletID)
	if err != nil && !state.IsNotFoundError(err) {
		return false, err
	}

	if reservation != nil && reservation.TypedSpec().Managed {
		return true, nil
	}

	status, err := safe.ReaderGetByID[*runtimeres.CPUPartitionStatus](ctx, r, runtimeres.CPUPartitionStatusID)
	if state.IsNotFoundError(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	for _, target := range status.TypedSpec().Targets {
		if target.Key == "kubepods" {
			phase := status.TypedSpec().Phase

			return phase == runtimeres.CPUPartitionPhaseConverging || phase == runtimeres.CPUPartitionPhaseRestoring, nil
		}
	}

	return false, nil
}

func (*KubeletCPUObservationController) publish(ctx context.Context, r controller.Runtime, observation k8s.KubeletCPUObservationSpec) error {
	return safe.WriterModify(ctx, r, k8s.NewKubeletCPUObservation(), func(res *k8s.KubeletCPUObservation) error {
		*res.TypedSpec() = observation

		return nil
	})
}

func observeKubeletCPU(ctx context.Context, client KubeletCPUClient) (k8s.KubeletCPUObservationSpec, error) {
	empty := k8s.KubeletCPUObservationSpec{}

	before, err := client.Info(ctx)
	if errdefs.IsNotFound(err) {
		return empty, nil
	}

	if err != nil {
		return empty, err
	}

	pid, err := runningKubeletTask(ctx, client)
	if err != nil || pid == 0 {
		return empty, err
	}

	after, err := client.Info(ctx)
	if err != nil {
		return empty, err
	}

	if !sameKubeletContainer(before, after) {
		return empty, fmt.Errorf("kubelet container changed during observation")
	}

	spec, err := kubeletOCISpec(before)
	if err != nil {
		return empty, err
	}

	observation, err := kubeletCPUArguments(spec)
	if err != nil {
		return empty, err
	}

	if observation.SpecToken == "" {
		return empty, nil
	}

	observation.ContainerCreated = before.CreatedAt.UTC().Format(time.RFC3339Nano)
	observation.TaskPID = pid

	return observation, nil
}

func runningKubeletTask(ctx context.Context, client KubeletCPUClient) (uint32, error) {
	pid, status, err := client.Task(ctx)
	if errdefs.IsNotFound(err) {
		return 0, nil
	}

	if err != nil {
		return 0, err
	}

	if status != containerd.Running {
		return 0, nil
	}

	return pid, nil
}

func sameKubeletContainer(before, after containers.Container) bool {
	return before.ID == "kubelet" && after.ID == "kubelet" && !before.CreatedAt.IsZero() && before.CreatedAt.Equal(after.CreatedAt) && before.UpdatedAt.Equal(after.UpdatedAt)
}

func kubeletOCISpec(info containers.Container) (*specs.Spec, error) {
	if info.Spec == nil {
		return nil, fmt.Errorf("kubelet OCI spec is missing")
	}

	decoded, err := typeurl.UnmarshalAny(info.Spec)
	if err != nil {
		return nil, err
	}

	spec, ok := decoded.(*specs.Spec)
	if !ok || spec.Process == nil || len(spec.Process.Args) == 0 {
		return nil, fmt.Errorf("invalid kubelet OCI process")
	}

	return spec, nil
}

func kubeletCPUArguments(spec *specs.Spec) (k8s.KubeletCPUObservationSpec, error) {
	observation := k8s.KubeletCPUObservationSpec{}

	args, err := parseKubeletCPUFlags(spec.Process.Args[1:])
	if err != nil {
		return observation, err
	}

	switch spec.Annotations[k8s.KubeletCPUManagedAnnotation] {
	case "", "false":
		return unmanagedKubeletBinding(spec.Annotations[k8s.KubeletSpecTokenAnnotation]), nil
	case "true":
	default:
		return observation, fmt.Errorf("invalid kubelet CPU ownership binding")
	}

	if len(args["reserved-cpus"]) != 1 || len(args["cpu-manager-policy"]) != 1 {
		return observation, fmt.Errorf("incomplete or ambiguous kubelet CPU arguments")
	}

	strict, err := observedStrictReservation(args["cpu-manager-policy-options"])
	if err != nil {
		return observation, err
	}

	token := spec.Annotations[k8s.KubeletSpecTokenAnnotation]
	if !validKubeletSpecToken(token) {
		return observation, fmt.Errorf("invalid kubelet spec binding")
	}

	if _, err := cpuset.Parse(args["reserved-cpus"][0]); err != nil {
		return observation, fmt.Errorf("invalid observed reserved CPUs: %w", err)
	}

	observation.SpecToken = token
	observation.Managed = true
	observation.ReservedCPUs = args["reserved-cpus"][0]
	observation.CPUManagerPolicy = args["cpu-manager-policy"][0]
	observation.StrictCPUReservation = strict

	return observation, nil
}

func unmanagedKubeletBinding(token string) k8s.KubeletCPUObservationSpec {
	if validKubeletSpecToken(token) {
		return k8s.KubeletCPUObservationSpec{SpecToken: token}
	}

	return k8s.KubeletCPUObservationSpec{}
}

func observedStrictReservation(values []string) (bool, error) {
	options := map[string]string{}

	for _, arg := range values {
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			return false, fmt.Errorf("malformed kubelet CPU option")
		}

		options[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}

	return strconv.ParseBool(options["strict-cpu-reservation"])
}

func parseKubeletCPUFlags(argv []string) (map[string][]string, error) {
	args := map[string][]string{}

	for _, arg := range argv {
		if arg == "--" {
			return nil, fmt.Errorf("kubelet argv contains a flag terminator")
		}

		key, value, ok := strings.Cut(strings.TrimPrefix(arg, "--"), "=")

		key = strings.ReplaceAll(key, "_", "-")
		switch key {
		case "reserved-cpus", "cpu-manager-policy", "cpu-manager-policy-options":
			if !strings.HasPrefix(arg, "--") || !ok {
				return nil, fmt.Errorf("ambiguous kubelet CPU argument")
			}

			args[key] = append(args[key], value)
		}
	}

	return args, nil
}

func validKubeletSpecToken(token string) bool {
	created, version, ok := strings.Cut(token, "/")
	stamp, stampErr := time.Parse(time.RFC3339Nano, created)
	parsedVersion, versionErr := resource.ParseVersion(version)

	return ok && stampErr == nil && !stamp.IsZero() && versionErr == nil && !parsedVersion.Equal(resource.VersionUndefined)
}

type kubeletCPUClient struct{ *containerd.Client }

func newKubeletCPUClient() (KubeletCPUClient, error) {
	client, err := containerd.New(constants.CRIContainerdAddress)
	if err != nil {
		return nil, err
	}

	return &kubeletCPUClient{client}, nil
}

func (client *kubeletCPUClient) Info(ctx context.Context) (containers.Container, error) {
	return client.ContainerService().Get(ctx, "kubelet")
}

func (client *kubeletCPUClient) Task(ctx context.Context) (uint32, containerd.ProcessStatus, error) {
	container, err := client.LoadContainer(ctx, "kubelet")
	if err != nil {
		return 0, "", err
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		return 0, "", err
	}

	status, err := task.Status(ctx)

	return task.Pid(), status.Status, err
}

func (client *kubeletCPUClient) Events(ctx context.Context) (<-chan *events.Envelope, <-chan error) {
	return client.Subscribe(ctx, `topic~="/tasks/",event.container_id=="kubelet"`, `topic~="/containers/",event.id=="kubelet"`)
}
