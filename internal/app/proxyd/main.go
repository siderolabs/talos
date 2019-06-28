/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"context"
	"flag"
	"log"

	"github.com/talos-systems/talos/internal/app/init/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/proxyd/internal/frontend"
	"github.com/talos-systems/talos/internal/pkg/startup"
	"github.com/talos-systems/talos/pkg/userdata"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	pkgnet "github.com/talos-systems/talos/internal/pkg/net"
)

var (
	dataPath *string
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
	dataPath = flag.String("userdata", "", "the path to the user data")
	flag.Parse()
}

func main() {
	if err := startup.RandSeed(); err != nil {
		log.Fatalf("startup: %s", err)
	}

	data, err := userdata.Open(*dataPath)
	if err != nil {
		log.Fatalf("open user data: %v", err)
	}

	bootstrapCtx, bootstrapCancel := context.WithCancel(context.Background())
	r, err := frontend.NewReverseProxy(data.Services.Trustd.Endpoints, bootstrapCancel)
	if err != nil {
		log.Fatalf("failed to initialize the reverse proxy: %v", err)
	}

	// Start up with initial bootstrap config
	go r.Bootstrap(bootstrapCtx)

	// nolint: errcheck
	go func() {
		kubeconfig := "/etc/kubernetes/admin.conf"
		if err = conditions.WaitForFilesToExist(kubeconfig).Wait(context.Background()); err != nil {
			log.Fatalf("failed to find %s: %v", kubeconfig, err)
		}

		config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			log.Fatalf("failed to read config %s: %v", kubeconfig, err)
		}

		// Discover local non loopback ips
		ips, err := pkgnet.IPAddrs()
		if err != nil {
			log.Fatalf("failed to get local address: %v", err)
		}
		if len(ips) == 0 {
			log.Fatalf("no IP address found for local api server")
		}
		ip := ips[0]

		// Overwrite defined host so we can target local apiserver
		// and bypass the admin.conf host which is configured for proxyd
		config.Host = ip.String() + ":6443"

		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			log.Fatalf("failed to generate a client from %s: %v", kubeconfig, err)
		}

		if err = r.Watch(clientset); err != nil {
			log.Fatalf("failed to watch kubernetes api server: %v", err)
		}
	}()

	// nolint: errcheck
	r.Listen(":443")
}

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
}
