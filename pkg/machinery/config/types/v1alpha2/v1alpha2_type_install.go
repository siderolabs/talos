package v1alpha2

import "github.com/talos-systems/talos/pkg/machinery/config"

func init() {
	config.Register("install", func(version string) (target interface{}) {
		if version == v1alpha1 {
			target = &InstallManifestV1Alpha1{}
		}

		return target
	})
}

// InstallManifestV1Alpha1 represents an install manifest.
type InstallManifestV1Alpha1 struct {
	Disk  string `yaml:"disk,omitempty"`
	Image string `yaml:"image,omitempty"`
	Zero  bool   `yaml:"zero,omitempty"`
}
