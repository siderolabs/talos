// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bootkube

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/kubernetes-sigs/bootkube/pkg/bootkube"

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
		AssetDir:        constants.AssetsDirectory,
		PodManifestPath: constants.ManifestsDirectory,
		Strict:          true,
		RequiredPods:    defaultRequiredPods,
	}

	bk, err := bootkube.NewBootkube(cfg)
	if err != nil {
		return err
	}

	defer func() {
		if err = os.RemoveAll(constants.AssetsDirectory); err != nil {
			log.Printf("failed to cleanup bootkube assets dir %s", constants.AssetsDirectory)
		}

		bootstrapWildcard := filepath.Join(constants.ManifestsDirectory, "bootstrap-*")

		bootstrapFiles, err := filepath.Glob(bootstrapWildcard)
		if err != nil {
			log.Printf("error finding bootstrap files in manifests dir %s", constants.ManifestsDirectory)
		}

		for _, bootstrapFile := range bootstrapFiles {
			if err := os.Remove(bootstrapFile); err != nil {
				log.Printf("error deleting bootstrap file in manifests dir : %s", err)
			}
		}
	}()

	return bk.Run()
}
