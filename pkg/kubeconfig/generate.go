// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeconfig

import (
	"bytes"
	stdlibx509 "crypto/x509"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// GenerateAdminInput is the interface for the GenerateAdmin function.
//
// This interface is implemented by config.Cluster().
type GenerateAdminInput interface {
	Name() string
	Endpoint() *url.URL
	IssuingCA() *x509.PEMEncodedCertificateAndKey
	AcceptedCAs() []*x509.PEMEncodedCertificate
	AdminKubeconfig() config.AdminKubeconfig
}

// GenerateAdmin generates admin kubeconfig for the cluster.
func GenerateAdmin(config GenerateAdminInput, out io.Writer) error {
	acceptedCAs := config.AcceptedCAs()

	if config.IssuingCA() != nil {
		acceptedCAs = append(acceptedCAs, &x509.PEMEncodedCertificate{Crt: config.IssuingCA().Crt})
	}

	return Generate(
		&GenerateInput{
			ClusterName:         config.Name(),
			IssuingCA:           config.IssuingCA(),
			AcceptedCAs:         acceptedCAs,
			CertificateLifetime: config.AdminKubeconfig().CertLifetime(),

			CommonName:   config.AdminKubeconfig().CommonName(),
			Organization: config.AdminKubeconfig().CertOrganization(),

			Endpoint:    config.Endpoint().String(),
			Username:    "admin",
			ContextName: "admin",
		},
		out,
	)
}

// GenerateInput are input parameters for Generate.
type GenerateInput struct {
	ClusterName string

	IssuingCA           *x509.PEMEncodedCertificateAndKey
	AcceptedCAs         []*x509.PEMEncodedCertificate
	CertificateLifetime time.Duration

	CommonName   string
	Organization string

	Endpoint    string
	Username    string
	ContextName string
}

const allowedTimeSkew = 10 * time.Second

// Generate a kubeconfig for the cluster from the given Input.
func Generate(in *GenerateInput, out io.Writer) error {
	k8sCA, err := x509.NewCertificateAuthorityFromCertificateAndKey(in.IssuingCA)
	if err != nil {
		return fmt.Errorf("error getting Kubernetes CA: %w", err)
	}

	clientCert, err := x509.NewKeyPair(k8sCA,
		x509.CommonName(in.CommonName),
		x509.Organization(in.Organization),
		x509.NotBefore(time.Now().Add(-allowedTimeSkew)),
		x509.NotAfter(time.Now().Add(in.CertificateLifetime)),
		x509.KeyUsage(stdlibx509.KeyUsageDigitalSignature|stdlibx509.KeyUsageKeyEncipherment),
		x509.ExtKeyUsage([]stdlibx509.ExtKeyUsage{
			stdlibx509.ExtKeyUsageClientAuth,
		}),
	)
	if err != nil {
		return fmt.Errorf("error generating Kubernetes client certificate: %w", err)
	}

	clientCertPEM := x509.NewCertificateAndKeyFromKeyPair(clientCert)

	serverCAs := bytes.Join(xslices.Map(in.AcceptedCAs, func(ca *x509.PEMEncodedCertificate) []byte { return ca.Crt }), nil)

	cfg := clientcmdapi.Config{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters: map[string]*clientcmdapi.Cluster{
			in.ClusterName: {
				Server:                   in.Endpoint,
				CertificateAuthorityData: serverCAs,
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			in.Username + "@" + in.ClusterName: {
				ClientCertificateData: clientCertPEM.Crt,
				ClientKeyData:         clientCertPEM.Key,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			in.ContextName + "@" + in.ClusterName: {
				Cluster:   in.ClusterName,
				Namespace: "default",
				AuthInfo:  in.Username + "@" + in.ClusterName,
			},
		},
		CurrentContext: in.ContextName + "@" + in.ClusterName,
	}

	marshaled, err := clientcmd.Write(cfg)
	if err != nil {
		return err
	}

	_, err = out.Write(marshaled)

	return err
}
