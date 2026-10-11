// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/core/events"
	"github.com/containerd/errdefs"
	"github.com/containerd/typeurl/v2"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/owned"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	controllers "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type cpuObservationClient struct {
	mu        sync.Mutex
	info      containers.Container
	infoErr   error
	infoCalls int
	taskErr   error
	status    containerd.ProcessStatus
	replace   bool
	events    chan *events.Envelope
}

func (client *cpuObservationClient) Info(context.Context) (containers.Container, error) {
	client.mu.Lock()
	defer client.mu.Unlock()

	client.infoCalls++

	return client.info, client.infoErr
}

func (client *cpuObservationClient) Task(context.Context) (uint32, containerd.ProcessStatus, error) {
	client.mu.Lock()
	defer client.mu.Unlock()

	if client.replace {
		client.info.CreatedAt = client.info.CreatedAt.Add(time.Nanosecond)
	}

	return 17, client.status, client.taskErr
}

func (client *cpuObservationClient) Events(context.Context) (<-chan *events.Envelope, <-chan error) {
	return client.events, nil
}

func (*cpuObservationClient) Close() error { return nil }

type failingObservationRuntime struct {
	controller.Runtime
	fail *atomic.Bool
}

func (r failingObservationRuntime) Modify(ctx context.Context, res resource.Resource, update func(resource.Resource) error, options ...owned.ModifyOption) error {
	if r.fail.Load() && res.Metadata().Type() == k8s.KubeletCPUObservationType {
		return errors.New("injected observation publication failure")
	}

	return r.Runtime.Modify(ctx, res, update, options...)
}

type failingObservationController struct {
	controllers.KubeletCPUObservationController
	fail atomic.Bool
}

func (ctrl *failingObservationController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	return ctrl.KubeletCPUObservationController.Run(ctx, failingObservationRuntime{Runtime: r, fail: &ctrl.fail}, logger)
}

func cpuObservationScenario(name string, client *cpuObservationClient, args []string, token string) ([]string, string) {
	switch name {
	case "replacement":
		client.replace = true
	case "missing task":
		client.taskErr = errdefs.ErrNotFound
	case "task error":
		client.taskErr = errors.New("task unavailable")
	case "info error":
		client.infoErr = errors.New("info unavailable")
	case "stopped":
		client.status = containerd.Stopped
	case "malformed token":
		token = "false"
	case "missing token":
		token = ""
	case "unmanaged":
		args = []string{"kubelet"}
	}

	badArgs := map[string][]string{
		"invalid CPUs":       {"kubelet", "--reserved-cpus=invalid", "--cpu-manager-policy=static", "--cpu-manager-policy-options=strict-cpu-reservation=true"},
		"duplicate CPU flag": append(append([]string(nil), args...), "--reserved_cpus=0-1"),
		"flag terminator":    append([]string{"kubelet", "--"}, args[1:]...),
	}
	if replacement := badArgs[name]; replacement != nil {
		args = replacement
	}

	return args, token
}

func TestKubeletCPUObservationIdleWithoutPolicy(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := &ctest.DefaultSuite{}
		s.SetT(t)

		s.SetupTest()
		defer s.TearDownTest()

		client := &cpuObservationClient{infoErr: errdefs.ErrNotFound, events: make(chan *events.Envelope)}

		require.NoError(t, s.Runtime().RegisterController(&controllers.KubeletCPUObservationController{NewClient: func() (controllers.KubeletCPUClient, error) { return client, nil }}))
		synctest.Wait()
		client.mu.Lock()
		calls := client.infoCalls
		client.mu.Unlock()
		synctest.Sleep(time.Minute)
		synctest.Wait()
		client.mu.Lock()
		require.Equal(t, calls, client.infoCalls, "no request means no retry poller")
		client.mu.Unlock()
	})
}

