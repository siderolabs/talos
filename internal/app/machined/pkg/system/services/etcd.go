/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"context"
	stdlibx509 "crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	stdlibnet "net"
	"os"
	"strings"
	"time"

	containerdapi "github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/pkg/transport"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"github.com/talos-systems/talos/pkg/kubernetes"
	"github.com/talos-systems/talos/pkg/net"
	"github.com/talos-systems/talos/pkg/retry"
)

var etcdImage = fmt.Sprintf("%s:%s", constants.EtcdImage, constants.DefaultEtcdVersion)

// Etcd implements the Service interface. It serves as the concrete type with
// the required methods.
type Etcd struct{}

// ID implements the Service interface.
func (e *Etcd) ID(config runtime.Configurator) string {
	return "etcd"
}

// PreFunc implements the Service interface.
func (e *Etcd) PreFunc(ctx context.Context, config runtime.Configurator) (err error) {
	if err = os.MkdirAll(constants.EtcdDataPath, 0755); err != nil {
		return err
	}

	if err = generatePKI(config); err != nil {
		return errors.Wrap(err, "failed to generate etcd PKI")
	}

	client, err := containerdapi.New(constants.ContainerdAddress)
	if err != nil {
		return err
	}
	// nolint: errcheck
	defer client.Close()

	// Pull the image and unpack it.
	containerdctx := namespaces.WithNamespace(ctx, constants.SystemContainerdNamespace)
	if _, err = client.Pull(containerdctx, etcdImage, containerdapi.WithPullUnpack); err != nil {
		return fmt.Errorf("failed to pull image %q: %v", etcdImage, err)
	}

	return nil
}

// PostFunc implements the Service interface.
func (e *Etcd) PostFunc(config runtime.Configurator) (err error) {
	return nil
}

// Condition implements the Service interface.
func (e *Etcd) Condition(config runtime.Configurator) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (e *Etcd) DependsOn(config runtime.Configurator) []string {
	return []string{"containerd"}
}

