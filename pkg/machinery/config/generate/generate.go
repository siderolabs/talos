// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package generate provides Talos machine configuration generation and client config generation.
//
// Please see the example for more information on using this package.
package generate

import (
	"errors"
	"net/netip"
	"net/url"
	"slices"
	"time"

	"github.com/siderolabs/go-pointer"

	coreconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Input holds info about certs, ips, and node type.
//
//nolint:maligned
type Input struct {
	Options Options

	// ControlplaneEndpoint is the canonical address of the kubernetes control
	// plane.  It can be a DNS name, the IP address of a load balancer, or
	// (default) the IP address of the first controlplane node.  It is NOT
	// multi-valued.  It may optionally specify the port.
	ControlPlaneEndpoint string

	AdditionalSubjectAltNames []string
	AdditionalMachineCertSANs []string

	ClusterName       string
	PodNet            []string
	ServiceNet        []string
	KubernetesVersion string
}

// GetAPIServerSANs returns the formatted list of Subject Alt Name addresses for the API Server.
func (in *Input) GetAPIServerSANs() []string {
	var list []string

	endpointURL, err := url.Parse(in.ControlPlaneEndpoint)
	if err == nil {
		list = append(list, endpointURL.Hostname())
	}

	list = append(list, in.AdditionalSubjectAltNames...)

	return list
}

// NewInput prepares a new Input struct to perform machine config generation.
func NewInput(clustername, endpoint, kubernetesVersion string, opts ...Option) (*Input, error) {
	input := &Input{}
	input.Options = DefaultOptions()

	for _, opt := range opts {
		if err := opt(&input.Options); err != nil {
			return nil, err
		}
	}

	var podNet, serviceNet string

	if addr, addrErr := netip.ParseAddr(endpoint); addrErr == nil && addr.Is6() {
		podNet = constants.DefaultIPv6PodNet
		serviceNet = constants.DefaultIPv6ServiceNet
	} else {
		podNet = constants.DefaultIPv4PodNet
		serviceNet = constants.DefaultIPv4ServiceNet
	}

	if input.Options.SecretsBundle == nil {
		var err error

		input.Options.SecretsBundle, err = secrets.NewBundle(secrets.NewFixedClock(time.Now()), input.Options.VersionContract)
		if err != nil {
			return nil, err
		}
	}

	additionalSubjectAltNames := slices.Clone(input.Options.AdditionalSubjectAltNames)

	if input.Options.DiscoveryEnabled == nil {
		input.Options.DiscoveryEnabled = pointer.To(true)
	}

	input.ClusterName = clustername
	input.KubernetesVersion = kubernetesVersion
	input.AdditionalMachineCertSANs = additionalSubjectAltNames
	input.AdditionalSubjectAltNames = additionalSubjectAltNames
	input.PodNet = []string{podNet}
	input.ServiceNet = []string{serviceNet}
	input.ControlPlaneEndpoint = endpoint
	input.KubernetesVersion = kubernetesVersion

	return input, nil
}

// Config returns the talos config for a given node type.
func (in *Input) Config(t machine.Type) (coreconfig.Provider, error) {
	var (
		documents []config.Document
		err       error
	)

	switch t {
	case machine.TypeInit:
		documents, err = in.init()
	case machine.TypeControlPlane:
		documents, err = in.controlPlane()
	case machine.TypeWorker:
		documents, err = in.worker()
	case machine.TypeUnknown:
		fallthrough
	default:
		return nil, errors.New("failed to determine config type to generate")
	}

	if err != nil {
		return nil, err
	}

	return container.New(documents...)
}

// emptyIf returns empty string if the 2nd argument is empty string, otherwise returns the first argument.
func emptyIf(str, check string) string {
	if check == "" {
		return ""
	}

	return str
}
