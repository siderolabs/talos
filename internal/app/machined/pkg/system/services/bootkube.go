/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"net"
	"net/url"
	"os"

	"github.com/kubernetes-incubator/bootkube/pkg/asset"
	"github.com/kubernetes-incubator/bootkube/pkg/tlsutil"
	"github.com/pkg/errors"

	"github.com/talos-systems/talos/internal/app/machined/internal/bootkube"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/goroutine"
	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/constants"
)

// Bootkube implements the Service interface. It serves as the concrete type with
// the required methods.
type Bootkube struct{}

// ID implements the Service interface.
func (b *Bootkube) ID(config config.Configurator) string {
	return "bootkube"
}

// PreFunc implements the Service interface.
func (b *Bootkube) PreFunc(ctx context.Context, config config.Configurator) (err error) {
	return generateAssets(config)
}

// PostFunc implements the Service interface.
func (b *Bootkube) PostFunc(config config.Configurator) error {
	return nil
}

// DependsOn implements the Service interface.
func (b *Bootkube) DependsOn(config config.Configurator) []string {
	deps := []string{"etcd"}

	return deps
}

// Condition implements the Service interface.
func (b *Bootkube) Condition(config config.Configurator) conditions.Condition {
	return nil
}

// Runner implements the Service interface.
func (b *Bootkube) Runner(config config.Configurator) (runner.Runner, error) {
	return goroutine.NewRunner(config, "bootkube", bootkube.NewService().Main), nil
}

// nolint: gocyclo
func generateAssets(config config.Configurator) (err error) {
	if err = os.MkdirAll("/etc/kubernetes/manifests", 0644); err != nil {
		return err
	}

	peerCrt, err := ioutil.ReadFile(constants.KubernetesEtcdPeerCert)
	if err != nil {
		return err
	}
	block, _ := pem.Decode(peerCrt)
	if block == nil {
		return errors.New("failed to decode peer certificate")
	}
	peer, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return errors.Wrap(err, "failed to parse client certificate")
	}

	caCrt, err := ioutil.ReadFile(constants.KubernetesEtcdCACert)
	if err != nil {
		return err
	}
	block, _ = pem.Decode(caCrt)
	if block == nil {
		return errors.New("failed to decode CA certificate")
	}
	ca, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return errors.Wrap(err, "failed to parse etcd CA certificate")
	}

	peerKey, err := ioutil.ReadFile(constants.KubernetesEtcdPeerKey)
	if err != nil {
		return err
	}
	block, _ = pem.Decode(peerKey)
	if block == nil {
		return errors.New("failed to peer key")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return errors.Wrap(err, "failed to parse client key")
	}

	etcdServer, err := url.Parse("https://127.0.0.1:2379")
	if err != nil {
		return err
	}

	apiServers := []*url.URL{}
	for _, ip := range config.Cluster().IPs() {
		var u *url.URL
		if u, err = url.Parse("https://" + ip + ":6443"); err != nil {
			return err
		}
		apiServers = append(apiServers, u)
	}
	u, err := url.Parse("https://127.0.0.1:6443")
	if err != nil {
		return err
	}
	apiServers = append(apiServers, u)

	_, podCIDR, err := net.ParseCIDR("10.2.0.0/16")
	if err != nil {
		return err
	}
	_, serviceCIDR, err := net.ParseCIDR("10.3.0.0/24")
	if err != nil {
		return err
	}

	altNames := altNamesFromURLs(apiServers)

	block, _ = pem.Decode(config.Cluster().CA().Crt)
	if block == nil {
		return errors.New("failed to Kubernetes CA certificate")
	}
	k8sCA, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return errors.Wrap(err, "failed to parse Kubernetes CA certificate")
	}

	block, _ = pem.Decode(config.Cluster().CA().Key)
	if block == nil {
		return errors.New("failed to Kubernetes CA key")
	}
	k8sKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return errors.Wrap(err, "failed to parse Kubernetes key")
	}

	conf := asset.Config{
		CACert:                 k8sCA,
		CAPrivKey:              k8sKey,
		EtcdCACert:             ca,
		EtcdClientCert:         peer,
		EtcdClientKey:          key,
		EtcdServers:            []*url.URL{etcdServer},
		EtcdUseTLS:             true,
		APIServers:             apiServers,
		APIServiceIP:           net.ParseIP("10.3.0.1"),
		DNSServiceIP:           net.ParseIP("10.3.0.10"),
		PodCIDR:                podCIDR,
		ServiceCIDR:            serviceCIDR,
		NetworkProvider:        "flannel",
		AltNames:               altNames,
		Images:                 asset.DefaultImages,
		BootstrapSecretsSubdir: "/assets/tls",
		BootstrapTokenID:       config.Cluster().Token().ID(),
		BootstrapTokenSecret:   config.Cluster().Token().Secret(),
	}

	as, err := asset.NewDefaultAssets(conf)
	if err != nil {
		return errors.Wrap(err, "failed to create list of assets")
	}

	if err = as.WriteFiles(constants.AssetsDirectory); err != nil {
		return err
	}

	input, err := ioutil.ReadFile(constants.GeneratedKubeconfigAsset)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(constants.AdminKubeconfig, input, 0600)
}

func altNamesFromURLs(urls []*url.URL) *tlsutil.AltNames {
	var an tlsutil.AltNames
	for _, u := range urls {
		host, _, err := net.SplitHostPort(u.Host)
		if err != nil {
			host = u.Host
		}
		ip := net.ParseIP(host)
		if ip == nil {
			an.DNSNames = append(an.DNSNames, host)
		} else {
			an.IPs = append(an.IPs, ip)
		}
	}
	return &an
}
