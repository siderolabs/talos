// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"flag"
	"log"
	"regexp"
	"strings"

	"github.com/talos-systems/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/talos-systems/talos/internal/app/apid/pkg/backend"
	"github.com/talos-systems/talos/internal/app/apid/pkg/director"
	"github.com/talos-systems/talos/internal/app/apid/pkg/provider"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/startup"
)

var (
	configPath *string
	endpoints  *string
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)

	configPath = flag.String("config", "", "the path to the config")
	endpoints = flag.String("endpoints", "", "the IPs of the control plane nodes")

	flag.Parse()
}

func main() {
	if err := startup.RandSeed(); err != nil {
		log.Fatalf("failed to seed RNG: %v", err)
	}

	config, err := loadConfig()
	if err != nil {
		log.Fatalf("open config: %v", err)
	}

	tlsConfig, err := provider.NewTLSConfig(config, strings.Split(*endpoints, ","))
	if err != nil {
		log.Fatalf("failed to create remote certificate provider: %+v", err)
	}

	serverTLSConfig, err := tlsConfig.ServerConfig()
	if err != nil {
		log.Fatalf("failed to create OS-level TLS configuration: %v", err)
	}

	clientTLSConfig, err := tlsConfig.ClientConfig()
	if err != nil {
		log.Fatalf("failed to create client TLS config: %v", err)
	}

	backendFactory := backend.NewAPIDFactory(clientTLSConfig)
	router := director.NewRouter(backendFactory.Get)

	router.RegisterLocalBackend("os.OS", backend.NewLocal("osd", constants.OSSocketPath))
	router.RegisterLocalBackend("machine.Machine", backend.NewLocal("machined", constants.MachineSocketPath))
	router.RegisterLocalBackend("time.Time", backend.NewLocal("timed", constants.TimeSocketPath))
	router.RegisterLocalBackend("network.Network", backend.NewLocal("networkd", constants.NetworkSocketPath))

	// all existing streaming methods
	for _, methodName := range []string{
		"/machine.Machine/CopyOut",
		"/machine.Machine/Kubeconfig",
		"/machine.Machine/LS",
		"/machine.Machine/Logs",
		"/machine.Machine/Read",
	} {
		router.RegisterStreamedRegex("^" + regexp.QuoteMeta(methodName) + "$")
	}

	// register future pattern: method should have suffix "Stream"
	router.RegisterStreamedRegex("Stream$")

	err = factory.ListenAndServe(
		router,
		factory.Port(constants.ApidPort),
		factory.WithDefaultLog(),
		factory.ServerOptions(
			grpc.Creds(
				credentials.NewTLS(serverTLSConfig),
			),
			grpc.CustomCodec(proxy.Codec()),
			grpc.UnknownServiceHandler(
				proxy.TransparentHandler(
					router.Director,
					proxy.WithStreamedDetector(router.StreamedDetector),
				)),
		),
	)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
}

func loadConfig() (runtime.Configurator, error) {
	content, err := config.FromFile(*configPath)
	if err != nil {
		return nil, err
	}

	return config.New(content)
}
