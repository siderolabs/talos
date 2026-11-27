// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rotatepatcher

import (
	"bytes"
	"errors"
	"slices"

	"github.com/siderolabs/crypto/x509"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/etcd"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
)

// ErrEtcdLegacyConfig is returned when the machine configuration doesn't contain the EtcdCAConfig document.
//
// The etcd CA rotation is only supported with the multi-document etcd configuration, as the legacy
// configuration (.cluster.etcd) has no way to specify the accepted CAs.
var ErrEtcdLegacyConfig = errors.New("etcd CA rotation requires the " + etcd.EtcdCAConfig + " machine configuration document")

func equalCA(ca string, caCrt []byte) bool {
	return bytes.Equal(bytes.TrimSpace([]byte(ca)), bytes.TrimSpace(caCrt))
}

func patchEtcdCA(in config.Provider, patcher func(*etcd.EtcdCAConfigV1Alpha1) error) (config.Provider, error) {
	if !in.Has(etcd.EtcdCAConfig) {
		return nil, ErrEtcdLegacyConfig
	}

	return container.PatchDocument(in, patcher)
}

// EtcdAddAcceptedCA adds the given CA certificate to the list of accepted etcd CAs.
//
// The patch is a no-op if the CA is already accepted (including the case when it is the issuing CA).
func EtcdAddAcceptedCA(caCrt []byte) PatcherFunc {
	return func(in config.Provider) (config.Provider, error) {
		return patchEtcdCA(in, func(caCfg *etcd.EtcdCAConfigV1Alpha1) error {
			if caCfg.CAIssuingCA != nil && equalCA(caCfg.CAIssuingCA.Cert, caCrt) {
				return nil
			}

			if slices.ContainsFunc(caCfg.CAAcceptedCAs, func(ca string) bool { return equalCA(ca, caCrt) }) {
				return nil
			}

			caCfg.CAAcceptedCAs = append(caCfg.CAAcceptedCAs, string(caCrt))

			return nil
		})
	}
}

// EtcdDeleteAcceptedCA removes the given CA certificate from the list of accepted etcd CAs.
//
// The issuing CA is always accepted, and it is not affected by this patch.
func EtcdDeleteAcceptedCA(caCrt []byte) PatcherFunc {
	return func(in config.Provider) (config.Provider, error) {
		return patchEtcdCA(in, func(caCfg *etcd.EtcdCAConfigV1Alpha1) error {
			caCfg.CAAcceptedCAs = slices.DeleteFunc(caCfg.CAAcceptedCAs, func(ca string) bool { return equalCA(ca, caCrt) })

			return nil
		})
	}
}

// EtcdSetCA makes the given CA the issuing etcd CA.
//
// The previous issuing CA of the machine is kept in the list of accepted CAs, so that the certificates
// issued by it are still trusted.
//
// The patch is a no-op if the CA is already the issuing CA.
func EtcdSetCA(newCA *x509.PEMEncodedCertificateAndKey) PatcherFunc {
	return func(in config.Provider) (config.Provider, error) {
		return patchEtcdCA(in, func(caCfg *etcd.EtcdCAConfigV1Alpha1) error {
			if caCfg.CAIssuingCA != nil {
				if equalCA(caCfg.CAIssuingCA.Cert, newCA.Crt) {
					return nil
				}

				previousCA := caCfg.CAIssuingCA.Cert

				if !slices.ContainsFunc(caCfg.CAAcceptedCAs, func(ca string) bool { return equalCA(ca, []byte(previousCA)) }) {
					caCfg.CAAcceptedCAs = slices.Insert(caCfg.CAAcceptedCAs, 0, previousCA)
				}
			}

			// the issuing CA is implicitly accepted
			caCfg.CAAcceptedCAs = slices.DeleteFunc(caCfg.CAAcceptedCAs, func(ca string) bool { return equalCA(ca, newCA.Crt) })

			caCfg.CAIssuingCA = &meta.CertificateAndKey{
				Cert: string(newCA.Crt),
				Key:  string(newCA.Key),
			}

			return nil
		})
	}
}
