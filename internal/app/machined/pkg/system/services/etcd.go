// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	stdlibx509 "crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	stdlibnet "net"
	"os"
	"strings"
	"time"

	containerdapi "github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"go.etcd.io/etcd/clientv3"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/internal/pkg/conditions"
	"github.com/talos-systems/talos/internal/pkg/containers/image"
	"github.com/talos-systems/talos/internal/pkg/etcd"
	"github.com/talos-systems/talos/internal/pkg/metadata"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/argsbuilder"
	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"github.com/talos-systems/talos/pkg/net"
	"github.com/talos-systems/talos/pkg/retry"
)

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
		return fmt.Errorf("failed to generate etcd PKI: %w", err)
	}

	client, err := containerdapi.New(constants.ContainerdAddress)
	if err != nil {
		return err
	}
	// nolint: errcheck
	defer client.Close()

	// Pull the image and unpack it.

	containerdctx := namespaces.WithNamespace(ctx, constants.SystemContainerdNamespace)
	if _, err = image.Pull(containerdctx, config.Machine().Registries(), client, config.Cluster().Etcd().Image()); err != nil {
		return fmt.Errorf("failed to pull image %q: %w", config.Cluster().Etcd().Image(), err)
	}

	return nil
}

// PostFunc implements the Service interface.
func (e *Etcd) PostFunc(config runtime.Configurator, state events.ServiceState) (err error) {
	return nil
}

// Condition implements the Service interface.
func (e *Etcd) Condition(config runtime.Configurator) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (e *Etcd) DependsOn(config runtime.Configurator) []string {
	return []string{"containerd", "networkd"}
}

// Runner implements the Service interface.
func (e *Etcd) Runner(config runtime.Configurator) (runner.Runner, error) {
	a, err := e.args(config)
	if err != nil {
		return nil, err
	}

	// Set the process arguments.
	args := runner.Args{
		ID:          e.ID(config),
		ProcessArgs: append([]string{"/usr/local/bin/etcd"}, a...),
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
		runner.WithContainerImage(config.Cluster().Etcd().Image()),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			oci.WithHostNamespace(specs.NetworkNamespace),
			oci.WithMounts(mounts),
		),
	),
		restart.WithType(restart.Forever),
	), nil
}

// HealthFunc implements the HealthcheckedService interface
func (e *Etcd) HealthFunc(runtime.Configurator) health.Check {
	return func(ctx context.Context) error {
		client, err := etcd.NewClient([]string{"127.0.0.1:2379"})
		if err != nil {
			return err
		}

		return client.Close()
	}
}

// HealthSettings implements the HealthcheckedService interface
func (e *Etcd) HealthSettings(runtime.Configurator) *health.Settings {
	return &health.DefaultSettings
}

// nolint: gocyclo
func generatePKI(config runtime.Configurator) (err error) {
	if err = os.MkdirAll(constants.EtcdPKIPath, 0644); err != nil {
		return err
	}

	if err = ioutil.WriteFile(constants.KubernetesEtcdCACert, config.Cluster().Etcd().CA().Crt, 0500); err != nil {
		return fmt.Errorf("failed to write CA certificate: %w", err)
	}

	if err = ioutil.WriteFile(constants.KubernetesEtcdCAKey, config.Cluster().Etcd().CA().Key, 0500); err != nil {
		return fmt.Errorf("failed to write CA key: %w", err)
	}

	ips, err := net.IPAddrs()
	if err != nil {
		return fmt.Errorf("failed to discover IP addresses: %w", err)
	}

	ips = append(ips, stdlibnet.ParseIP("127.0.0.1"))
	if net.IsIPv6(ips...) {
		ips = append(ips, stdlibnet.ParseIP("::1"))
	}

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}

	dnsNames, err := net.DNSNames()
	if err != nil {
		return fmt.Errorf("failed to get host DNS names: %w", err)
	}

	dnsNames = append(dnsNames, "localhost")

	opts := []x509.Option{
		x509.CommonName(hostname),
		x509.DNSNames(dnsNames),
		x509.RSA(true),
		x509.IPAddresses(ips),
		x509.NotAfter(time.Now().Add(87600 * time.Hour)),
	}

	peerKey, err := x509.NewRSAKey()
	if err != nil {
		return fmt.Errorf("failled to create RSA key: %w", err)
	}

	pemBlock, _ := pem.Decode(peerKey.KeyPEM)
	if pemBlock == nil {
		return errors.New("failed to decode peer key pem")
	}

	peerKeyRSA, err := stdlibx509.ParsePKCS1PrivateKey(pemBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failled to parse private key: %w", err)
	}

	csr, err := x509.NewCertificateSigningRequest(peerKeyRSA, opts...)
	if err != nil {
		return fmt.Errorf("failed to create CSR: %w", err)
	}

	csrPemBlock, _ := pem.Decode(csr.X509CertificateRequestPEM)
	if csrPemBlock == nil {
		return errors.New("failed to decode csr pem")
	}

	ccsr, err := stdlibx509.ParseCertificateRequest(csrPemBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failled to parse certificate request: %w", err)
	}

	caPemBlock, _ := pem.Decode(config.Cluster().Etcd().CA().Crt)
	if caPemBlock == nil {
		return errors.New("failed to decode ca cert pem")
	}

	caCrt, err := stdlibx509.ParseCertificate(caPemBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA: %w", err)
	}

	caKeyPemBlock, _ := pem.Decode(config.Cluster().Etcd().CA().Key)
	if caKeyPemBlock == nil {
		return errors.New("failed to decode ca key pem")
	}

	caKey, err := stdlibx509.ParsePKCS1PrivateKey(caKeyPemBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA private key: %w", err)
	}

	peer, err := x509.NewCertificateFromCSR(caCrt, caKey, ccsr, opts...)
	if err != nil {
		return fmt.Errorf("failled to create peer certificate: %w", err)
	}

	if err := ioutil.WriteFile(constants.KubernetesEtcdPeerKey, peerKey.KeyPEM, 0500); err != nil {
		return err
	}

	if err := ioutil.WriteFile(constants.KubernetesEtcdPeerCert, peer.X509CertificatePEM, 0500); err != nil {
		return err
	}

	return nil
}