func TestKubeletCPUObservationLostStartupEvent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := &ctest.DefaultSuite{}
		s.SetT(t)

		s.SetupTest()
		defer s.TearDownTest()

		rendered := k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID)
		rendered.Metadata().Annotations().Set(k8s.KubeletCPUManagedAnnotation, "true")
		s.Create(rendered)
		rendered, err := safe.StateGetByID[*k8s.KubeletSpec](s.Ctx(), s.State(), k8s.KubeletID)
		require.NoError(t, err)

		reservation := k8s.NewKubeletCPUReservation()
		*reservation.TypedSpec() = k8s.KubeletCPUReservationSpec{Managed: true, ReservedCPUs: "0-1"}
		s.Create(reservation)

		encoded, err := typeurl.MarshalAny(&specs.Spec{
			Process:     &specs.Process{Args: []string{"kubelet", "--reserved-cpus=0-1", "--cpu-manager-policy=static", "--cpu-manager-policy-options=strict-cpu-reservation=true"}},
			Annotations: map[string]string{k8s.KubeletSpecTokenAnnotation: k8s.KubeletSpecToken(rendered), k8s.KubeletCPUManagedAnnotation: "true"},
		})
		require.NoError(t, err)

		client := &cpuObservationClient{
			info:    containers.Container{ID: "kubelet", CreatedAt: time.Now(), UpdatedAt: time.Now(), Spec: encoded},
			infoErr: errdefs.ErrNotFound, status: containerd.Running, events: make(chan *events.Envelope, 1),
		}

		require.NoError(t, s.Runtime().RegisterController(&controllers.KubeletCPUObservationController{NewClient: func() (controllers.KubeletCPUClient, error) { return client, nil }}))
		synctest.Wait()
		client.mu.Lock()
		client.infoErr = nil
		client.mu.Unlock()
		// Subscription installation misses the start: no event or resource update follows.
		synctest.Sleep(time.Minute)
		synctest.Wait()

		observed, err := safe.StateGetByID[*k8s.KubeletCPUObservation](s.Ctx(), s.State(), k8s.KubeletID)
		require.NoError(t, err)
		require.Equal(t, k8s.KubeletSpecToken(rendered), observed.TypedSpec().SpecToken)
		oldToken := observed.TypedSpec().SpecToken

		require.NoError(t, safe.StateModify(s.Ctx(), s.State(), rendered, func(spec *k8s.KubeletSpec) error {
			spec.TypedSpec().Image = "next"

			return nil
		}))
		synctest.Wait()

		observed, err = safe.StateGetByID[*k8s.KubeletCPUObservation](s.Ctx(), s.State(), k8s.KubeletID)
		require.NoError(t, err)
		require.Equal(t, oldToken, observed.TypedSpec().SpecToken, "pending retries must not relabel stale evidence")

		rendered, err = safe.StateGetByID[*k8s.KubeletSpec](s.Ctx(), s.State(), k8s.KubeletID)
		require.NoError(t, err)
		encoded, err = typeurl.MarshalAny(&specs.Spec{
			Process:     &specs.Process{Args: []string{"kubelet", "--reserved-cpus=0-1", "--cpu-manager-policy=static", "--cpu-manager-policy-options=strict-cpu-reservation=true"}},
			Annotations: map[string]string{k8s.KubeletSpecTokenAnnotation: k8s.KubeletSpecToken(rendered), k8s.KubeletCPUManagedAnnotation: "true"},
		})
		require.NoError(t, err)
		client.mu.Lock()
		client.info.Spec = encoded
		client.info.CreatedAt = client.info.CreatedAt.Add(time.Nanosecond)
		client.mu.Unlock()
		synctest.Sleep(2 * time.Second)
		synctest.Wait()

		observed, err = safe.StateGetByID[*k8s.KubeletCPUObservation](s.Ctx(), s.State(), k8s.KubeletID)
		require.NoError(t, err)
		require.Equal(t, k8s.KubeletSpecToken(rendered), observed.TypedSpec().SpecToken)
		client.mu.Lock()
		calls := client.infoCalls
		client.mu.Unlock()
		synctest.Sleep(time.Minute)
		synctest.Wait()
		client.mu.Lock()
		require.Equal(t, calls, client.infoCalls, "retry disarms once the required binding is observed")
		client.mu.Unlock()
	})
}

