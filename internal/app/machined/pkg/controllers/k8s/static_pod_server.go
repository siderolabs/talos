// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// StaticPodServerController renders all static pod definitions as a PodList and serves it as YAML via HTTP.
type StaticPodServerController struct {
	podList   []byte
	podListMu sync.Mutex

	staticPodVersions map[string]string
}

// Name implements controller.Controller interface.
func (ctrl *StaticPodServerController) Name() string {
	return "k8s.StaticPodServerController"
}

// Inputs implements controller.Controller interface.
func (ctrl *StaticPodServerController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.StaticPodType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *StaticPodServerController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.StaticPodServerStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

type pod map[string]any

type podList struct {
	Kind string `json:"kind,omitempty" protobuf:"bytes,1,opt,name=kind"`

	Items []pod `json:"items" protobuf:"bytes,2,rep,name=items"`

	APIVersion string `json:"apiVersion,omitempty" protobuf:"bytes,3,opt,name=apiVersion"`
}

// Run implements controller.Controller interface.
func (ctrl *StaticPodServerController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	ctrl.staticPodVersions = map[string]string{}

	shutdownServer, serverError, err := ctrl.createServer(ctx, r, logger)
	if err != nil {
		return fmt.Errorf("failed to start http server to serve static pod list: %w", err)
	}

	defer shutdownServer()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-serverError:
			return fmt.Errorf("http server closed unexpectedly: %w", err)
		case <-r.EventCh():
			staticPodList, err := ctrl.buildPodList(ctx, r, logger)
			if err != nil {
				logger.Error("error building static pod list", zap.Error(err))
			}

			ctrl.podListMu.Lock()
			ctrl.podList = staticPodList
			ctrl.podListMu.Unlock()
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *StaticPodServerController) buildPodList(ctx context.Context, r controller.Runtime, logger *zap.Logger) ([]byte, error) {
	staticPods, err := safe.ReaderListAll[*k8s.StaticPod](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("error listing static pods: %w", err)
	}

	pl := podList{
		Kind:       "PodList",
		APIVersion: "v1",
	}

	touchedPodIDs := map[string]struct{}{}

	for staticPod := range staticPods.All() {
		id := staticPod.Metadata().ID()
		version := staticPod.Metadata().Version().String()

		if oldVersion, exists := ctrl.staticPodVersions[id]; !exists || oldVersion != version {
			ctrl.staticPodVersions[id] = version

			if !exists {
				logger.Info("rendered new static pod", zap.String("id", id))
			} else {
				logger.Info("rendered updated static pod", zap.String("id", id), zap.String("old_version", oldVersion), zap.String("new_version", version))
			}
		}

		staticPodSpec := staticPod.TypedSpec()

		pl.Items = append(pl.Items, staticPodSpec.Pod)

		touchedPodIDs[id] = struct{}{}
	}

	for id := range ctrl.staticPodVersions {
		if _, exists := touchedPodIDs[id]; exists {
			continue
		}

		logger.Info("removed static pod", zap.String("id", id))

		delete(ctrl.staticPodVersions, id)
	}

	manifestContent, err := yaml.Marshal(pl)
	if err != nil {
		return nil, fmt.Errorf("error rendering list of static pods as yaml: %w", err)
	}

	return manifestContent, nil
}

func (ctrl *StaticPodServerController) createServer(ctx context.Context, r controller.Runtime, logger *zap.Logger) (func(), <-chan error, error) {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ctrl.podListMu.Lock()
		staticPodList := ctrl.podList
		ctrl.podListMu.Unlock()

		logger.Debug("serving static pod manifests", zap.Int("size", len(staticPodList)))

		if staticPodList == nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

			return
		}

		_, err := w.Write(staticPodList)
		if err != nil {
			logger.Error("failed to serve static pod manifests", zap.Error(err))
		}
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create listener for serving static pod manifests: %w", err)
	}

	httpServer := &http.Server{
		Handler: mux,
	}

	shutdownServer := func() {
		if err := httpServer.Shutdown(ctx); err != nil {
			logger.Error("failed to shut down HTTP server, serving static pod manifests", zap.Error(err))
		}
	}

	go func() {
		<-ctx.Done()

		shutdownServer()
	}()

	if err := safe.WriterModify(ctx, r, k8s.NewStaticPodServerStatus(k8s.NamespaceName, k8s.StaticPodServerStatusResourceID), func(r *k8s.StaticPodServerStatus) error {
		url := fmt.Sprintf("http://%s", listener.Addr().String())

		r.TypedSpec().URL = url

		return nil
	}); err != nil {
		return nil, nil, fmt.Errorf("error modifying StaticPodListURL resource: %w", err)
	}

	serverError := make(chan error, 1)

	go func() {
		serverError <- httpServer.Serve(listener)
	}()

	return shutdownServer, serverError, nil
}
