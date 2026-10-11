// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/core/events"
	"github.com/containerd/errdefs"
	"github.com/containerd/typeurl/v2"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/cpuset"

	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type partitionKubeletClient struct {
	mu        sync.Mutex
	container containers.Container
	events    chan *events.Envelope
}

func (client *partitionKubeletClient) launch(spec *k8s.KubeletSpec) error {
	client.mu.Lock()
	defer client.mu.Unlock()

	encoded, err := typeurl.MarshalAny(&specs.Spec{
		Process: &specs.Process{Args: append([]string{"/usr/local/bin/kubelet"}, spec.TypedSpec().Args...)},
		Annotations: map[string]string{
			k8s.KubeletSpecTokenAnnotation:  k8s.KubeletSpecToken(spec),
			k8s.KubeletCPUManagedAnnotation: strconv.FormatBool(k8s.KubeletCPUManaged(spec)),
		},
	})
	if err != nil {
		return err
	}

	created := time.Now()
	if !created.After(client.container.CreatedAt) {
		created = client.container.CreatedAt.Add(time.Nanosecond)
	}

	client.container = containers.Container{ID: "kubelet", CreatedAt: created, UpdatedAt: created, Spec: encoded}
	select {
	case client.events <- nil:
	default:
	}

	return nil
}

func (client *partitionKubeletClient) Info(context.Context) (containers.Container, error) {
	client.mu.Lock()
	defer client.mu.Unlock()

	if client.container.ID == "" {
		return containers.Container{}, errdefs.ErrNotFound
	}

	return client.container, nil
}

func (*partitionKubeletClient) Task(context.Context) (uint32, containerd.ProcessStatus, error) {
	return 1, containerd.Running, nil
}

func (client *partitionKubeletClient) Events(context.Context) (<-chan *events.Envelope, <-chan error) {
	return client.events, nil
}

func (*partitionKubeletClient) Close() error { return nil }

func TestCPUPartitionLegacyCPURelease(t *testing.T) {
	for _, kind := range []string{"partial", "complete", "yaml reservation"} {
		t.Run(kind, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := newCPUPartitionSuite(t)
				defer s.TearDownTest()

				s.fs.set("kubepods", "0-2")
				s.kubelet("0,4-7")
				s.start()
				s.publish(kubernetesOnlyPolicy())
				synctest.Wait()
				s.requirePhase(runtime.CPUPartitionPhaseReady)
				require.Equal(t, "1-3", s.fs.mask("kubepods"))

				policy := kubernetesOnlyPolicy()
				delete(policy.TypedSpec().Roots, "kubepods")
				s.publish(policy)
				synctest.Wait()
				s.requirePhase(runtime.CPUPartitionPhaseConverging)
				require.False(t, s.reservation().Managed)

				spec := k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID)
				spec.TypedSpec().Config = map[string]any{}
				spec.TypedSpec().Args = []string{"--cpu-manager-policy=static"}

				if kind == "complete" {
					spec.TypedSpec().Args = append(spec.TypedSpec().Args, "--reserved-cpus=7", "--cpu-manager-policy-options=strict-cpu-reservation=true")
				}

				if kind == "yaml reservation" {
					spec.TypedSpec().Config["reservedSystemCPUs"] = "7"
				}

				replace(s, spec)
				synctest.Wait()
				s.requirePhase(runtime.CPUPartitionPhaseConverging)
				legacy, err := safe.StateGetByID[*k8s.KubeletSpec](s.Ctx(), s.State(), k8s.KubeletID)
				require.NoError(t, err)
				require.NoError(t, s.cpu.launch(legacy))
				synctest.Wait()
				s.requirePhase(runtime.CPUPartitionPhaseReady)
				require.Equal(t, "0-2", s.fs.mask("kubepods"))
			})
		})
	}
}

