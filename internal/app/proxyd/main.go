/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"context"
	"flag"
	"log"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/proxyd/internal/frontend"
	"github.com/talos-systems/talos/internal/app/proxyd/internal/reg"
	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/startup"

	pkgnet "github.com/talos-systems/talos/pkg/net"
)

var configPath *string

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
	configPath = flag.String("config", "", "the path to the config")
	flag.Parse()
}

func main() {
	if err := startup.RandSeed(); err != nil {
		log.Fatalf("startup: %s", err)
	}

	content, err := config.FromFile(*configPath)
	if err != nil {
		log.Fatalf("open config: %v", err)
	}
	config, err := config.New(content)
	if err != nil {
		log.Fatalf("open config: %v", err)
	}

	bootstrapCtx, bootstrapCancel := context.WithCancel(context.Background())
	r, err := frontend.NewReverseProxy(config.Cluster().IPs(), bootstrapCancel)
	if err != nil {
		log.Fatalf("failed to initialize the reverse proxy: %v", err)
	}

	// Start up with initial bootstrap config
	go r.Bootstrap(bootstrapCtx)

	go waitForKube(r)

	errch := make(chan error)

	// Start up reverse proxy
	go func() {
		errch <- r.Listen(":443")
	}()

	// Start up gRPC server
	go func() {
		errch <- factory.ListenAndServe(
			reg.NewRegistrator(r),
			factory.Network("unix"),
			factory.SocketPath(constants.ProxydSocketPath),
		)
	}()

	log.Fatal(<-errch)
}

func waitForKube(r *frontend.ReverseProxy) {
	kubeconfig := "/etc/kubernetes/admin.conf"
	if err := conditions.WaitForFilesToExist(kubeconfig).Wait(context.Background()); err != nil {
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
}
