// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package secrets provides types and methods to handle base machine configuration secrets.
package secrets

import (
	"time"

	"github.com/siderolabs/crypto/x509"
)

// CAValidityTime is the default validity time for CA certificates.
const CAValidityTime = 87600 * time.Hour

// Bundle contains all cluster secrets required to generate machine configuration.
//
// NB: this structure is marhsalled/unmarshalled to/from JSON in various projects, so
// we need to keep representation compatible.
type Bundle struct {
	Clock      Clock       `yaml:"-" json:"-"`
	Cluster    *Cluster    `json:"Cluster"`
	Secrets    *Secrets    `json:"Secrets"`
	TrustdInfo *TrustdInfo `json:"TrustdInfo"`
	Certs      *Certs      `json:"Certs"`
}

// Certs holds the base64 encoded keys and certificates.
type Certs struct {
	// Admin is Talos admin talosconfig client certificate and key.
	//
	// Deprecated: should not be used anymore.
	Admin *x509.PEMEncodedCertificateAndKey `json:"Admin,omitempty" yaml:",omitempty"`
	// Etcd is etcd CA certificate and key.
	Etcd *x509.PEMEncodedCertificateAndKey `json:"Etcd"`
	// K8s is Kubernetes CA certificate and key.
	K8s *x509.PEMEncodedCertificateAndKey `json:"K8s"`
	// K8sAggregator is Kubernetes aggregator CA certificate and key.
	K8sAggregator *x509.PEMEncodedCertificateAndKey `json:"K8sAggregator"`
	// K8sServiceAccount is Kubernetes service account key.
	K8sServiceAccount *x509.PEMEncodedKey `json:"K8sServiceAccount"`
	// OS is Talos API CA certificate and key.
	OS *x509.PEMEncodedCertificateAndKey `json:"OS"`
}

// Cluster holds Talos cluster-wide secrets.
type Cluster struct {
	ID     string `json:"Id"`
	Secret string `json:"Secret"`
}

// Secrets holds the sensitive kubeadm data.
type Secrets struct {
	BootstrapToken            string `json:"BootstrapToken"`
	AESCBCEncryptionSecret    string `json:"AESCBCEncryptionSecret,omitempty" yaml:",omitempty"`
	SecretboxEncryptionSecret string `json:"SecretboxEncryptionSecret,omitempty" yaml:",omitempty"`
}

// TrustdInfo holds the trustd credentials.
type TrustdInfo struct {
	Token string `json:"Token"`
}
