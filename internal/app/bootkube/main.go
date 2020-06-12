// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/kubernetes-sigs/bootkube/pkg/bootkube"
	"github.com/kubernetes-sigs/bootkube/pkg/util"

	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/constants"
)

var (
	configPath *string
	strict     *bool
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)

	configPath = flag.String("config", "", "the path to the config")
	strict = flag.Bool("strict", true, "require all manifests to cleanly apply")

	flag.Parse()
}

func run() error {
	defaultRequiredPods := []string{
		"kube-system/pod-checkpointer",
		"kube-system/kube-apiserver",
		"kube-system/kube-scheduler",
		"kube-system/kube-controller-manager",
	}

	cfg := bootkube.Config{
		AssetDir:        constants.AssetsDirectory,
		PodManifestPath: constants.ManifestsDirectory,
		Strict:          *strict,
		RequiredPods:    defaultRequiredPods,
	}

	bk, err := bootkube.NewBootkube(cfg)
	if err != nil {
		return err
	}

	defer func() {
		// We want to cleanup the manifests directory only if bootkube fails.
		if err != nil {
			if err = os.RemoveAll(constants.ManifestsDirectory); err != nil {
				log.Printf("failed to cleanup manifests dir %s", constants.ManifestsDirectory)
			}
		}

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

func main() {
	util.InitLogs()

	defer util.FlushLogs()

	config, err := config.NewFromFile(*configPath)
	if err != nil {
		log.Fatalf("failed to create config from file: %v", err)
	}

	if err := generateAssets(config); err != nil {
		log.Fatalf("error generating assets: %s", err)
	}

	if err := run(); err != nil {
		log.Fatalf("bootkube failed: %s", err)
	}
}
