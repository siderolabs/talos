/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"log"

	"github.com/talos-systems/talos/internal/app/proxyd/internal/frontend"
	pkgnet "github.com/talos-systems/talos/internal/pkg/net"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {

	// Discovey local non loopback ips
	ips, err := pkgnet.IPAddrs()
	if err != nil {
		log.Fatalf("failed to get local address: %v", err)
	}
	if len(ips) == 0 {
		log.Fatalf("no IP address found for bootstrap backend")
	}
	ip := ips[0]

	r, err := frontend.NewReverseProxy(ip)
	if err != nil {
		log.Fatalf("failed to initialize the reverse proxy: %v", err)
	}

	// Set up kubernetes client
	kubeconfig := "/etc/kubernetes/admin.conf"
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return
	}

	// Update the host to the node's IP.
	config.Host = ip.String() + ":6443"

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return
	}

	go r.Watch(clientset)

	// nolint: errcheck
	r.Listen(":443")
}

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
}
