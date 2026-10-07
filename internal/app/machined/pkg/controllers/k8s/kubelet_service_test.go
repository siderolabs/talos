// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"

	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type kubeletServiceManager struct {
	stop     func(context.Context) error
	loaded   system.Service
	running  bool
	starts   int
	startErr error
}

func (m *kubeletServiceManager) IsRunning(string) (system.Service, bool, error) {
	if m.loaded == nil {
		return nil, false, errors.New("not loaded")
	}

	return m.loaded, m.running, nil
}

func (m *kubeletServiceManager) Stop(ctx context.Context, _ ...string) error {
	if m.stop != nil {
		if err := m.stop(ctx); err != nil {
			return err
		}
	}

	m.running = false

	return nil
}

func (m *kubeletServiceManager) Load(svcs ...system.Service) []string {
	if m.loaded == nil {
		m.loaded = svcs[0]
	}

	return []string{"kubelet"}
}

func (m *kubeletServiceManager) Start(_ ...string) error {
	m.starts++

	return m.startErr
}

func TestKubeletPrepareWaitsForOldService(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		stopEntered := make(chan struct{})
		oldExited := make(chan struct{})
		manager := &kubeletServiceManager{
			loaded:  &services.Kubelet{},
			running: true,
			stop: func(ctx context.Context) error {
				close(stopEntered)

				select {
				case <-oldExited:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
		}
		ctrl := k8sctrl.KubeletServiceController{V1Alpha1Services: manager}
		prepared := false
		done := make(chan error, 1)

		go func() {
			done <- k8sctrl.PrepareKubeletAndStart(&ctrl, t.Context(), k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID), func() error {
				// This is the same boundary enclosing all file writes and checkpoint cleanup in Run.
				prepared = true

				return nil
			})
		}()

		<-stopEntered
		synctest.Wait()
		require.False(t, prepared, "shared files must not change before the previous service exits")
		require.IsType(t, &services.Kubelet{}, manager.loaded)
		require.Zero(t, manager.starts)
		close(oldExited)
		require.NoError(t, <-done)
		require.True(t, prepared)
		require.IsType(t, &services.Kubelet{}, manager.loaded)
		require.Equal(t, 1, manager.starts)
	})
}

func TestKubeletPreparationFailureDoesNotPublish(t *testing.T) {
	manager := &kubeletServiceManager{loaded: &services.Kubelet{}}
	ctrl := k8sctrl.KubeletServiceController{V1Alpha1Services: manager}
	spec := k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID)
	failure := errors.New("checkpoint cleanup failed")
	require.ErrorIs(t, k8sctrl.PrepareKubeletAndStart(&ctrl, t.Context(), spec, func() error { return failure }), failure)
	require.IsType(t, &services.Kubelet{}, manager.loaded, "keep the loaded service so pending API starts can wait for a retry")
	require.Zero(t, manager.starts)

	// A later successful reconcile replaces the snapshot and uses the usual asynchronous Start.
	require.NoError(t, k8sctrl.PrepareKubeletAndStart(&ctrl, t.Context(), spec, func() error { return nil }))
	require.IsType(t, &services.Kubelet{}, manager.loaded)
	require.Equal(t, 1, manager.starts)
}

func TestKubeletPreparationFailureKeepsServiceRetired(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		kubelet := &services.Kubelet{}
		spec := k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID)
		kubelet.Publish(spec)
		manager := &kubeletServiceManager{loaded: kubelet}
		ctrl := k8sctrl.KubeletServiceController{V1Alpha1Services: manager}
		failure := errors.New("checkpoint cleanup failed")
		require.ErrorIs(t, k8sctrl.PrepareKubeletAndStart(&ctrl, t.Context(), spec, func() error { return failure }), failure)
		require.Same(t, kubelet, manager.loaded)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		done := make(chan error, 1)

		go func() {
			defer func() {
				if recover() != nil {
					done <- errors.New("unprepared launch accessed runtime")
				}
			}()

			done <- kubelet.PreFunc(ctx, nil)
		}()

		synctest.Wait()
		cancel()
		require.ErrorIs(t, <-done, context.Canceled, "failed preparation must not release a pending start")
	})
}

func TestKubeletStopFailureDoesNotPrepare(t *testing.T) {
	manager := &kubeletServiceManager{loaded: &services.Kubelet{}, running: true, stop: func(context.Context) error { return context.Canceled }}
	ctrl := k8sctrl.KubeletServiceController{V1Alpha1Services: manager}
	require.ErrorIs(t, k8sctrl.PrepareKubeletAndStart(&ctrl, t.Context(), k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID), func() error {
		t.Fatal("preparation called after failed stop")

		return nil
	}), context.Canceled)
	require.IsType(t, &services.Kubelet{}, manager.loaded)
	require.Zero(t, manager.starts)
}

func TestKubeletStartErrorPreservesPreparedService(t *testing.T) {
	failure := errors.New("start failed")
	manager := &kubeletServiceManager{startErr: failure}
	ctrl := k8sctrl.KubeletServiceController{V1Alpha1Services: manager}
	require.ErrorIs(t, k8sctrl.PrepareKubeletAndStart(&ctrl, t.Context(), k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID), func() error { return nil }), failure)
	require.IsType(t, &services.Kubelet{}, manager.loaded)
	require.Equal(t, 1, manager.starts)
}
