// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/kubernetes-sigs/bootkube/pkg/bootkube"
	"github.com/kubernetes-sigs/bootkube/pkg/util"
	"golang.org/x/net/http/httpproxy"

	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/constants"
)

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

func main() {
	configPath := flag.String("config", "", "the path to the config")

	flag.Parse()
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

func init() {
	// Explicitly set the default http client transport
	// to work around our fun proxy.Do once bug.
	// This is the http.DefaultTransport with the Proxy
	// func overridden so that the environment variables
	// with be reread/initialized each time the http call
	// is made.
	http.DefaultClient.Transport = &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return httpproxy.FromEnvironment().ProxyFunc()(req.URL)
		},
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

}