// Runner implements the Service interface.
func (e *Etcd) Runner(config runtime.Configurator) (runner.Runner, error) {
	ips, err := net.IPAddrs()
	if err != nil {
		return nil, errors.Wrap(err, "failed to discover IP addresses")
	}

	if len(ips) == 0 {
		return nil, errors.New("failed to discover local IP")
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	initialClusterState := "new"
	initialCluster := hostname + "=https://" + ips[0].String() + ":2380"

	if config.Machine().Type() == machine.ControlPlane {
		initialClusterState = "existing"

		initialCluster, err = buildInitialCluster(config, hostname, ips[0].String())
		if err != nil {
			return nil, err
		}
	}

	// Set the process arguments.
	args := runner.Args{
		ID: e.ID(config),
		ProcessArgs: []string{
			"/usr/local/bin/etcd",
			"--name=" + hostname,
			"--listen-peer-urls=https://0.0.0.0:2380",
			"--listen-client-urls=https://0.0.0.0:2379",
			"--initial-advertise-peer-urls=https://" + ips[0].String() + ":2380",
			"--advertise-client-urls=https://" + ips[0].String() + ":2379",
			"--cert-file=" + constants.KubernetesEtcdPeerCert,
			"--key-file=" + constants.KubernetesEtcdPeerKey,
			"--trusted-ca-file=" + constants.KubernetesEtcdCACert,
			"--peer-client-cert-auth=true",
			"--peer-cert-file=" + constants.KubernetesEtcdPeerCert,
			"--peer-trusted-ca-file=" + constants.KubernetesEtcdCACert,
			"--peer-key-file=" + constants.KubernetesEtcdPeerKey,
			"--initial-cluster=" + initialCluster,
			"--initial-cluster-state=" + initialClusterState,
		},
	}

	mounts := []specs.Mount{
		{Type: "bind", Destination: constants.EtcdPKIPath, Source: constants.EtcdPKIPath, Options: []string{"rbind", "rw"}},
		{Type: "bind", Destination: constants.EtcdDataPath, Source: constants.EtcdDataPath, Options: []string{"rbind", "rw"}},
	}

	env := []string{}
	for key, val := range config.Machine().Env() {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	return restart.New(containerd.NewRunner(
		config.Debug(),
		&args,
		runner.WithNamespace(constants.SystemContainerdNamespace),
		runner.WithContainerImage(etcdImage),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			oci.WithMounts(mounts),
			oci.WithHostNamespace(specs.PIDNamespace),
			oci.WithParentCgroupDevices,
			oci.WithPrivileged,
		),
	),
		restart.WithType(restart.Forever),
	), nil
}

// nolint: gocyclo
func generatePKI(config runtime.Configurator) (err error) {
	if err = os.MkdirAll(constants.EtcdPKIPath, 0644); err != nil {
		return err
	}

	if err = ioutil.WriteFile(constants.KubernetesEtcdCACert, config.Cluster().Etcd().CA().Crt, 0500); err != nil {
		return errors.Wrap(err, "failed to write CA certificate")
	}

	if err = ioutil.WriteFile(constants.KubernetesEtcdCAKey, config.Cluster().Etcd().CA().Key, 0500); err != nil {
		return errors.Wrap(err, "failed to write CA key")
	}

	ips, err := net.IPAddrs()
	if err != nil {
		return errors.Wrap(err, "failed to discover IP addresses")
	}

	ips = append(ips, stdlibnet.ParseIP("127.0.0.1"))

	hostname, err := os.Hostname()
	if err != nil {
		return errors.Wrap(err, "failed to get hostname")
	}

	opts := []x509.Option{
		x509.CommonName(hostname),
		x509.DNSNames([]string{"localhost", hostname}),
		x509.RSA(true),
		x509.IPAddresses(ips),
		x509.NotAfter(time.Now().Add(87600 * time.Hour)),
	}

	peerKey, err := x509.NewRSAKey()
	if err != nil {
		return errors.Wrap(err, "failled to create RSA key")
	}

	pemBlock, _ := pem.Decode(peerKey.KeyPEM)
	if pemBlock == nil {
		return errors.New("failed to decode peer key pem")
	}

	peerKeyRSA, err := stdlibx509.ParsePKCS1PrivateKey(pemBlock.Bytes)
	if err != nil {
		return errors.Wrap(err, "failled to parse private key")
	}

	csr, err := x509.NewCertificateSigningRequest(peerKeyRSA, opts...)
	if err != nil {
		return errors.Wrap(err, "failed to create CSR")
	}

	csrPemBlock, _ := pem.Decode(csr.X509CertificateRequestPEM)
	if csrPemBlock == nil {
		return errors.New("failed to decode csr pem")
	}

	ccsr, err := stdlibx509.ParseCertificateRequest(csrPemBlock.Bytes)
	if err != nil {
		return errors.Wrap(err, "failled to parse certificate request")
	}

	caPemBlock, _ := pem.Decode(config.Cluster().Etcd().CA().Crt)
	if caPemBlock == nil {
		return errors.New("failed to decode ca cert pem")
	}

	caCrt, err := stdlibx509.ParseCertificate(caPemBlock.Bytes)
	if err != nil {
		return errors.Wrap(err, "failed to parse CA")
	}

	caKeyPemBlock, _ := pem.Decode(config.Cluster().Etcd().CA().Key)
	if caKeyPemBlock == nil {
		return errors.New("failed to decode ca key pem")
	}

	caKey, err := stdlibx509.ParsePKCS1PrivateKey(caKeyPemBlock.Bytes)
	if err != nil {
		return errors.Wrap(err, "failed to parse CA private key")
	}

	peer, err := x509.NewCertificateFromCSR(caCrt, caKey, ccsr, opts...)
	if err != nil {
		return errors.Wrap(err, "failled to create peer certificate")
	}

	if err := ioutil.WriteFile(constants.KubernetesEtcdPeerKey, peerKey.KeyPEM, 0500); err != nil {
		return err
	}

	if err := ioutil.WriteFile(constants.KubernetesEtcdPeerCert, peer.X509CertificatePEM, 0500); err != nil {
		return err
	}

	return nil
}

func addMember(endpoints, addrs []string) (*clientv3.MemberAddResponse, error) {
	tlsInfo := transport.TLSInfo{
		CertFile:      constants.KubernetesEtcdPeerCert,
		KeyFile:       constants.KubernetesEtcdPeerKey,
		TrustedCAFile: constants.KubernetesEtcdCACert,
	}

	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		return nil, err
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
		TLS:         tlsConfig,
	})
	if err != nil {
		return nil, err
	}
	// nolint: errcheck
	defer cli.Close()

	resp, err := cli.MemberAdd(context.Background(), addrs)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func buildInitialCluster(config runtime.Configurator, name, ip string) (initial string, err error) {
	endpoint := stdlibnet.ParseIP(config.Cluster().Endpoint())

	h, err := kubernetes.NewTemporaryClientFromPKI(config.Cluster().CA().Crt, config.Cluster().CA().Key, endpoint.String(), "6443")
	if err != nil {
		return "", err
	}

	opts := []retry.Option{retry.WithUnits(3 * time.Second), retry.WithJitter(time.Second)}
	err = retry.Constant(10*time.Minute, opts...).Retry(func() error {
		var endpoints []string
		endpoints, err = h.MasterIPs()
		if err != nil {
			return retry.ExpectedError(err)
		}

		// Etcd expects host:port format.
		for i := 0; i < len(endpoints); i++ {
			endpoints[i] += ":2379"
		}

		peerAddrs := []string{"https://" + ip + ":2380"}

		var resp *clientv3.MemberAddResponse
		resp, err = addMember(endpoints, peerAddrs)
		if err != nil {
			return retry.ExpectedError(err)
		}

		newID := resp.Member.ID
		conf := []string{}

		for _, memb := range resp.Members {
			for _, u := range memb.PeerURLs {
				n := memb.Name
				if memb.ID == newID {
					n = name
				}

				conf = append(conf, fmt.Sprintf("%s=%s", n, u))
			}
		}

		initial = strings.Join(conf, ",")

		return nil
	})

	if err != nil {
		return "", errors.New("failed to discover etcd cluster")
	}

	return initial, nil
}
