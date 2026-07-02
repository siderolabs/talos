// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rotatepatcher

import (
	"bytes"
	"slices"

	"github.com/siderolabs/crypto/x509"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

// PatcherFunc is the actual machine config patcher function.
type PatcherFunc func(config.Provider) (config.Provider, error)

// K8sAddAcceptedCA adds the specified accepted CA to the machine configuration.
func K8sAddAcceptedCA(caCrt []byte) PatcherFunc {
	return func(in config.Provider) (config.Provider, error) {
		// detect version contract, and act accordingly
		multidoc := in.Has(k8s.KubeAPIServerCAConfig)

		if !multidoc {
			return in.PatchV1Alpha1(func(cfg *v1alpha1.Config) error {
				cfg.ClusterConfig.ClusterAcceptedCAs = append( //nolint:staticcheck // legacy config
					cfg.ClusterConfig.ClusterAcceptedCAs, //nolint:staticcheck // legacy config
					&x509.PEMEncodedCertificate{
						Crt: caCrt,
					},
				)

				return nil
			})
		}

		return container.PatchDocument(
			in,
			func(caCfg *k8s.KubeAPIServerCAConfigV1Alpha1) error {
				if slices.ContainsFunc(caCfg.APIAcceptedCAs, func(ca string) bool {
					return bytes.Equal([]byte(ca), caCrt)
				}) {
					return nil
				}

				caCfg.APIAcceptedCAs = append(caCfg.APIAcceptedCAs, string(caCrt))

				return nil
			},
		)
	}
}

// K8sDeleteAcceptedCA deletes the specified accepted CA from the machine configuration.
func K8sDeleteAcceptedCA(caCrt []byte) PatcherFunc {
	return func(in config.Provider) (config.Provider, error) {
		// detect version contract, and act accordingly
		multidoc := in.Has(k8s.KubeAPIServerCAConfig)

		if !multidoc {
			return in.PatchV1Alpha1(func(cfg *v1alpha1.Config) error {
				cfg.ClusterConfig.ClusterAcceptedCAs = slices.DeleteFunc( //nolint:staticcheck // legacy config
					cfg.ClusterConfig.ClusterAcceptedCAs, //nolint:staticcheck // legacy config
					func(ca *x509.PEMEncodedCertificate) bool {
						return bytes.Equal(ca.Crt, caCrt)
					},
				)

				return nil
			})
		}

		return container.PatchDocument(
			in,
			func(caCfg *k8s.KubeAPIServerCAConfigV1Alpha1) error {
				caCfg.APIAcceptedCAs = slices.DeleteFunc(
					caCfg.APIAcceptedCAs,
					func(ca string) bool {
						return bytes.Equal([]byte(ca), caCrt)
					},
				)

				return nil
			},
		)
	}
}

// K8sSetCA sets the specified CA to the machine configuration.
//
// This function acts a bit different way for controlplanes and workers:
// * workers just set as effectively accepted
// * controlplanes set as the issuing CA.
func K8sSetCA(newCA *x509.PEMEncodedCertificateAndKey) PatcherFunc {
	return func(in config.Provider) (config.Provider, error) {
		// detect version contract, and act accordingly
		multidoc := in.Has(k8s.KubeAPIServerCAConfig)
		machineType := in.Machine().Type()

		if !multidoc {
			return in.PatchV1Alpha1(func(cfg *v1alpha1.Config) error {
				if machineType.IsControlPlane() {
					cfg.ClusterConfig.ClusterCA = newCA //nolint:staticcheck // legacy config
				} else {
					cfg.ClusterConfig.ClusterCA = &x509.PEMEncodedCertificateAndKey{ //nolint:staticcheck // legacy config
						Crt: newCA.Crt,
					}
				}

				return nil
			})
		}

		if !machineType.IsControlPlane() {
			return container.PatchDocument(
				in,
				func(caCfg *k8s.KubeAPIServerCAConfigV1Alpha1) error {
					// delete the accepted CA if it already exists to avoid duplicates
					caCfg.APIAcceptedCAs = slices.DeleteFunc(caCfg.APIAcceptedCAs, func(ca string) bool {
						return bytes.Equal([]byte(ca), newCA.Crt)
					})

					// prepend the new CA to the accepted CAs list to match pre-multidoc ordering
					caCfg.APIAcceptedCAs = slices.Insert(caCfg.APIAcceptedCAs, 0, string(newCA.Crt))

					return nil
				},
			)
		}

		return container.PatchDocument(
			in,
			func(caCfg *k8s.KubeAPIServerCAConfigV1Alpha1) error {
				caCfg.APIIssuingCA = &meta.CertificateAndKey{
					Cert: string(newCA.Crt),
					Key:  string(newCA.Key),
				}

				return nil
			},
		)
	}
}