func TestCPUPartitionActualCPUHandoff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		require.NoError(t, s.Runtime().RegisterController(&k8sctrl.KubeletSpecController{}))

		cfg := k8s.NewKubeletConfig(k8s.NamespaceName, k8s.KubeletID)
		cfg.TypedSpec().Image = "kubelet:v1.38.0-alpha.1"
		cfg.TypedSpec().ExtraArgs = map[string]k8s.ArgValues{"node-ip": {Values: []string{"192.0.2.1"}}}
		s.Create(cfg)

		name := k8s.NewNodename(k8s.NamespaceName, k8s.NodenameID)
		name.TypedSpec().Nodename = "worker"
		s.Create(name)

		typ := config.NewMachineType()
		typ.SetMachineType(machine.TypeWorker)
		s.Create(typ)
		s.start()
		s.publish(kubernetesOnlyPolicy())
		synctest.Wait()
		s.requirePhase(runtime.CPUPartitionPhaseConverging)
		require.Empty(t, s.fs.mask("kubepods"))

		read := func() *k8s.KubeletSpec {
			spec, err := safe.StateGetByID[*k8s.KubeletSpec](s.Ctx(), s.State(), k8s.KubeletID)
			require.NoError(t, err)

			return spec
		}
		require.NoError(t, s.cpu.launch(read()))
		synctest.Wait()
		s.requirePhase(runtime.CPUPartitionPhaseReady)
		require.Equal(t, "1-3", s.fs.mask("kubepods"))

		policy := kubernetesOnlyPolicy()
		policy.TypedSpec().Roots["kubepods"] = "1-2"
		s.publish(policy)
		synctest.Wait()
		require.Contains(t, s.requirePhase(runtime.CPUPartitionPhaseConverging).Waiting, "binding")
		require.Equal(t, "1-3", s.fs.mask("kubepods"), "an old task after a failed stop cannot acknowledge newly rendered arguments")
		s.fs.do(func(fs *fakeCgroupFS) { fs.leaves = map[string]cpuset.CPUSet{"pod/container": mustCPUs("3")} })
		require.NoError(t, s.cpu.launch(read()))
		synctest.Wait()
		require.Contains(t, s.requirePhase(runtime.CPUPartitionPhaseConverging).Waiting, "leaf")
		require.Equal(t, "1-3", s.fs.mask("kubepods"))
		s.fs.do(func(fs *fakeCgroupFS) { fs.leaves = nil })
		synctest.Sleep(time.Second)
		synctest.Wait()
		s.requirePhase(runtime.CPUPartitionPhaseReady)
		require.Equal(t, "1-2", s.fs.mask("kubepods"))
	})
}

func TestCPUPartitionObservedSingletonRecreation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.kubelet("0,4-7")
		s.start()
		synctest.Wait()

		old, err := safe.StateGetByID[*k8s.KubeletSpec](s.Ctx(), s.State(), k8s.KubeletID)
		require.NoError(t, err)
		s.Destroy(old)
		synctest.Sleep(time.Nanosecond)

		recreated := k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID)
		*recreated.TypedSpec() = old.TypedSpec().DeepCopy()
		recreated.Metadata().Annotations().Set(k8s.KubeletCPUManagedAnnotation, "true")
		s.Create(recreated)
		s.publish(kubernetesOnlyPolicy())
		synctest.Wait()
		require.Contains(t, s.requirePhase(runtime.CPUPartitionPhaseConverging).Waiting, "binding")
		require.Empty(t, s.fs.mask("kubepods"))
		current, err := safe.StateGetByID[*k8s.KubeletSpec](s.Ctx(), s.State(), k8s.KubeletID)
		require.NoError(t, err)
		require.Equal(t, old.Metadata().Version(), current.Metadata().Version())
		require.NotEqual(t, k8s.KubeletSpecToken(old), k8s.KubeletSpecToken(current))
		require.NoError(t, s.cpu.launch(current))
		synctest.Wait()
		s.requirePhase(runtime.CPUPartitionPhaseReady)
	})
}

func TestCPUPartitionObservedGenerationABA(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.kubelet("0,4-7")
		s.start()
		synctest.Wait()

		old, err := safe.StateGetByID[*k8s.KubeletSpec](s.Ctx(), s.State(), k8s.KubeletID)
		require.NoError(t, err)

		for _, image := range []string{"B", ""} {
			require.NoError(t, safe.StateModify(s.Ctx(), s.State(), old, func(spec *k8s.KubeletSpec) error {
				spec.TypedSpec().Image = image

				return nil
			}))
		}

		s.publish(kubernetesOnlyPolicy())
		synctest.Wait()
		require.Contains(t, s.requirePhase(runtime.CPUPartitionPhaseConverging).Waiting, "binding")
		require.Empty(t, s.fs.mask("kubepods"), "equal CPU arguments do not identify a source generation")
		current, err := safe.StateGetByID[*k8s.KubeletSpec](s.Ctx(), s.State(), k8s.KubeletID)
		require.NoError(t, err)
		require.NoError(t, s.cpu.launch(current))
		synctest.Wait()
		s.requirePhase(runtime.CPUPartitionPhaseReady)
	})
}
