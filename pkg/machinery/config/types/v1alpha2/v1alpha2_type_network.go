package v1alpha2

import "github.com/talos-systems/talos/pkg/machinery/config"

func init() {
	config.Register("network", func(version string) (target interface{}) {
		if version == v1alpha1 {
			target = &NetworkManifestV1Alpha1{}
		}

		return target
	})
}

// NetworkManifestV1Alpha1 represents a network manifest.
type NetworkManifestV1Alpha1 struct {
	Hostname    string               `yaml:"hostname,omitempty"`
	Nameservers []string             `yaml:"nameservers,omitempty"`
	Interfaces  []*InterfaceV1Alpha1 `yaml:"interfaces,omitempty"`
	Routes      []*RouteV1Alpha1     `yaml:"routes,omitempty"`
	Bonds       []*BondV1Alpha1      `yaml:"bonds,omitempty"`
}

type InterfaceV1Alpha1 struct{}
type RouteV1Alpha1 struct{}
type BondV1Alpha1 struct{}
