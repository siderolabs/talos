// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"

	getter "github.com/hashicorp/go-getter"
	"github.com/hashicorp/go-multierror"
	"github.com/kubernetes-sigs/bootkube/pkg/asset"
	"github.com/kubernetes-sigs/bootkube/pkg/tlsutil"
	"go.etcd.io/etcd/clientv3"

	"github.com/talos-systems/talos/internal/app/machined/internal/bootkube"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/goroutine"
	"github.com/talos-systems/talos/internal/pkg/conditions"
	"github.com/talos-systems/talos/internal/pkg/etcd"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
	tnet "github.com/talos-systems/talos/pkg/net"
	"github.com/talos-systems/talos/pkg/retry"
)

// DefaultPodSecurityPolicy is the default PSP.
var DefaultPodSecurityPolicy = []byte(`---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: psp:privileged
rules:
- apiGroups: ['policy']
  resources: ['podsecuritypolicies']
  verbs:     ['use']
  resourceNames:
  - privileged
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: psp:privileged
roleRef:
  kind: ClusterRole
  name: psp:privileged
  apiGroup: rbac.authorization.k8s.io
subjects:
# Authorize all service accounts in a namespace:
- kind: Group
  apiGroup: rbac.authorization.k8s.io
  name: system:serviceaccounts
# Authorize all authenticated users in a namespace:
- kind: Group
  apiGroup: rbac.authorization.k8s.io
  name: system:authenticated
---
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: privileged
spec:
  fsGroup:
    rule: RunAsAny
  privileged: true
  runAsUser:
    rule: RunAsAny
  seLinux:
    rule: RunAsAny
  supplementalGroups:
    rule: RunAsAny
  volumes:
  - '*'
  allowedCapabilities:
  - '*'
  hostPID: true
  hostIPC: true
  hostNetwork: true
  hostPorts:
  - min: 1
    max: 65536
`)

// Bootkube implements the Service interface. It serves as the concrete type with
// the required methods.
type Bootkube struct {
	provisioned bool
}

// ID implements the Service interface.
func (b *Bootkube) ID(config runtime.Configurator) string {
	return "bootkube"
}

// PreFunc implements the Service interface.
func (b *Bootkube) PreFunc(ctx context.Context, config runtime.Configurator) (err error) {
	client, err := etcd.NewClient([]string{"127.0.0.1:2379"})
	if err != nil {
		return err
	}

	// nolint: errcheck
	defer client.Close()

	// nolint: errcheck
	retry.Exponential(15*time.Second, retry.WithUnits(50*time.Millisecond), retry.WithJitter(25*time.Millisecond)).Retry(func() error {
		var resp *clientv3.GetResponse
		var err error

		ctx := clientv3.WithRequireLeader(context.Background())
		if resp, err = client.Get(ctx, constants.InitializedKey); err != nil {
			return retry.ExpectedError(err)
		}

		if len(resp.Kvs) == 0 {
			return retry.ExpectedError(errors.New("no value found"))
		}

		if len(resp.Kvs) > 0 {
			if string(resp.Kvs[0].Value) == "true" {
				b.provisioned = true
			}
		}

		return nil
	})

	if b.provisioned {
		return nil
	}

	return generateAssets(config)
}

