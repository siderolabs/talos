package v1alpha2

import "github.com/talos-systems/talos/pkg/machinery/config"

func init() {
	config.Register("bootstrap", func(version string) (target interface{}) {
		if version == v1alpha1 {
			target = &BootstrapManifestV1Alpha1{}
		}

		return target
	})
}

// BootstrapManifestV1Alpha1 represents a bootstrap manifest.
type BootstrapManifestV1Alpha1 struct {
	Cluster struct {
		Name string `yaml:"name,omitempty"`
	} `yaml:"cluster,omitempty"`
}