func TestKubeletCPUObservation(t *testing.T) {
	for _, name := range []string{
		"existing task", "publication failure", "replacement", "missing task", "task error", "info error", "stopped",
		"malformed token", "missing token", "unmanaged", "invalid CPUs", "duplicate CPU flag", "flag terminator", "initial publication failure",
	} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := &ctest.DefaultSuite{}
				s.SetT(t)

				s.SetupTest()
				defer s.TearDownTest()

				rendered := k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID)
				s.Create(rendered)
				rendered, err := safe.StateGetByID[*k8s.KubeletSpec](s.Ctx(), s.State(), k8s.KubeletID)
				require.NoError(t, err)

				token := k8s.KubeletSpecToken(rendered)
				args := []string{"kubelet", "--reserved-cpus=0-1", "--cpu-manager-policy=static", "--cpu-manager-policy-options=full-pcpus-only=true", "--cpu-manager-policy-options=strict-cpu-reservation=true"}
				client := &cpuObservationClient{status: containerd.Running, events: make(chan *events.Envelope, 1)}

				args, token = cpuObservationScenario(name, client, args, token)

				annotations := map[string]string{k8s.KubeletSpecTokenAnnotation: token, k8s.KubeletCPUManagedAnnotation: "true"}
				if name == "unmanaged" {
					annotations[k8s.KubeletCPUManagedAnnotation] = "false"
				}

				encoded, err := typeurl.MarshalAny(&specs.Spec{Process: &specs.Process{Args: args}, Annotations: annotations})
				require.NoError(t, err)

				client.info = containers.Container{ID: "kubelet", CreatedAt: time.Now(), UpdatedAt: time.Now(), Spec: encoded}
				ctrl := &failingObservationController{NewClient: func() (controllers.KubeletCPUClient, error) { return client, nil }}
				ctrl.fail.Store(name == "initial publication failure")
				require.NoError(t, s.Runtime().RegisterController(ctrl))
				synctest.Wait()

				read := func() *k8s.KubeletCPUObservationSpec {
					res, err := safe.StateGetByID[*k8s.KubeletCPUObservation](s.Ctx(), s.State(), k8s.KubeletID)
					require.NoError(t, err)

					return res.TypedSpec()
				}

				switch name {
				case "existing task", "publication failure":
					ctrl.fail.Store(name == "publication failure")
					require.Equal(t, token, read().SpecToken)
					require.Equal(t, "0-1", read().ReservedCPUs)
					require.True(t, read().StrictCPUReservation)
					client.mu.Lock()
					client.taskErr = errdefs.ErrNotFound
					client.mu.Unlock()

					client.events <- nil

					synctest.Wait()

					if name == "publication failure" {
						require.Equal(t, token, read().SpecToken, "failed invalidation retains historical evidence")
						require.NoError(t, safe.StateModify(s.Ctx(), s.State(), rendered, func(spec *k8s.KubeletSpec) error {
							spec.TypedSpec().Image = "next"

							return nil
						}))
						rendered, err = safe.StateGetByID[*k8s.KubeletSpec](s.Ctx(), s.State(), k8s.KubeletID)
						require.NoError(t, err)
						require.NotEqual(t, k8s.KubeletSpecToken(rendered), read().SpecToken)
						ctrl.fail.Store(false)
					} else {
						require.Empty(t, read().SpecToken, "a task exit invalidates observation without a service event")
						client.mu.Lock()
						client.taskErr = nil
						client.mu.Unlock()

						client.events <- nil

						synctest.Wait()
						require.Equal(t, token, read().SpecToken, "same-container task restart keeps the immutable binding")
						client.mu.Lock()
						client.info.CreatedAt = client.info.CreatedAt.Add(time.Nanosecond)
						client.mu.Unlock()

						client.events <- nil

						synctest.Wait()
						require.Equal(t, token, read().SpecToken, "a new container with identical arguments retains the source binding")
					}
				case "initial publication failure":
					_, err := safe.StateGetByID[*k8s.KubeletCPUObservation](s.Ctx(), s.State(), k8s.KubeletID)
					require.True(t, state.IsNotFoundError(err))
					ctrl.fail.Store(false)
				case "unmanaged":
					require.Equal(t, token, read().SpecToken)
					require.False(t, read().Managed)
				default:
					require.Empty(t, read().SpecToken)
				}

				client.mu.Lock()
				client.replace = false
				client.taskErr, client.infoErr = nil, nil
				client.status = containerd.Running
				encoded, err = typeurl.MarshalAny(&specs.Spec{
					Process:     &specs.Process{Args: []string{"kubelet", "--reserved-cpus=2-3", "--cpu-manager-policy=static", "--cpu-manager-policy-options=strict-cpu-reservation=true"}},
					Annotations: map[string]string{k8s.KubeletSpecTokenAnnotation: k8s.KubeletSpecToken(rendered), k8s.KubeletCPUManagedAnnotation: "true"},
				})
				require.NoError(t, err)

				client.info.Spec = encoded
				client.info.UpdatedAt = client.info.UpdatedAt.Add(time.Nanosecond)
				client.mu.Unlock()

				client.events <- nil

				synctest.Sleep(time.Minute)
				synctest.Wait()
				require.Equal(t, "2-3", read().ReservedCPUs, "events and COSI backoff recover observation")
			})
		})
	}
}
