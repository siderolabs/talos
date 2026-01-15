// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/cri"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

//nolint:gocyclo,cyclop
func (in *Input) generateRegistryConfigs(machine *v1alpha1.MachineConfig) ([]config.Document, error) {
	if !in.Options.VersionContract.MultidocNetworkConfigSupported() {
		// old-style registry config
		machine.MachineRegistries = v1alpha1.RegistriesConfig{ //nolint:staticcheck // backwards compatibility
			RegistryMirrors: map[string]*v1alpha1.RegistryMirrorConfig{},
			RegistryConfig:  map[string]*v1alpha1.RegistryConfig{},
		}

		if in.Options.VersionContract.KubernetesAlternateImageRegistries() {
			if _, ok := machine.MachineRegistries.RegistryMirrors["k8s.gcr.io"]; !ok { //nolint:staticcheck // backwards compatibility, Talos v1.1->1.2
				machine.MachineRegistries.RegistryMirrors["k8s.gcr.io"] = &v1alpha1.RegistryMirrorConfig{ //nolint:staticcheck // backwards compatibility, Talos v1.1->1.2
					MirrorEndpoints: []string{
						"https://registry.k8s.io",
						"https://k8s.gcr.io",
					},
				}
			}
		}

		for host, endpoints := range in.Options.RegistryEndpoints {
			machine.MachineRegistries.RegistryMirrors[host] = &v1alpha1.RegistryMirrorConfig{ //nolint:staticcheck // backwards compatibility
				MirrorEndpoints: endpoints,
			}
		}

		for host, cacert := range in.Options.RegistryCACerts {
			if _, ok := machine.MachineRegistries.RegistryConfig[host]; !ok { //nolint:staticcheck // backwards compatibility
				machine.MachineRegistries.RegistryConfig[host] = &v1alpha1.RegistryConfig{} //nolint:staticcheck // backwards compatibility
			}

			if machine.MachineRegistries.RegistryConfig[host].RegistryTLS == nil { //nolint:staticcheck // backwards compatibility
				machine.MachineRegistries.RegistryConfig[host].RegistryTLS = &v1alpha1.RegistryTLSConfig{} //nolint:staticcheck // backwards compatibility
			}

			machine.MachineRegistries.RegistryConfig[host].RegistryTLS.TLSCA = v1alpha1.Base64Bytes(cacert) //nolint:staticcheck // backwards compatibility
		}

		for host := range in.Options.RegistryInsecure {
			if _, ok := machine.MachineRegistries.RegistryConfig[host]; !ok { //nolint:staticcheck // backwards compatibility
				machine.MachineRegistries.RegistryConfig[host] = &v1alpha1.RegistryConfig{} //nolint:staticcheck // backwards compatibility
			}

			if machine.MachineRegistries.RegistryConfig[host].RegistryTLS == nil { //nolint:staticcheck // backwards compatibility
				machine.MachineRegistries.RegistryConfig[host].RegistryTLS = &v1alpha1.RegistryTLSConfig{} //nolint:staticcheck // backwards compatibility
			}

			machine.MachineRegistries.RegistryConfig[host].RegistryTLS.TLSInsecureSkipVerify = pointer.To(true) //nolint:staticcheck // backwards compatibility
		}

		return nil, nil
	}

	documents := make([]config.Document, 0, len(in.Options.RegistryEndpoints))

	// use new-style registry config via separate documents
	for host, endpoints := range in.Options.RegistryEndpoints {
		registryMirrorConfig := cri.NewRegistryMirrorConfigV1Alpha1(host)
		registryMirrorConfig.RegistryEndpoints = make([]cri.RegistryEndpoint, 0, len(endpoints))

		for _, ep := range endpoints {
			u, err := url.Parse(ep)
			if err != nil {
				return nil, fmt.Errorf("failed to parse registry mirror endpoint %q: %w", ep, err)
			}

			registryMirrorConfig.RegistryEndpoints = append(registryMirrorConfig.RegistryEndpoints, cri.RegistryEndpoint{
				EndpointURL: meta.URL{URL: u},
			})
		}

		documents = append(documents, registryMirrorConfig)
	}

	tlsConfigs := make(map[string]*cri.RegistryTLSConfigV1Alpha1)

	for host, cacert := range in.Options.RegistryCACerts {
		if _, ok := tlsConfigs[host]; !ok {
			tlsConfigs[host] = cri.NewRegistryTLSConfigV1Alpha1(host)
		}

		tlsConfigs[host].TLSCA = cacert
	}

	for host := range in.Options.RegistryInsecure {
		if _, ok := tlsConfigs[host]; !ok {
			tlsConfigs[host] = cri.NewRegistryTLSConfigV1Alpha1(host)
		}

		tlsConfigs[host].TLSInsecureSkipVerify = pointer.To(true)
	}

	for _, tlsConfig := range tlsConfigs {
		documents = append(documents, tlsConfig)
	}

	// sort the TLS config and registry mirrors docs alphabetically by the name
	slices.SortStableFunc(documents, func(a, b config.Document) int {
		na, aok := a.(config.NamedDocument)
		nb, bok := b.(config.NamedDocument)

		if c := strings.Compare(a.Kind(), b.Kind()); c != 0 {
			return c
		}

		if aok && bok {
			return strings.Compare(na.Name(), nb.Name())
		}

		return 0
	})

	return documents, nil
}
