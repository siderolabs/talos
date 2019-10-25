// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bootkube

import (
	"context"
	"io"

	"github.com/kubernetes-incubator/bootkube/pkg/bootkube"

	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
)

// Service wraps bootkube.
type Service struct{}

// NewService creates new Service.
func NewService() *Service {
	return &Service{}
}

// Main is the entrypoint for bootkube.
func (s *Service) Main(ctx context.Context, config runtime.Configurator, logWriter io.Writer) error {
	defaultRequiredPods := []string{
		"kube-system/pod-checkpointer",
		"kube-system/kube-apiserver",
		"kube-system/kube-scheduler",
		"kube-system/kube-controller-manager",
	}

	cfg := bootkube.Config{
		// TODO(andrewrynhard): Clean this directory up once bootstrap is
		// complete.
		AssetDir:        constants.AssetsDirectory,
		PodManifestPath: "/etc/kubernetes/manifests",
		Strict:          true,
		RequiredPods:    defaultRequiredPods,
	}

	bk, err := bootkube.NewBootkube(cfg)
	if err != nil {
		return err
	}

	return bk.Run()
}
