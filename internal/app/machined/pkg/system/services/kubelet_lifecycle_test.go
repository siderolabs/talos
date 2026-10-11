// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services_test

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// Keep the actual PreFunc/Runner and API permissions, replacing only boot prerequisites.
type preparedLifecycleService struct{ system.Service }

func (preparedLifecycleService) Condition(runtime.Runtime) conditions.Condition { return nil }
func (preparedLifecycleService) DependsOn(runtime.Runtime) []string             { return nil }
func (preparedLifecycleService) Volumes(runtime.Runtime) []string               { return nil }
func (preparedLifecycleService) APIStartAllowed(runtime.Runtime) bool           { return true }
func (preparedLifecycleService) APIRestartAllowed(runtime.Runtime) bool         { return true }

type kubeletManagerRuntime struct{ kubeletTestRuntime }

func (kubeletManagerRuntime) Events() runtime.EventStream { return kubeletTestEvents{} }

type kubeletTestEvents struct{ runtime.EventStream }

func (kubeletTestEvents) Publish(context.Context, proto.Message) {}

type kubeletTestProcess struct{}

func (kubeletTestProcess) String() string { return "kubelet test process" }
func (kubeletTestProcess) Open() error    { return nil }
func (kubeletTestProcess) Close() error   { return nil }
func (kubeletTestProcess) Run(ctx context.Context, _ events.Recorder, onStart runner.OnStart) (runner.Status, error) {
	onStart(123)
	<-ctx.Done()

	return runner.Status{Started: true}, nil
}

func TestKubeletAPIStartsWaitForPreparation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		st := state.WrapCore(namespaced.NewState(inmem.Build))
		r := kubeletManagerRuntime{kubeletTestRuntime{resources: st}}
		manager := system.Services(r)
		kubelet := &services.Kubelet{}
		pulled := make(chan string, 8)
		launched := make(chan []string, 8)

		kubelet.SetKubeletPullImage(func(_ context.Context, _ runtime.Runtime, image string) (string, error) {
			pulled <- image

			return image, nil
		})
		kubelet.SetKubeletRunnerFactory(func(_ bool, args *runner.Args, _ ...runner.Option) runner.Runner {
			launched <- append([]string(nil), args.ProcessArgs...)

			return kubeletTestProcess{}
		})
		manager.Load(preparedLifecycleService{Service: kubelet})

		a := k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID)
		a.TypedSpec().Image = "image-a"
		a.TypedSpec().Args = []string{"--reserved-cpus=0-1"}
		a.Metadata().Annotations().Set(k8s.KubeletCPUManagedAnnotation, "true")
		kubelet.Publish(a)
		require.NoError(t, manager.APIStart(t.Context(), "kubelet"))
		require.Equal(t, "image-a", <-pulled)
		require.Equal(t, []string{"/usr/local/bin/kubelet", "--reserved-cpus=0-1"}, <-launched)

		kubelet.Retire()
		require.NoError(t, manager.Stop(t.Context(), "kubelet"))
		require.NoError(t, manager.APIStart(t.Context(), "kubelet"))
		synctest.Wait()
		require.Empty(t, pulled, "API start during file preparation must not pull the old image")
		require.Empty(t, launched)

		// A failed preparation retries Retire without stranding the pending API start.
		kubelet.Retire()
		synctest.Wait()
		require.Empty(t, pulled)
		require.NoError(t, manager.APIRestart(t.Context(), "kubelet"))
		synctest.Wait()
		require.Empty(t, pulled, "API restart must cancel its old waiter and wait again")

		b := a.DeepCopy().(*k8s.KubeletSpec)
		b.TypedSpec().Image = "image-b"
		b.TypedSpec().Args[0] = "--reserved-cpus=2-3"
		b.Metadata().Annotations().Delete(k8s.KubeletCPUManagedAnnotation)
		kubelet.Publish(b)
		require.Equal(t, "image-b", <-pulled)
		require.Equal(t, []string{"/usr/local/bin/kubelet", "--reserved-cpus=2-3"}, <-launched)
		require.NoError(t, manager.Unload(t.Context(), "kubelet"))
	})
}

func TestKubeletRepeatedRetireKeepsWaiters(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		kubelet := &services.Kubelet{}
		pulled := errors.New("image pull reached")

		kubelet.SetKubeletPullImage(func(context.Context, runtime.Runtime, string) (string, error) { return "", pulled })

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		done := make(chan error, 1)
		go func() { done <- kubelet.PreFunc(ctx, preparedKubeletRuntime{}) }()

		synctest.Wait()
		kubelet.Retire()
		kubelet.Retire()
		kubelet.Publish(k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID))
		synctest.Wait()

		select {
		case err := <-done:
			require.ErrorIs(t, err, pulled)
		default:
			cancel()
			<-done
			t.Fatal("repeated Retire stranded the pending start")
		}
	})
}

func TestKubeletRetiredPreFuncCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		kubelet := &services.Kubelet{}
		ctx, cancel := context.WithCancel(t.Context())

		done := make(chan error, 1)
		go func() { done <- kubelet.PreFunc(ctx, preparedKubeletRuntime{}) }()

		synctest.Wait()
		kubelet.Retire()
		kubelet.Retire()
		cancel()
		require.ErrorIs(t, <-done, context.Canceled)
	})
}
