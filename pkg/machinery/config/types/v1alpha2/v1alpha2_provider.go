package v1alpha2

import (
	"github.com/talos-systems/talos/pkg/machinery/config"
)

const (
	v1alpha1 = "v1alpha1"
)

var (
	_ config.Provider = (*Provider)(nil)
)

func init() {
	config.Register("machine", func(version string) (target interface{}) {
		if version == v1alpha1 {
			target = &MachineManifestV1Alpha1{}
		}

		return target
	})
}

type Provider struct {
	MachineManifest struct {
		v1alpha1 *MachineManifestV1Alpha1
	}
	NetworkManifest struct {
		v1alpha1 *NetworkManifestV1Alpha1
	}
	InstallManifest struct {
		v1alpha1 *InstallManifestV1Alpha1
	}
	BootstrapManifest struct {
		v1alpha1 *BootstrapManifestV1Alpha1
	}
}

func New(manifests []interface{}) (c *Provider, err error) {
	c = &Provider{}

	for _, manifest := range manifests {
		switch m := manifest.(type) {
		case *MachineManifestV1Alpha1:
			c.MachineManifest.v1alpha1 = m
		case *NetworkManifestV1Alpha1:
			c.NetworkManifest.v1alpha1 = m
		case *InstallManifestV1Alpha1:
			c.InstallManifest.v1alpha1 = m
		case *BootstrapManifestV1Alpha1:
			c.BootstrapManifest.v1alpha1 = m
		}
	}

	return c, nil
}

func (p *Provider) Version() string {
	return "v1alpha2"
}

func (p *Provider) Debug() bool {
	if machine := p.MachineManifest.v1alpha1; machine != nil {
		return machine.Debug
	}

	return false
}

func (p *Provider) Persist() bool {
	if machine := p.MachineManifest.v1alpha1; machine != nil {
		return machine.Persist
	}

	return false
}

func (p *Provider) Cluster() config.Cluster {
	return &Cluster{p}
}

func (p *Provider) Machine() config.Machine {
	return &Machine{p}
}

func (p *Provider) Validate(config.RuntimeMode) error {
	return nil
}

func (p *Provider) Bytes() ([]byte, error) {
	return nil, nil
}

func (p *Provider) String() (string, error) {
	return "", nil
}
