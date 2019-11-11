// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config/cluster"
	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/crypto/x509"
)

const (
	// Version is the version string for v1alpha1.
	Version = "v1alpha1"
)

// Version implements the Configurator interface.
func (c *Config) Version() string {
	return Version
}

// Debug implements the Configurator interface.
func (c *Config) Debug() bool {
	return false
}

// Machine implements the Configurator interface.
func (c *Config) Machine() machine.Machine {
	return c.MachineConfig
}

// Cluster implements the Configurator interface.
func (c *Config) Cluster() cluster.Cluster {
	return c.ClusterConfig
}

// Validate implements the Configurator interface.
func (c *Config) Validate(mode runtime.Mode) error {
	if c.MachineConfig == nil {
		return errors.New("machine instructions are required")
	}

	if c.ClusterConfig == nil {
		return errors.New("cluster instructions are required")
	}

	if c.Cluster().Endpoint() == nil || c.Cluster().Endpoint().String() == "" {
		return errors.New("a cluster endpoint is required")
	}

	if mode == runtime.Metal {
		if c.MachineConfig.MachineInstall == nil {
			return fmt.Errorf("install instructions are required by the %q mode", runtime.Metal.String())
		}
	}

	return nil
}

// String implements the Configurator interface.
func (c *Config) String() (string, error) {
	b, err := yaml.Marshal(c)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// Install implements the Configurator interface.
func (m *MachineConfig) Install() machine.Install {
	if m.MachineInstall == nil {
		return &InstallConfig{}
	}

	return m.MachineInstall
}

// Security implements the Configurator interface.
func (m *MachineConfig) Security() machine.Security {
	return m
}

// Disks implements the Configurator interface.
func (m *MachineConfig) Disks() []machine.Disk {
	return m.MachineDisks
}

// Network implements the Configurator interface.
func (m *MachineConfig) Network() machine.Network {
	if m.MachineNetwork == nil {
		return &NetworkConfig{}
	}

	return m.MachineNetwork
}

// Time implements the Configurator interface.
func (m *MachineConfig) Time() machine.Time {
	if m.MachineTime == nil {
		return &TimeConfig{}
	}

	return m.MachineTime
}

// Kubelet implements the Configurator interface.
func (m *MachineConfig) Kubelet() machine.Kubelet {
	return m
}

// Env implements the Configurator interface.
func (m *MachineConfig) Env() machine.Env {
	return m.MachineEnv
}

// Files implements the Configurator interface.
func (m *MachineConfig) Files() []machine.File {
	return m.MachineFiles
}

// Type implements the Configurator interface.
func (m *MachineConfig) Type() machine.Type {
	switch m.MachineType {
	case "init":
		return machine.Bootstrap
	case "controlplane":
		return machine.ControlPlane
	default:
		return machine.Worker
	}
}

// Server implements the Configurator interface.
func (m *MachineConfig) Server() string {
	return ""
}

// CA implements the Configurator interface.
func (m *MachineConfig) CA() *x509.PEMEncodedCertificateAndKey {
	return m.MachineCA
}

// Token implements the Configurator interface.
func (m *MachineConfig) Token() string {
	return m.MachineToken
}

// CertSANs implements the Configurator interface.
func (m *MachineConfig) CertSANs() []string {
	return m.MachineCertSANs
}

// SetCertSANs implements the Configurator interface.
func (m *MachineConfig) SetCertSANs(sans []string) {
	m.MachineCertSANs = append(m.MachineCertSANs, sans...)
}

// ExtraMounts implements the Configurator interface.
func (m *MachineConfig) ExtraMounts() []specs.Mount {
	return nil
}

// Version implements the Configurator interface.
func (c *ClusterConfig) Version() string {
	return c.ControlPlane.Version
}

// Endpoint implements the Configurator interface.
func (c *ClusterConfig) Endpoint() *url.URL {
	return c.ControlPlane.Endpoint.URL
}

// LocalAPIServerPort implements the Configurator interface.
func (c *ClusterConfig) LocalAPIServerPort() int {
	if c.ControlPlane.LocalAPIServerPort == 0 {
		return 6443
	}

	return c.ControlPlane.LocalAPIServerPort
}

// CertSANs implements the Configurator interface.
func (c *ClusterConfig) CertSANs() []string {
	return c.APIServer.CertSANs
}

// SetCertSANs implements the Configurator interface.
func (c *ClusterConfig) SetCertSANs(sans []string) {
	if c.APIServer == nil {
		c.APIServer = &APIServerConfig{}
	}

	c.APIServer.CertSANs = append(c.APIServer.CertSANs, sans...)
}

// CA implements the Configurator interface.
func (c *ClusterConfig) CA() *x509.PEMEncodedCertificateAndKey {
	return c.ClusterCA
}

// AESCBCEncryptionSecret implements the Configurator interface.
func (c *ClusterConfig) AESCBCEncryptionSecret() string {
	return c.ClusterAESCBCEncryptionSecret
}

// Config implements the Configurator interface.
func (c *ClusterConfig) Config(t machine.Type) (string, error) {
	return "", nil
}

// Etcd implements the Configurator interface.
func (c *ClusterConfig) Etcd() cluster.Etcd {
	return c.EtcdConfig
}

// Image implements the Configurator interface.
func (e *EtcdConfig) Image() string {
	return e.ContainerImage
}

// CA implements the Configurator interface.
func (e *EtcdConfig) CA() *x509.PEMEncodedCertificateAndKey {
	return e.RootCA
}

// Token implements the Configurator interface.
func (c *ClusterConfig) Token() cluster.Token {
	return c
}

// ID implements the Configurator interface.
func (c *ClusterConfig) ID() string {
	parts := strings.Split(c.BootstrapToken, ".")
	if len(parts) != 2 {
		return ""
	}

	return parts[0]
}

// Secret implements the Configurator interface.
func (c *ClusterConfig) Secret() string {
	parts := strings.Split(c.BootstrapToken, ".")
	if len(parts) != 2 {
		return ""
	}

	return parts[1]
}

// Network implements the Configurator interface.
func (c *ClusterConfig) Network() cluster.Network {
	return c
}

// CNI implements the Configurator interface.
func (c *ClusterConfig) CNI() string {
	switch {
	case c.ClusterNetwork == nil:
		fallthrough
	case c.ClusterNetwork.CNI == "":
		return constants.DefaultCNI
	}

	return c.ClusterNetwork.CNI
}

// PodCIDR implements the Configurator interface.
func (c *ClusterConfig) PodCIDR() string {
	switch {
	case c.ClusterNetwork == nil:
		fallthrough
	case len(c.ClusterNetwork.PodSubnet) == 0:
		return constants.DefaultPodCIDR
	}

	return c.ClusterNetwork.PodSubnet[0]
}

// ServiceCIDR implements the Configurator interface.
func (c *ClusterConfig) ServiceCIDR() string {
	switch {
	case c.ClusterNetwork == nil:
		fallthrough
	case len(c.ClusterNetwork.ServiceSubnet) == 0:
		return constants.DefaultServiceCIDR
	}

	return c.ClusterNetwork.ServiceSubnet[0]
}

// Hostname implements the Configurator interface.
func (n *NetworkConfig) Hostname() string {
	return n.NetworkHostname
}

// SetHostname implements the Configurator interface.
func (n *NetworkConfig) SetHostname(hostname string) {
	n.NetworkHostname = hostname
}

// Devices implements the Configurator interface.
func (n *NetworkConfig) Devices() []machine.Device {
	return n.NetworkInterfaces
}

// Resolvers implements the Configurator interface.
func (n *NetworkConfig) Resolvers() []string {
	return n.NameServers
}

// Servers implements the Configurator interface.
func (t *TimeConfig) Servers() []string {
	return t.TimeServers
}

// Image implements the Configurator interface.
func (i *InstallConfig) Image() string {
	return i.InstallImage
}

// Disk implements the Configurator interface.
func (i *InstallConfig) Disk() string {
	return i.InstallDisk
}

// ExtraKernelArgs implements the Configurator interface.
func (i *InstallConfig) ExtraKernelArgs() []string {
	return i.InstallExtraKernelArgs
}

// Zero implements the Configurator interface.
func (i *InstallConfig) Zero() bool {
	return i.InstallWipe
}

// Force implements the Configurator interface.
func (i *InstallConfig) Force() bool {
	return i.InstallForce
}

// WithBootloader implements the Configurator interface.
func (i *InstallConfig) WithBootloader() bool {
	return i.InstallBootloader
}