func addMember(config runtime.Configurator, addrs []string, name string) (*clientv3.MemberListResponse, uint64, error) {
	client, err := etcd.NewClientFromControlPlaneIPs(config.Cluster().CA(), config.Cluster().Endpoint())
	if err != nil {
		return nil, 0, err
	}

	// nolint: errcheck
	defer client.Close()

	list, err := client.MemberList(context.Background())
	if err != nil {
		return nil, 0, err
	}

	for _, member := range list.Members {
		if member.Name == name {
			return list, member.ID, nil
		}
	}

	add, err := client.MemberAdd(context.Background(), addrs)
	if err != nil {
		return nil, 0, err
	}

	list, err = client.MemberList(context.Background())
	if err != nil {
		return nil, 0, err
	}

	return list, add.Member.ID, nil
}

func buildInitialCluster(config runtime.Configurator, name, ip string) (initial string, err error) {
	err = retry.Constant(10*time.Minute, retry.WithUnits(3*time.Second), retry.WithJitter(time.Second)).Retry(func() error {
		var (
			peerAddrs = []string{"https://" + ip + ":2380"}
			resp      *clientv3.MemberListResponse
			id        uint64
		)

		resp, id, err = addMember(config, peerAddrs, name)
		if err != nil {
			// TODO(andrewrynhard): We should check the error type here and
			// handle the specific error accordingly.
			return retry.ExpectedError(err)
		}

		conf := []string{}

		for _, memb := range resp.Members {
			for _, u := range memb.PeerURLs {
				n := memb.Name
				if memb.ID == id {
					n = name
				}

				conf = append(conf, fmt.Sprintf("%s=%s", n, u))
			}
		}

		initial = strings.Join(conf, ",")

		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to build cluster arguments: %w", err)
	}

	return initial, nil
}

// nolint: gocyclo
func (e *Etcd) args(config runtime.Configurator) ([]string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	metadata, err := metadata.Open()
	if err != nil {
		return nil, err
	}

	ips, err := net.IPAddrs()
	if err != nil {
		return nil, fmt.Errorf("failed to discover IP addresses: %w", err)
	}

	if len(ips) == 0 {
		return nil, errors.New("failed to discover local IP")
	}

	listenAddress := "0.0.0.0"
	if net.IsIPv6(ips...) {
		listenAddress = "[::]"
	}

	blackListArgs := argsbuilder.Args{
		"name":                  hostname,
		"data-dir":              constants.EtcdDataPath,
		"listen-peer-urls":      "https://" + listenAddress + ":2380",
		"listen-client-urls":    "https://" + listenAddress + ":2379",
		"cert-file":             constants.KubernetesEtcdPeerCert,
		"key-file":              constants.KubernetesEtcdPeerKey,
		"trusted-ca-file":       constants.KubernetesEtcdCACert,
		"peer-client-cert-auth": "true",
		"peer-cert-file":        constants.KubernetesEtcdPeerCert,
		"peer-trusted-ca-file":  constants.KubernetesEtcdCACert,
		"peer-key-file":         constants.KubernetesEtcdPeerKey,
	}

	extraArgs := argsbuilder.Args(config.Cluster().Etcd().ExtraArgs())

	for k := range blackListArgs {
		if extraArgs.Contains(k) {
			return nil, argsbuilder.NewBlacklistError(k)
		}
	}

	if !extraArgs.Contains("initial-cluster-state") {
		blackListArgs.Set("initial-cluster-state", "new")
	}

	// If the initial cluster isn't explicitly defined, we need to discover any
	// existing members.
	if !extraArgs.Contains("initial-cluster") {
		ok, err := IsDirEmpty(constants.EtcdDataPath)
		if err != nil {
			return nil, err
		}

		if ok {
			initialCluster := fmt.Sprintf("%s=https://%s:2380", hostname, ips[0].String())

			existing := config.Machine().Type() == machine.TypeControlPlane || metadata.Upgraded
			if existing {
				blackListArgs.Set("initial-cluster-state", "existing")

				initialCluster, err = buildInitialCluster(config, hostname, ips[0].String())
				if err != nil {
					return nil, err
				}
			}

			blackListArgs.Set("initial-cluster", initialCluster)
		} else {
			blackListArgs.Set("initial-cluster-state", "existing")
		}
	}

	if !extraArgs.Contains("initial-advertise-peer-urls") {
		blackListArgs.Set("initial-advertise-peer-urls", fmt.Sprintf("https://%s:2380", ips[0].String()))
	}

	if !extraArgs.Contains("advertise-client-urls") {
		blackListArgs.Set("advertise-client-urls", fmt.Sprintf("https://%s:2379", ips[0].String()))
	}

	return blackListArgs.Merge(extraArgs).Args(), nil
}

// IsDirEmpty checks if a directory is empty or not.
func IsDirEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	// nolint: errcheck
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}

	return false, err
}