// PostFunc implements the Service interface.
func (b *Bootkube) PostFunc(config runtime.Configurator, state events.ServiceState) (err error) {
	if state != events.StateFinished {
		log.Println("bootkube run did not complete successfully. skipping etcd update")
		return nil
	}

	client, err := etcd.NewClient([]string{"127.0.0.1:2379"})
	if err != nil {
		return err
	}

	// nolint: errcheck
	defer client.Close()

	err = retry.Exponential(15*time.Second, retry.WithUnits(50*time.Millisecond), retry.WithJitter(25*time.Millisecond)).Retry(func() error {
		ctx := clientv3.WithRequireLeader(context.Background())
		if _, err = client.Put(ctx, constants.InitializedKey, "true"); err != nil {
			return retry.ExpectedError(err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to put state into etcd: %w", err)
	}

	log.Println("updated initialization status in etcd")

	return nil
}

// DependsOn implements the Service interface.
func (b *Bootkube) DependsOn(config runtime.Configurator) []string {
	deps := []string{"etcd"}

	return deps
}

// Condition implements the Service interface.
func (b *Bootkube) Condition(config runtime.Configurator) conditions.Condition {
	return nil
}

// Runner implements the Service interface.
func (b *Bootkube) Runner(config runtime.Configurator) (runner.Runner, error) {
	if b.provisioned {
		return nil, nil
	}

	return goroutine.NewRunner(config, "bootkube", bootkube.NewService().Main), nil
}

// nolint: gocyclo
func generateAssets(config runtime.Configurator) (err error) {
	if err = os.MkdirAll("/etc/kubernetes/manifests", 0644); err != nil {
		return err
	}

	// Ensure assets directory does not exist / is left over from a failed install
	if err = os.RemoveAll(constants.AssetsDirectory); err != nil {
		// Ignore if the directory does not exist
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
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
		return fmt.Errorf("failed to parse client certificate: %w", err)
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
		return fmt.Errorf("failed to parse etcd CA certificate: %w", err)
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
		return fmt.Errorf("failed to parse client key: %w", err)
	}

	etcdServer, err := url.Parse("https://127.0.0.1:2379")
	if err != nil {
		return err
	}

	_, podCIDR, err := net.ParseCIDR(config.Cluster().Network().PodCIDR())
	if err != nil {
		return err
	}

	_, serviceCIDR, err := net.ParseCIDR(config.Cluster().Network().ServiceCIDR())
	if err != nil {
		return err
	}

	urls := []string{config.Cluster().Endpoint().Hostname()}
	urls = append(urls, config.Cluster().CertSANs()...)
	altNames := altNamesFromURLs(urls)

	k8sCA, err := config.Cluster().CA().GetCert()
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes CA certificate: %w", err)
	}

	k8sKey, err := config.Cluster().CA().GetRSAKey()
	if err != nil {
		return fmt.Errorf("failed to parse Kubernetes key: %w", err)
	}

	apiServiceIP, err := tnet.NthIPInNetwork(serviceCIDR, 1)
	if err != nil {
		return err
	}

	dnsServiceIP, err := tnet.NthIPInNetwork(serviceCIDR, 10)
	if err != nil {
		return err
	}

	images := asset.DefaultImages

	images.Hyperkube = config.Machine().Kubelet().Image()

	// Allow for overriding by users via config data
	images.CoreDNS = config.Cluster().CoreDNS().Image()
	images.PodCheckpointer = config.Cluster().PodCheckpointer().Image()

	conf := asset.Config{
		ClusterName:                config.Cluster().Name(),
		APIServerExtraArgs:         config.Cluster().APIServer().ExtraArgs(),
		ControllerManagerExtraArgs: config.Cluster().ControllerManager().ExtraArgs(),
		SchedulerExtraArgs:         config.Cluster().Scheduler().ExtraArgs(),
		CACert:                     k8sCA,
		CAPrivKey:                  k8sKey,
		EtcdCACert:                 ca,
		EtcdClientCert:             peer,
		EtcdClientKey:              key,
		EtcdServers:                []*url.URL{etcdServer},
		EtcdUseTLS:                 true,
		ControlPlaneEndpoint:       config.Cluster().Endpoint(),
		LocalAPIServerPort:         config.Cluster().LocalAPIServerPort(),
		APIServiceIP:               apiServiceIP,
		DNSServiceIP:               dnsServiceIP,
		PodCIDR:                    podCIDR,
		ServiceCIDR:                serviceCIDR,
		NetworkProvider:            config.Cluster().Network().CNI().Name(),
		AltNames:                   altNames,
		Images:                     images,
		BootstrapSecretsSubdir:     "/assets/tls",
		BootstrapTokenID:           config.Cluster().Token().ID(),
		BootstrapTokenSecret:       config.Cluster().Token().Secret(),
		AESCBCEncryptionSecret:     config.Cluster().AESCBCEncryptionSecret(),
	}

	as, err := asset.NewDefaultAssets(conf)
	if err != nil {
		return fmt.Errorf("failed to create list of assets: %w", err)
	}

	if err = as.WriteFiles(constants.AssetsDirectory); err != nil {
		return err
	}

	if err = ioutil.WriteFile(filepath.Join(constants.AssetsDirectory, "manifests", "psp.yaml"), DefaultPodSecurityPolicy, 0600); err != nil {
		return err
	}

	// If "custom" is the CNI, we expect the user to supply one or more urls that point to CNI yamls
	if config.Cluster().Network().CNI().Name() == constants.CustomCNI {
		if err = fetchManifests(config.Cluster().Network().CNI().URLs()); err != nil {
			return err
		}
	}

	if len(config.Cluster().ExtraManifestURLs()) > 0 {
		if err = fetchManifests(config.Cluster().ExtraManifestURLs()); err != nil {
			return err
		}
	}

	return nil
}

func altNamesFromURLs(urls []string) *tlsutil.AltNames {
	var an tlsutil.AltNames

	for _, u := range urls {
		ip := net.ParseIP(u)
		if ip != nil {
			an.IPs = append(an.IPs, ip)
			continue
		}

		an.DNSNames = append(an.DNSNames, u)
	}

	return &an
}

// fetchManifests will lay down manifests in the provided urls to the bootkube assets directory
func fetchManifests(urls []string) error {
	ctx := context.Background()

	var result *multierror.Error

	for _, url := range urls {
		fileName := path.Base(url)

		pwd, err := os.Getwd()
		if err != nil {
			result = multierror.Append(result, err)
			continue
		}

		// Disable netrc since we don't have getent installed, and most likely
		// never will.
		httpGetter := &getter.HttpGetter{
			Netrc:  false,
			Client: http.DefaultClient,
		}

		getter.Getters["http"] = httpGetter
		getter.Getters["https"] = httpGetter

		// We will squirrel all user-supplied manifests into a `zzz-talos` directory.
		// Bootkube applies manifests alphabetically, so pushing these into a subdir with this name
		// allows us to ensure they're the last things that get applied and things like PSPs and whatnot are present
		client := &getter.Client{
			Ctx:     ctx,
			Src:     url,
			Dst:     filepath.Join(constants.AssetsDirectory, "manifests", "zzz-talos", fileName),
			Pwd:     pwd,
			Mode:    getter.ClientModeFile,
			Options: []getter.ClientOption{},
		}

		if err = client.Get(); err != nil {
			result = multierror.Append(result, err)
			continue
		}
	}

	return result.ErrorOrNil()
}
