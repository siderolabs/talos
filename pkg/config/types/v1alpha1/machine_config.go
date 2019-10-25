// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/crypto/x509"
)

// MachineConfig reperesents the machine-specific config values
type MachineConfig struct {
	MachineType     string                            `yaml:"type"`
	MachineToken    string                            `yaml:"token"`
	MachineCA       *x509.PEMEncodedCertificateAndKey `yaml:"ca,omitempty"`
	MachineCertSANs []string                          `yaml:"certSANs"`
	MachineKubelet  *KubeletConfig                    `yaml:"kubelet,omitempty"`
	MachineNetwork  *NetworkConfig                    `yaml:"network,omitempty"`
	MachineInstall  *InstallConfig                    `yaml:"install,omitempty"`
	MachineFiles    []machine.File                    `yaml:"files,omitempty"`
	MachineEnv      machine.Env                       `yaml:"env,omitempty"`
}

// KubeletConfig reperesents the kubelet config values
type KubeletConfig struct {
	Image     string            `yaml:"image,omitempty"`
	ExtraArgs map[string]string `yaml:"extraArgs,omitempty"`
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

// Network implements the Configurator interface.
func (m *MachineConfig) Network() machine.Network {
	if m.MachineNetwork == nil {
		return &NetworkConfig{}
	}

	return m.MachineNetwork
}

// Time implements the Configurator interface.
func (m *MachineConfig) Time() machine.Time {
	return m
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
