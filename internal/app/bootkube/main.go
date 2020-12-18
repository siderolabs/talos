// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-multierror"
	"github.com/kubernetes-sigs/bootkube/pkg/bootkube"
	"github.com/kubernetes-sigs/bootkube/pkg/util"

	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

var (
	strict        *bool
	recover       *bool
	recoverSource *string
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)

	strict = flag.Bool("strict", true, "require all manifests to cleanly apply")
	recover = flag.Bool("recover", false, "run recovery instead of generate")
	recoverSource = flag.String("recover-source", "ETCD", "recovery source to use")

	flag.Parse()
}

//nolint: gocyclo
func run() error {
	util.InitLogs()

	defer util.FlushLogs()

	config, err := configloader.NewFromStdin()
	if err != nil {
		return err
	}

	if *recover {
		if err = recoverAssets(config); err != nil {
			return fmt.Errorf("error recovering assets: %w", err)
		}
	} else {
		if err = generateAssets(config); err != nil {
			return fmt.Errorf("error generating assets: %w", err)
		}
	}

	if err = os.MkdirAll(constants.ManifestsDirectory, 0o644); err != nil {
		return err
	}

	// cleanup manifests which might have been left from previous bootkube run
	cleanupManifests("bootkube-*")                     //nolint: errcheck
	cleanupManifests("kube-system-pod-checkpointer-*") //nolint: errcheck

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

	failed := true

	defer func() {
		// We want to cleanup the manifests directory only if bootkube fails.
		if failed {
			if err = os.RemoveAll(constants.ManifestsDirectory); err != nil {
				log.Printf("failed to cleanup manifests dir %s", constants.ManifestsDirectory)
			}
		}

		if err = os.RemoveAll(constants.AssetsDirectory); err != nil {
			log.Printf("failed to cleanup bootkube assets dir %s", constants.AssetsDirectory)
		}

		if err = cleanupManifests("bootstrap-*"); err != nil {
			log.Printf("%s", err)
		}
	}()

	if err = bk.Run(); err != nil {
		return err
	}

	failed = false

	return nil
}

func cleanupManifests(wildcard string) error {
	bootstrapWildcard := filepath.Join(constants.ManifestsDirectory, wildcard)

	bootstrapFiles, err := filepath.Glob(bootstrapWildcard)
	if err != nil {
		return fmt.Errorf("error finding bootstrap files in manifests dir %s", constants.ManifestsDirectory)
	}

	var multiErr *multierror.Error

	for _, bootstrapFile := range bootstrapFiles {
		if err = os.Remove(bootstrapFile); err != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("error deleting bootstrap file in manifests dir: %s", err))
		}
	}

	return multiErr.ErrorOrNil()
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("bootkube failed: %s", err)
	}
}
