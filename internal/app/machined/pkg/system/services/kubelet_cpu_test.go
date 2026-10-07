// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services_test

import (
	"context"
	"testing"
	"testing/synctest"

	"github.com/containerd/containerd/v2/core/containers"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type preparedKubeletRuntime struct {
	runtime.Runtime
}

func (preparedKubeletRuntime) State() runtime.State {
	panic("Runner must not read the latest KubeletSpec after preparation")
}

func (preparedKubeletRuntime) Config() config.Config {
	return container.NewV1Alpha1(&v1alpha1.Config{})
}

func (preparedKubeletRuntime) Logging() runtime.LoggingManager { return nil }

func newPreparedKubelet(spec *k8s.KubeletSpec) *services.Kubelet {
	kubelet := &services.Kubelet{}
	kubelet.Publish(spec)

	return kubelet
}

func TestKubeletRunnerUsesPreparedSnapshot(t *testing.T) {
	kubelet := newPreparedKubelet(k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID))
	kubelet.SetKubeletPullImage(func(context.Context, runtime.Runtime, string) (string, error) { return "image-a", nil })
	require.NoError(t, kubelet.PreFunc(t.Context(), kubeletTestRuntime{resources: state.WrapCore(namespaced.NewState(inmem.Build))}))

	require.NotPanics(t, func() {
		_, err := kubelet.Runner(preparedKubeletRuntime{})
		require.NoError(t, err)
	})
}

type kubeletTestRuntime struct {
	preparedKubeletRuntime
	resources state.State
}

func (r kubeletTestRuntime) State() runtime.State {
	return kubeletTestState{resources: r.resources}
}

type kubeletTestState struct {
	runtime.State
	resources state.State
}

func (s kubeletTestState) V1Alpha2() runtime.V1Alpha2State {
	return kubeletTestV2State{resources: s.resources}
}

type kubeletTestV2State struct {
	runtime.V1Alpha2State
	resources state.State
}

func (s kubeletTestV2State) Resources() state.State { return s.resources }

func TestKubeletPreparedLifecycle(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		st := state.WrapCore(namespaced.NewState(inmem.Build))
		r := kubeletTestRuntime{resources: st}
		a := k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID)
		a.TypedSpec().Image = "image-a"
		a.TypedSpec().Args = []string{"--reserved-cpus=0-1"}
		a.TypedSpec().ExtraMounts = []specs.Mount{{Source: t.TempDir(), Destination: "/test-prepared-mount", Type: "bind", Options: []string{"ro"}}}
		a.Metadata().Annotations().Set(k8s.KubeletCPUManagedAnnotation, "true")
		require.NoError(t, st.Create(ctx, a))
		token := k8s.KubeletSpecToken(a)
		kubelet := newPreparedKubelet(a)

		pullEntered := make(chan struct{})
		releasePull := make(chan struct{})
		pulledImages := []string{}

		kubelet.SetKubeletPullImage(func(_ context.Context, _ runtime.Runtime, image string) (string, error) {
			pulledImages = append(pulledImages, image)
			if len(pulledImages) == 1 {
				close(pullEntered)
				<-releasePull
			}

			return "digest-a", nil
		})

		preDone := make(chan error, 1)
		go func() { preDone <- kubelet.PreFunc(ctx, r) }()

		<-pullEntered

		// A newer desired spec arrives while A's asynchronous start is still preparing.
		// Also mutate the original object, proving the constructor owns a deep copy.
		a.TypedSpec().Image = "image-b"
		a.TypedSpec().Args[0] = "--reserved-cpus=2-3"
		a.TypedSpec().ExtraMounts[0].Options[0] = "rw"
		a.Metadata().Annotations().Delete(k8s.KubeletCPUManagedAnnotation)
		require.NoError(t, st.Update(ctx, a))
		close(releasePull)
		require.NoError(t, <-preDone)

		var (
			capturedArgs []string
			capturedOpts runner.Options
		)

		factory := func(_ bool, args *runner.Args, options ...runner.Option) runner.Runner {
			capturedArgs = append([]string(nil), args.ProcessArgs...)

			capturedOpts = runner.Options{}
			for _, option := range options {
				option(&capturedOpts)
			}

			return nil
		}
		kubelet.SetKubeletRunnerFactory(factory)

		for range 2 {
			_, err := kubelet.Runner(r)
			require.NoError(t, err)
			require.Equal(t, []string{"/usr/local/bin/kubelet", "--reserved-cpus=0-1"}, capturedArgs)
			require.Equal(t, "digest-a", capturedOpts.ContainerImage)

			ociSpec := specs.Spec{Linux: &specs.Linux{Resources: &specs.LinuxResources{}}, Process: &specs.Process{}}
			for _, option := range capturedOpts.OCISpecOpts {
				require.NoError(t, option(ctx, nil, &containers.Container{}, &ociSpec))
			}

			require.Equal(t, []string{"ro"}, ociSpec.Mounts[len(ociSpec.Mounts)-1].Options)
			require.Equal(t, token, ociSpec.Annotations[k8s.KubeletSpecTokenAnnotation])
			require.Equal(t, "true", ociSpec.Annotations[k8s.KubeletCPUManagedAnnotation])

			// A service retry must not silently adopt B either.
			require.NoError(t, kubelet.PreFunc(ctx, r))
		}

		require.Equal(t, []string{"image-a", "image-a", "image-a"}, pulledImages)

		// Once B has its own preparation, a replacement may start with B, including
		// the managed-to-unmanaged transition. It must not inherit A's binding.
		replacement := newPreparedKubelet(a)
		replacement.SetKubeletPullImage(func(_ context.Context, _ runtime.Runtime, image string) (string, error) {
			require.Equal(t, "image-b", image)

			return "digest-b", nil
		})
		replacement.SetKubeletRunnerFactory(factory)
		require.NoError(t, replacement.PreFunc(ctx, r))
		_, err := replacement.Runner(r)
		require.NoError(t, err)
		require.Equal(t, []string{"/usr/local/bin/kubelet", "--reserved-cpus=2-3"}, capturedArgs)
		require.Equal(t, "digest-b", capturedOpts.ContainerImage)

		ociSpec := specs.Spec{Linux: &specs.Linux{Resources: &specs.LinuxResources{}}, Process: &specs.Process{}}
		for _, option := range capturedOpts.OCISpecOpts {
			require.NoError(t, option(ctx, nil, &containers.Container{}, &ociSpec))
		}

		require.Equal(t, []string{"rw"}, ociSpec.Mounts[len(ociSpec.Mounts)-1].Options)
		require.Equal(t, k8s.KubeletSpecToken(a), ociSpec.Annotations[k8s.KubeletSpecTokenAnnotation])
		require.Equal(t, "false", ociSpec.Annotations[k8s.KubeletCPUManagedAnnotation])
	})
}

