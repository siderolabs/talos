// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package basicintegration

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/talos-systems/talos/pkg/constants"
)

// Option allows for the configuration of the BasicIntegration.
type Option func(*BasicIntegration) error

// defaultOptions sets a default set of options to be used for the container.
func defaultOptions() *BasicIntegration {
	tmpPath, err := ioutil.TempDir("", "e2e")
	if err != nil {
		log.Fatal(err)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	return &BasicIntegration{
		osctl:           filepath.Join(currentDir, "build", strings.Join([]string{"osctl", runtime.GOOS, "amd64"}, "-")),
		integrationTest: filepath.Join(currentDir, "bin", "integration-test"),
		containerImage:  constants.KubernetesImage + ":v" + constants.DefaultKubernetesVersion,
		tmpDir:          tmpPath,
		kubeConfig:      filepath.Join(tmpPath, "kubeconfig"),
		talosConfig:     filepath.Join(tmpPath, "talosconfig"),
	}
}

// WithCleanup specifies if cleanup should be performed after the tests
// run.
func WithCleanup(o bool) Option {
	return func(b *BasicIntegration) (err error) {
		b.cleanup = o

		return err
	}
}

// WithContainerImage specified the container that should be used when
// performing container based checks.
func WithContainerImage(o string) Option {
	return func(b *BasicIntegration) (err error) {
		if o != "" {
			b.containerImage = o
		}

		return err
	}
}

// WithClusterName specifies the name of the cluster.
// This is used during cluster creation for docker network checks as well
// as runner container tags.
func WithClusterName(o string) Option {
	return func(b *BasicIntegration) (err error) {
		b.clusterName = o

		return err
	}
}

// WithTalosConfig specifies an existing location to a talosconfig.
func WithTalosConfig(o string) Option {
	return func(b *BasicIntegration) (err error) {
		// Not sure if we want to do this yet
		// since the cluster may or may not exist
		/*
			if !checkFile(o)		{
				return fmt.Errorf("%s is not a file or does not exist", o)
			}
		*/

		if o != "<tmpdir>/e2e*/talosconfig" {
			b.talosConfig = o
		}

		return err
	}
}

// WithKubeConfig specifies an existing kubeconfig.
func WithKubeConfig(o string) Option {
	return func(b *BasicIntegration) (err error) {
		if o != "<tmpdir>/e2e*/kubeconfig" {
			b.kubeConfig = o
		}

		return err
	}
}

// WithOsctl specifies the location of the osctl binary.
func WithOsctl(o string) Option {
	return func(b *BasicIntegration) (err error) {
		if !checkFile(o) {
			return fmt.Errorf("%s is not a file or does not exist", o)
		}

		if o != "" {
			b.osctl = o
		}

		return err
	}
}

// WithIntegrationTester specifies the location of the tester binary.
func WithIntegrationTester(o string) Option {
	return func(b *BasicIntegration) (err error) {
		if !checkFile(o) {
			return fmt.Errorf("%s is not a file or does not exist", o)
		}

		if o != "" {
			b.integrationTest = o
		}

		return err
	}
}

// WithTalosImage specifies the version of the talos container to use for
// cluster creation.
func WithTalosImage(o string) Option {
	return func(b *BasicIntegration) (err error) {
		if o != "" {
			b.talosImage = o
		}

		return err
	}
}

func checkFile(filename string) bool {
	f, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}

	if err != nil {
		return false
	}

	return !f.IsDir()
}
