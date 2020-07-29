package v1alpha2

import (
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/talos-systems/crypto/x509"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

var (
	_ config.Machine        = (*Machine)(nil)
	_ config.MachineNetwork = (*MachineNetwork)(nil)
	_ config.Security       = (*Security)(nil)
	_ config.Install        = (*Install)(nil)
)

type Machine struct {
	provider *Provider
}

func (m *Machine) Network() config.MachineNetwork {
	return &MachineNetwork{m.provider}
}

func (m *Machine) Security() config.Security {
	return &Security{m.provider}
}

func (m *Machine) Install() config.Install {
	return &Install{m.provider}
}

func (m *Machine) Kubelet() config.Kubelet {
	return &Kubelet{m.provider}
}

func (m *Machine) Sysctls() map[string]string {
	if m.provider.MachineManifest.v1alpha1 != nil {
		return m.provider.MachineManifest.v1alpha1.Kernel.Sysctls
	}

	return nil
}

func (m *Machine) Disks() []config.Disk {
	if m.provider.MachineManifest.v1alpha1 != nil {
		return nil
	}
	return nil
}

func (m *Machine) Time() config.Time {
	if m.provider.MachineManifest.v1alpha1 != nil {
		return nil
	}

	return nil
}

func (m *Machine) Env() config.Env {
	if m.provider.MachineManifest.v1alpha1 != nil {
		return nil
	}

	return nil
}

func (m *Machine) Files() ([]config.File, error) {
	if m.provider.MachineManifest.v1alpha1 != nil {
		return nil, nil
	}

	return nil, nil
}

func (m *Machine) Type() machine.Type {
	if m := m.provider.MachineManifest.v1alpha1; m != nil {
		switch m.Type {
		case "controlplane":
			return machine.TypeControlPlane
		case "worker":
			return machine.TypeJoin
		}
	}

	return machine.TypeUnknown
}

func (m *Machine) Registries() config.Registries {
	return &Registries{m.provider}
}

// Install

type Install struct {
	provider *Provider
}

func (i *Install) Image() string {
	if install := i.provider.InstallManifest.v1alpha1; install != nil {
		return install.Image
	}

	return ""
}

func (i *Install) Disk() string {
	if install := i.provider.InstallManifest.v1alpha1; install != nil {
		return install.Disk
	}

	return ""
}

func (i *Install) ExtraKernelArgs() []string {
	if machine := i.provider.MachineManifest.v1alpha1; machine != nil {
		return machine.Kernel.Args
	}

	return nil
}

func (i *Install) Zero() bool {
	if install := i.provider.InstallManifest.v1alpha1; install != nil {
		return install.Zero
	}

	return false
}

func (i *Install) Force() bool {
	return true
}

func (i *Install) WithBootloader() bool {
	return true
}

// Security

type Security struct {
	provider *Provider
}

func (s *Security) CA() *x509.PEMEncodedCertificateAndKey {
	return nil
}

func (s *Security) Token() string {
	return ""
}

func (s *Security) CertSANs() []string {
	return nil
}

func (s *Security) SetCertSANs(sans []string) {
	return
}

// MachineNetwork

type MachineNetwork struct {
	provider *Provider
}

func (m *MachineNetwork) Hostname() string {
	return ""
}

func (m *MachineNetwork) SetHostname(hostname string) {
	return
}

func (m *MachineNetwork) Resolvers() []string {
	return nil
}

func (m *MachineNetwork) Devices() []config.Device {
	return nil
}

func (m *MachineNetwork) ExtraHosts() []config.ExtraHost {
	return nil
}

// Kubelet

type Kubelet struct {
	provider *Provider
}

func (k *Kubelet) Image() string {
	return ""
}

func (k *Kubelet) ExtraArgs() map[string]string {
	return nil
}

func (k *Kubelet) ExtraMounts() []specs.Mount {
	return nil
}

// Sysctls

// Registries

type Registries struct {
	provider *Provider
}

func (r *Registries) Mirrors() map[string]config.RegistryMirrorConfig {
	return nil
}

func (r *Registries) Config() map[string]config.RegistryConfig {
	return nil
}

func (r *Registries) ExtraFiles() ([]config.File, error) {
	return nil, nil
}