func TestKubeletManagedLaunchRemainsLatched(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := kubeletTestRuntime{resources: state.WrapCore(namespaced.NewState(inmem.Build))}
		a := k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID)
		a.TypedSpec().Image = "image-a"
		a.TypedSpec().Args = []string{"--reserved-cpus=0-1"}
		a.Metadata().Annotations().Set(k8s.KubeletCPUManagedAnnotation, "true")
		require.NoError(t, r.resources.Create(t.Context(), a))
		aToken := k8s.KubeletSpecToken(a)
		kubelet := newPreparedKubelet(a)
		pullEntered, releasePull := make(chan struct{}), make(chan struct{})

		kubelet.SetKubeletPullImage(func(_ context.Context, _ runtime.Runtime, image string) (string, error) {
			close(pullEntered)
			<-releasePull

			return image, nil
		})

		done := make(chan error, 1)
		go func() { done <- kubelet.PreFunc(t.Context(), r) }()

		<-pullEntered

		b := a.DeepCopy().(*k8s.KubeletSpec)
		b.TypedSpec().Image = "image-b"
		b.TypedSpec().Args[0] = "--reserved-cpus=2-3"
		require.NoError(t, r.resources.Update(t.Context(), b))
		kubelet.Retire()
		kubelet.Publish(b)
		close(releasePull)
		require.NoError(t, <-done)

		kubelet.SetKubeletRunnerFactory(func(_ bool, args *runner.Args, options ...runner.Option) runner.Runner {
			require.Equal(t, []string{"/usr/local/bin/kubelet", "--reserved-cpus=0-1"}, args.ProcessArgs)

			var captured runner.Options
			for _, option := range options {
				option(&captured)
			}

			require.Equal(t, "image-a", captured.ContainerImage)

			ociSpec := specs.Spec{Linux: &specs.Linux{Resources: &specs.LinuxResources{}}, Process: &specs.Process{}}
			for _, option := range captured.OCISpecOpts {
				require.NoError(t, option(t.Context(), nil, &containers.Container{}, &ociSpec))
			}

			require.Equal(t, aToken, ociSpec.Annotations[k8s.KubeletSpecTokenAnnotation])
			require.Equal(t, "true", ociSpec.Annotations[k8s.KubeletCPUManagedAnnotation])

			return nil
		})
		_, err := kubelet.Runner(r)
		require.NoError(t, err)
	})
}

func TestKubeletCPUArgumentsSameSnapshot(t *testing.T) {
	spec := k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID)
	spec.TypedSpec().Args = []string{"--reserved-cpus=0-1"}
	spec.Metadata().Annotations().Set(k8s.KubeletCPUManagedAnnotation, "true")
	token := k8s.KubeletSpecToken(spec)
	args, option := services.KubeletCPUArguments(spec)
	spec.TypedSpec().Args[0] = "--reserved-cpus=2-3"
	spec.Metadata().Annotations().Delete(k8s.KubeletCPUManagedAnnotation)

	version, err := resource.ParseVersion("2")
	require.NoError(t, err)
	spec.Metadata().SetVersion(version)

	var ociSpec specs.Spec
	require.NoError(t, option(context.Background(), nil, &containers.Container{}, &ociSpec))
	require.Equal(t, []string{"/usr/local/bin/kubelet", "--reserved-cpus=0-1"}, args)
	require.Equal(t, token, ociSpec.Annotations[k8s.KubeletSpecTokenAnnotation])
	require.Equal(t, "true", ociSpec.Annotations[k8s.KubeletCPUManagedAnnotation])
	require.NotEqual(t, k8s.KubeletSpecToken(spec), token)
}
